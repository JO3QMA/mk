package twofactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateBackupCodes_CountAndShape(t *testing.T) {
	codes, err := GenerateBackupCodes()
	require.NoError(t, err)
	assert.Len(t, codes, BackupCodeCount, "should produce BackupCodeCount codes")
	for _, c := range codes {
		// hex-encoded 8 bytes → 16 chars
		assert.Len(t, c, BackupCodeBytes*2)
	}
}

func TestGenerateBackupCodes_Unique(t *testing.T) {
	// 5 codes from a 64-bit space — collision practically impossible.
	codes, err := GenerateBackupCodes()
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, c := range codes {
		assert.False(t, seen[c], "duplicate code %s", c)
		seen[c] = true
	}
}

func TestConsumeBackupCode_Match(t *testing.T) {
	codes := []string{"aaaa", "bbbb", "cccc"}
	remaining, err := ConsumeBackupCode(codes, "bbbb")
	require.NoError(t, err)
	assert.Equal(t, []string{"aaaa", "cccc"}, remaining)
}

func TestConsumeBackupCode_FirstMatch(t *testing.T) {
	codes := []string{"aaaa", "bbbb"}
	remaining, err := ConsumeBackupCode(codes, "aaaa")
	require.NoError(t, err)
	assert.Equal(t, []string{"bbbb"}, remaining)
}

func TestConsumeBackupCode_LastMatch(t *testing.T) {
	codes := []string{"aaaa", "bbbb"}
	remaining, err := ConsumeBackupCode(codes, "bbbb")
	require.NoError(t, err)
	assert.Equal(t, []string{"aaaa"}, remaining)
}

func TestConsumeBackupCode_Mismatch(t *testing.T) {
	codes := []string{"aaaa", "bbbb"}
	_, err := ConsumeBackupCode(codes, "cccc")
	assert.ErrorIs(t, err, ErrBackupCodeMismatch)
}

func TestConsumeBackupCode_EmptyInput(t *testing.T) {
	_, err := ConsumeBackupCode([]string{"aaaa"}, "")
	assert.ErrorIs(t, err, ErrBackupCodeMismatch)
}

func TestConsumeBackupCode_EmptyStored(t *testing.T) {
	_, err := ConsumeBackupCode(nil, "aaaa")
	assert.ErrorIs(t, err, ErrBackupCodeMismatch)
}

// rand 経路エラー: webauthn.go と共通の readRandom を差し替えて exercise する。
func TestGenerateBackupCodes_RandError(t *testing.T) {
	old := readRandom
	defer func() { readRandom = old }()
	readRandom = func(b []byte) (int, error) {
		return 0, assertErr{}
	}
	_, err := GenerateBackupCodes()
	assert.Error(t, err)
}

type assertErr struct{}

func (assertErr) Error() string { return "test rand failure" }
