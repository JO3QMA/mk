package federation

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/repository"
)

// Errors returned by DeliverService.
var (
	// ErrSignerKeyMissing is returned when no keypair is found for the signer.
	ErrSignerKeyMissing = errors.New("signer keypair not found")
	// ErrNoLocalSigner is returned when DeliverService is asked to sign with a
	// remote user. リモートユーザーの代理署名は不可能。
	ErrNoLocalSigner = errors.New("cannot sign on behalf of a remote user")
)

// HostBlockChecker reports whether a host is on the blocked list. The
// federation package only depends on this minimal interface to avoid coupling
// to core/instance directly.
type HostBlockChecker interface {
	IsBlocked(host string) bool
}

// DeliverService computes recipient inboxes and enqueues HTTP-signed delivery
// jobs onto the asynq queue.
//
// 配信先計算と enqueue を分離するため、実際のHTTP送信は queue/processors の
// DeliverProcessor が担当する。
type DeliverService struct {
	enqueuer      queue.Enqueuer
	userRepo      repository.UserRepository
	followingRepo repository.FollowingRepository
	keypairRepo   repository.UserKeypairRepository
	urls          *activitypub.URLBuilder
	hostBlocker   HostBlockChecker
}

// NewDeliverService constructs a DeliverService.
func NewDeliverService(
	enqueuer queue.Enqueuer,
	userRepo repository.UserRepository,
	followingRepo repository.FollowingRepository,
	keypairRepo repository.UserKeypairRepository,
	urls *activitypub.URLBuilder,
) *DeliverService {
	return &DeliverService{
		enqueuer:      enqueuer,
		userRepo:      userRepo,
		followingRepo: followingRepo,
		keypairRepo:   keypairRepo,
		urls:          urls,
	}
}

// SetHostBlockChecker attaches a HostBlockChecker. 配信時にこの checker が
// 呼ばれ、ブロック対象ホストの inbox にはエンキューしない。
func (s *DeliverService) SetHostBlockChecker(c HostBlockChecker) {
	s.hostBlocker = c
}

// DeliverActivity enqueues a deliver job for each unique inbox in inboxes.
// signerUserID は署名に使うローカルユーザー。ブロック対象ホストの inbox は
// スキップする。
func (s *DeliverService) DeliverActivity(signerUserID string, body []byte, inboxes []string) error {
	if len(inboxes) == 0 {
		return nil
	}
	keyID, keyPEM, err := s.signerCredentials(signerUserID)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(inboxes))
	for _, inbox := range inboxes {
		if inbox == "" {
			continue
		}
		if _, dup := seen[inbox]; dup {
			continue
		}
		if s.isBlockedInbox(inbox) {
			continue
		}
		seen[inbox] = struct{}{}
		payload := queue.DeliverPayload{
			Inbox:  inbox,
			Body:   body,
			KeyID:  keyID,
			KeyPEM: keyPEM,
		}
		if err := s.enqueuer.EnqueueDeliver(payload); err != nil {
			return fmt.Errorf("enqueue deliver to %s: %w", inbox, err)
		}
	}
	return nil
}

// isBlockedInbox returns true when the inbox URL points at a host the local
// instance has blocked. blocker が未設定なら常に false。
func (s *DeliverService) isBlockedInbox(inbox string) bool {
	if s.hostBlocker == nil {
		return false
	}
	u, err := url.Parse(inbox)
	if err != nil || u.Host == "" {
		return false
	}
	return s.hostBlocker.IsBlocked(u.Host)
}

// DeliverToFollowers enqueues delivery to all remote followers of signerUserID.
// sharedInbox は repository 側で集約済み。
func (s *DeliverService) DeliverToFollowers(signerUserID string, body []byte) error {
	inboxes, err := s.followingRepo.ListRemoteFollowerInboxes(signerUserID)
	if err != nil {
		return err
	}
	return s.DeliverActivity(signerUserID, body, inboxes)
}

// DeliverToUser enqueues a delivery to a single recipient user. Local users
// are skipped (no AP delivery needed). リモートユーザーの sharedInbox があれば
// 優先する。
func (s *DeliverService) DeliverToUser(signerUserID string, recipient *model.User, body []byte) error {
	if recipient == nil || recipient.IsLocal() {
		return nil
	}
	inbox := preferredInbox(recipient)
	if inbox == "" {
		return nil
	}
	return s.DeliverActivity(signerUserID, body, []string{inbox})
}

// signerCredentials returns the keyId URI and PEM private key for a local
// signer user.
func (s *DeliverService) signerCredentials(userID string) (string, string, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return "", "", err
	}
	if !user.IsLocal() {
		return "", "", ErrNoLocalSigner
	}
	kp, err := s.keypairRepo.FindByUserID(userID)
	if err != nil {
		return "", "", ErrSignerKeyMissing
	}
	keyID := s.urls.UserKeyURI(userID)
	return keyID, kp.PrivateKey, nil
}

// preferredInbox returns the sharedInbox of u when present, otherwise the
// individual inbox.
func preferredInbox(u *model.User) string {
	if u.SharedInbox != nil && *u.SharedInbox != "" {
		return *u.SharedInbox
	}
	if u.Inbox != nil {
		return *u.Inbox
	}
	return ""
}
