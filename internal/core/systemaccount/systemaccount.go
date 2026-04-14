// Package systemaccount manages built-in local users that represent
// subsystems of the instance itself (relay actor, instance actor,
// proxy fetcher etc). 本家 Misskey の SystemAccountService 相当。
//
// 各 kind に 1 つずつ user + user_profile + user_keypair + system_account
// 行を作成・取得する。初回 Fetch 時に行が無ければ lazy に作成する。
package systemaccount

import (
	"errors"
	"fmt"
	"time"

	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
	"gorm.io/gorm"
)

// Service fetches (or lazily creates) the built-in system users used
// for subsystem-to-AP interactions. 各 kind は一意なので 1 行のみ。
type Service struct {
	userRepo    repository.UserRepository
	keypairRepo repository.UserKeypairRepository
	saRepo      repository.SystemAccountRepository
	idGen       id.Generator
	clock       func() time.Time
}

// NewService constructs a Service.
func NewService(
	userRepo repository.UserRepository,
	keypairRepo repository.UserKeypairRepository,
	saRepo repository.SystemAccountRepository,
	idGen id.Generator,
) *Service {
	return &Service{
		userRepo:    userRepo,
		keypairRepo: keypairRepo,
		saRepo:      saRepo,
		idGen:       idGen,
		clock:       time.Now,
	}
}

// SetClock replaces the clock, primarily for tests.
func (s *Service) SetClock(now func() time.Time) {
	if now != nil {
		s.clock = now
	}
}

// Fetch returns the system user for the given kind, creating it if
// necessary. kind は TS Misskey 本家と同じ識別子 ("relay", "actor",
// "proxy") を想定する。
func (s *Service) Fetch(kind string) (*model.User, error) {
	if kind == "" {
		return nil, errors.New("systemaccount: kind is required")
	}
	sa, err := s.saRepo.FindByType(kind)
	if err == nil {
		return s.userRepo.FindByID(sa.UserID)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return s.create(kind)
}

// create materialises a new system user. username は kind ごとに決め打ち
// ("relay.actor" / "instance.actor" / "proxy.actor") で、本家 Misskey の
// SystemAccountService.createCorrespondingUser と同じ形式を採用する。
func (s *Service) create(kind string) (*model.User, error) {
	now := s.clock()
	username := kind + ".actor"
	// 鍵ペアを先に生成しておく (User 行を作ってから keypair 作成する順だが
	// 鍵生成失敗時に user_profile を残さないよう先に作る)。
	privPEM, pubPEM, err := activitypub.GenerateRSAKeypair()
	if err != nil {
		return nil, fmt.Errorf("systemaccount: generate keypair: %w", err)
	}

	user := &model.User{
		ID:            s.idGen.Generate(now),
		Username:      username,
		UsernameLower: username,
		Host:          nil,
		IsExplorable:  false,
		IsBot:         true,
		IsLocked:      true,
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("systemaccount: create user: %w", err)
	}
	// user_profile は本家互換のため最小行だけ入れる (email/password は nil)
	profile := &model.UserProfile{UserID: user.ID}
	if err := s.userRepo.CreateProfile(profile); err != nil {
		return nil, fmt.Errorf("systemaccount: create profile: %w", err)
	}
	if err := s.keypairRepo.Create(&model.UserKeypair{
		UserID:     user.ID,
		PublicKey:  pubPEM,
		PrivateKey: privPEM,
	}); err != nil {
		return nil, fmt.Errorf("systemaccount: create keypair: %w", err)
	}
	if err := s.saRepo.Create(&model.SystemAccount{
		ID:     s.idGen.Generate(now),
		UserID: user.ID,
		Type:   kind,
	}); err != nil {
		return nil, fmt.Errorf("systemaccount: create system_account: %w", err)
	}
	return user, nil
}
