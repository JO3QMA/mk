package repository

import (
	"context"
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupUserKeypair(t *testing.T, userID string) {
	t.Helper()
	testDB.Exec(`DELETE FROM "user_keypair" WHERE "userId" = ?`, userID)
}

func TestUserKeypairRepository_CreateAndFind(t *testing.T) {
	repo := NewUserKeypairRepository(testDB)
	user := insertTestUser(t, "u_kp_1", "kp1")
	defer cleanupUser(t, user.ID)

	k := &model.UserKeypair{
		UserID:     user.ID,
		PublicKey:  "PUBKEY",
		PrivateKey: "PRIVKEY",
	}
	require.NoError(t, repo.Create(k))
	defer cleanupUserKeypair(t, user.ID)

	got, err := repo.FindByUserID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, "PUBKEY", got.PublicKey)
}

func TestUserKeypairRepository_QueryErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewUserKeypairRepository(testDB.WithContext(ctx))
	err := repo.Create(&model.UserKeypair{UserID: "x", PublicKey: "p", PrivateKey: "k"})
	assert.Error(t, err)
	_, err = repo.FindByUserID("x")
	assert.Error(t, err)
}
