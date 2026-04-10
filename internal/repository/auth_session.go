package repository

import (
	"github.com/shiroha-a/mk/internal/model"
	"gorm.io/gorm"
)

// AuthSessionRepository handles auth_session + app persistence.
type AuthSessionRepository interface {
	// App operations
	FindAppBySecret(secret string) (*model.App, error)
	CreateApp(app *model.App) error

	// Session operations
	CreateSession(session *model.AuthSession) error
	FindSessionByToken(token string) (*model.AuthSession, error)
	FindSessionByTokenAndAppID(token, appID string) (*model.AuthSession, error)
	UpdateSessionUserID(sessionID, userID string) error
	DeleteSession(sessionID string) error

	// AccessToken operations for MiAuth
	FindAccessTokenByAppAndUser(appID, userID string) (*model.AccessToken, error)
	CreateAccessToken(token *model.AccessToken) error
}

type authSessionRepository struct {
	db *gorm.DB
}

// NewAuthSessionRepository creates a new AuthSessionRepository.
func NewAuthSessionRepository(db *gorm.DB) AuthSessionRepository {
	return &authSessionRepository{db: db}
}

func (r *authSessionRepository) FindAppBySecret(secret string) (*model.App, error) {
	var app model.App
	if err := r.db.Where(`"secret" = ?`, secret).First(&app).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *authSessionRepository) CreateApp(app *model.App) error {
	return r.db.Create(app).Error
}

func (r *authSessionRepository) CreateSession(session *model.AuthSession) error {
	return r.db.Create(session).Error
}

func (r *authSessionRepository) FindSessionByToken(token string) (*model.AuthSession, error) {
	var session model.AuthSession
	if err := r.db.Preload("App").Where(`"token" = ?`, token).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *authSessionRepository) FindSessionByTokenAndAppID(token, appID string) (*model.AuthSession, error) {
	var session model.AuthSession
	if err := r.db.Preload("App").Preload("User").Where(`"token" = ? AND "appId" = ?`, token, appID).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *authSessionRepository) UpdateSessionUserID(sessionID, userID string) error {
	return r.db.Model(&model.AuthSession{}).Where(`"id" = ?`, sessionID).Update("userId", userID).Error
}

func (r *authSessionRepository) DeleteSession(sessionID string) error {
	return r.db.Delete(&model.AuthSession{}, `"id" = ?`, sessionID).Error
}

func (r *authSessionRepository) FindAccessTokenByAppAndUser(appID, userID string) (*model.AccessToken, error) {
	var token model.AccessToken
	if err := r.db.Where(`"appId" = ? AND "userId" = ?`, appID, userID).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *authSessionRepository) CreateAccessToken(token *model.AccessToken) error {
	return r.db.Create(token).Error
}
