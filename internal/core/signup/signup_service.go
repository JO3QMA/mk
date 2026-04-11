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
)

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
	webhookHook WebhookHook
	idGen       id.Generator
}

// NewService creates a new SignupService.
func NewService(userRepo repository.UserRepository, metaRepo repository.MetaRepository, idGen id.Generator) *Service {
	return &Service{userRepo: userRepo, metaRepo: metaRepo, idGen: idGen}
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
