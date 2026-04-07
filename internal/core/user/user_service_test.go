package user_test

import (
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_ShowByID_Success(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	desc := "hello"
	repo.Profiles["u1"] = &model.UserProfile{UserID: "u1", Description: &desc}
	svc := user.NewService(repo)

	bundle, err := svc.ShowByID("u1")
	require.NoError(t, err)
	assert.Equal(t, "alice", bundle.User.Username)
	require.NotNil(t, bundle.Profile)
	assert.Equal(t, "hello", *bundle.Profile.Description)
}

func TestService_ShowByID_NotFound(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	svc := user.NewService(repo)

	_, err := svc.ShowByID("missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrUserNotFound))
}

func TestService_ShowByID_NoProfile(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice"}
	svc := user.NewService(repo)

	bundle, err := svc.ShowByID("u1")
	require.NoError(t, err)
	assert.Nil(t, bundle.Profile)
}

func TestService_ShowByUsername_Success(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	repo.Users["u1"] = &model.User{ID: "u1", Username: "alice", UsernameLower: "alice"}
	svc := user.NewService(repo)

	bundle, err := svc.ShowByUsername("alice", nil)
	require.NoError(t, err)
	assert.Equal(t, "u1", bundle.User.ID)
}

func TestService_ShowByUsername_WithHost(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	host := "remote.example.com"
	repo.Users["u2"] = &model.User{ID: "u2", Username: "bob", UsernameLower: "bob", Host: &host}
	svc := user.NewService(repo)

	bundle, err := svc.ShowByUsername("bob", &host)
	require.NoError(t, err)
	assert.Equal(t, "u2", bundle.User.ID)
}

func TestService_ShowByUsername_NotFound(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	svc := user.NewService(repo)

	_, err := svc.ShowByUsername("nobody", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, user.ErrUserNotFound))
}

func TestService_GetProfile_Found(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	desc := "hi"
	repo.Profiles["u1"] = &model.UserProfile{UserID: "u1", Description: &desc}
	svc := user.NewService(repo)

	p := svc.GetProfile("u1")
	require.NotNil(t, p)
	assert.Equal(t, "hi", *p.Description)
}

func TestService_GetProfile_Missing(t *testing.T) {
	repo := testutil.NewMockUserRepository()
	svc := user.NewService(repo)

	assert.Nil(t, svc.GetProfile("missing"))
}
