package signup

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrUsernameAlreadyExists is returned when the username is taken.
	ErrUsernameAlreadyExists = errors.New("username already exists")
	// ErrInvalidUsername is returned when the username is invalid.
	ErrInvalidUsername = errors.New("invalid username")
	// ErrUsernameReserved is returned when the username matches an entry in
	// meta.preservedUsernames (case-insensitive). 初回セットアップ時は root
	// ユーザー作成を妨げないため、このチェックはスキップする。
	ErrUsernameReserved = errors.New("username is reserved")
	// ErrPendingNotFound is returned when no user_pending row matches the code.
	ErrPendingNotFound = errors.New("pending signup not found")
	// ErrPendingExpired is returned when the pending signup is past its TTL.
	// TTL は ID (ULID) 由来 timestamp から算出する (createdAt カラム不在のため)。
	ErrPendingExpired = errors.New("pending signup expired")
)

// PendingSignupTTL is the default lifetime of a pending signup row. Misskey TS
// 実装では明示的な TTL は無いが、放置 row の蓄積を避けるため 24h で運用する。
// ID (ULID) の timestamp と比較して PromotePending 時に判定する。
const PendingSignupTTL = 24 * time.Hour

// WebhookHook is invoked after a new local user has been created so that
// system webhooks subscribed to `userCreated` can fire. 循環依存を避けるため
// interface で受け取る (実装は core/webhook)。
type WebhookHook interface {
	OnUserCreated(user *model.User)
}

// Service handles user registration.
type Service struct {
	userRepo    repository.UserRepository
	metaRepo    repository.MetaRepository
	keypairRepo repository.UserKeypairRepository
	pendingRepo repository.UserPendingRepository
	webhookHook WebhookHook
	idGen       id.Generator
}

// NewService creates a new SignupService.
func NewService(userRepo repository.UserRepository, metaRepo repository.MetaRepository, idGen id.Generator) *Service {
	return &Service{userRepo: userRepo, metaRepo: metaRepo, idGen: idGen}
}

// SetUserPendingRepo wires the user_pending repository so CreatePending /
// PromotePending become available. emailRequiredForSignup フローを使う場合は
// 必須。未設定でも通常の Signup は動く。
func (s *Service) SetUserPendingRepo(r repository.UserPendingRepository) {
	s.pendingRepo = r
}

// SetKeypairRepo wires the user keypair repository. When set, Signup will
// generate a fresh RSA keypair for each newly created local user, which is
// required for ActivityPub federation (actor endpoints return publicKey).
func (s *Service) SetKeypairRepo(r repository.UserKeypairRepository) {
	s.keypairRepo = r
}

// SetWebhookHook attaches a WebhookHook invoked after user creation so that
// system webhooks subscribed to `userCreated` can fire.
func (s *Service) SetWebhookHook(h WebhookHook) {
	s.webhookHook = h
}

// SignupResult holds the created user and their native token.
type SignupResult struct {
	User  *model.User
	Token string
}

// Signup creates a new local user with the given username and password.
// isInitialSetup=true の場合、作成したユーザーを rootUser に設定する。
func (s *Service) Signup(username, password string, isInitialSetup bool) (*SignupResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(username) > 128 {
		return nil, ErrInvalidUsername
	}

	// ユーザー名の重複チェック
	lower := strings.ToLower(username)
	if _, err := s.userRepo.FindByUsernameLower(lower, nil); err == nil {
		return nil, ErrUsernameAlreadyExists
	}

	// meta.preservedUsernames チェック。初回セットアップ (root ユーザー作成) は
	// admin / root が予約ワードに含まれうるので除外する。meta fetch 失敗時は
	// ベストエフォートで通過させる (オンライン性を優先)。
	if !isInitialSetup {
		if meta, err := s.metaRepo.Fetch(); err == nil && isReservedUsername(lower, meta.PreservedUsernames) {
			return nil, ErrUsernameReserved
		}
	}

	// パスワードハッシュ (bcrypt.DefaultCostで有効なパスワードに対して失敗しない)
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	// native token 生成 (16文字hex)
	token := generateToken()

	now := time.Now()
	userID := s.idGen.Generate(now)
	user := &model.User{
		ID:                userID,
		Username:          username,
		UsernameLower:     lower,
		Token:             &token,
		IsExplorable:      true,
		AvatarDecorations: []byte("[]"),
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	// user_profile 作成
	hashStr := string(hash)
	profile := &model.UserProfile{
		UserID:             userID,
		Password:           &hashStr,
		AutoAcceptFollowed: true,
		PreventAiLearning:  true,
		PublicReactions:    true,
	}
	if err := s.userRepo.CreateProfile(profile); err != nil {
		return nil, err
	}

	// RSA keypair (federation 用)。keypairRepo が未設定の場合はスキップする
	// (従来の単体テストのため)。
	if s.keypairRepo != nil {
		privPEM, pubPEM, err := activitypub.GenerateRSAKeypair()
		if err != nil {
			return nil, err
		}
		if err := s.keypairRepo.Create(&model.UserKeypair{
			UserID:     userID,
			PublicKey:  pubPEM,
			PrivateKey: privPEM,
		}); err != nil {
			return nil, err
		}
	}

	// 初回セットアップの場合は rootUserId を設定
	if isInitialSetup {
		_ = s.metaRepo.Update(map[string]any{"rootUserId": userID})
	}

	// システムWebhook発火（`userCreated` 相当）。ベストエフォート。
	if s.webhookHook != nil {
		s.webhookHook.OnUserCreated(user)
	}

	return &SignupResult{User: user, Token: token}, nil
}

// generateToken creates a random 16-character token.
// crypto/rand.Read は実質的に失敗しない。
func generateToken() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// generatePendingCode creates a 32-char (16 byte) hex code used both as the
// user_pending.code column and the email confirmation link path segment.
// signup native token と区別するため長め。
func generatePendingCode() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// CreatePending stores a pending signup row and returns it. Username重複と
// 予約名チェックは通常 Signup と揃え、password は bcrypt で hash 済を保管
// (PromotePending 時に再 hash しない)。emailRequiredForSignup=true 用。
func (s *Service) CreatePending(username, email, password string) (*model.UserPending, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(username) > 128 {
		return nil, ErrInvalidUsername
	}
	lower := strings.ToLower(username)

	// 確定済 user との衝突チェック (この段階で押さえておかないと、後段の
	// PromotePending 直前まで気付けず無駄なメール送信になる)。
	if _, err := s.userRepo.FindByUsernameLower(lower, nil); err == nil {
		return nil, ErrUsernameAlreadyExists
	}
	if meta, err := s.metaRepo.Fetch(); err == nil && isReservedUsername(lower, meta.PreservedUsernames) {
		return nil, ErrUsernameReserved
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	now := time.Now()
	row := &model.UserPending{
		ID:       s.idGen.Generate(now),
		Code:     generatePendingCode(),
		Username: username,
		Email:    email,
		Password: string(hash),
	}
	if err := s.pendingRepo.Create(row); err != nil {
		return nil, err
	}
	return row, nil
}

// PromotePending finalizes a pending signup: looks up by code, confirms the
// row hasn't expired, then creates the user using the stored hashed password
// (再 hash しない)。成功時に user_pending row を delete する。
//
// 失敗パターン:
//   - ErrPendingNotFound: code が無い / DB error
//   - ErrPendingExpired:  ID (ULID) timestamp が PendingSignupTTL を超過
//   - ErrUsernameAlreadyExists: 確認 link 待ちの間に同名 user が登録されたケース
func (s *Service) PromotePending(code string) (*SignupResult, error) {
	pending, err := s.pendingRepo.FindByCode(code)
	if err != nil {
		return nil, ErrPendingNotFound
	}
	// ID (ULID) から作成時刻を引き、TTL を過ぎていれば拒否。row 自体は
	// 残しておく (cron での bulk cleanup を前提)。
	if t, err := s.idGen.ParseTime(pending.ID); err == nil {
		if time.Since(t) > PendingSignupTTL {
			return nil, ErrPendingExpired
		}
	}

	lower := strings.ToLower(pending.Username)
	if _, err := s.userRepo.FindByUsernameLower(lower, nil); err == nil {
		return nil, ErrUsernameAlreadyExists
	}

	token := generateToken()
	now := time.Now()
	userID := s.idGen.Generate(now)
	user := &model.User{
		ID:                userID,
		Username:          pending.Username,
		UsernameLower:     lower,
		Token:             &token,
		IsExplorable:      true,
		AvatarDecorations: []byte("[]"),
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}
	storedHash := pending.Password
	profile := &model.UserProfile{
		UserID:             userID,
		Email:              &pending.Email,
		EmailVerified:      true,
		Password:           &storedHash,
		AutoAcceptFollowed: true,
		PreventAiLearning:  true,
		PublicReactions:    true,
	}
	if err := s.userRepo.CreateProfile(profile); err != nil {
		return nil, err
	}
	if s.keypairRepo != nil {
		privPEM, pubPEM, err := activitypub.GenerateRSAKeypair()
		if err != nil {
			return nil, err
		}
		if err := s.keypairRepo.Create(&model.UserKeypair{
			UserID:     userID,
			PublicKey:  pubPEM,
			PrivateKey: privPEM,
		}); err != nil {
			return nil, err
		}
	}
	// pending を削除 (失敗してもユーザー作成自体は成功なので無視)。
	_ = s.pendingRepo.Delete(pending.ID)

	if s.webhookHook != nil {
		s.webhookHook.OnUserCreated(user)
	}
	return &SignupResult{User: user, Token: token}, nil
}

// isReservedUsername reports whether lower (already lowercased) matches any
// entry in reserved case-insensitively. Entries in reserved are trimmed and
// lowercased before comparison so DB-side whitespace noise does not defeat
// the check.
func isReservedUsername(lower string, reserved []string) bool {
	for _, r := range reserved {
		if strings.EqualFold(strings.TrimSpace(r), lower) {
			return true
		}
	}
	return false
}
