package admin

import (
	"errors"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAidxCreatedAtString_NilGenerator(t *testing.T) {
	s, err := aidxCreatedAtString(nil, "anything")
	assert.Equal(t, "", s)
	assert.True(t, errors.Is(err, ErrIDGenMissing))
}

func TestAidxCreatedAtString_ValidAidx(t *testing.T) {
	gen, err := id.NewGenerator("aidx")
	require.NoError(t, err)

	fixed := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	aidx := gen.Generate(fixed)

	s, err := aidxCreatedAtString(gen, aidx)
	require.NoError(t, err)
	parsed, err := time.Parse("2006-01-02T15:04:05.000Z", s)
	require.NoError(t, err)
	assert.WithinDuration(t, fixed, parsed, time.Millisecond)
}

func TestAidxCreatedAtString_InvalidID(t *testing.T) {
	gen, err := id.NewGenerator("aidx")
	require.NoError(t, err)

	// base36 として読めない記号を含む ID は ParseTime が err を返す
	s, err := aidxCreatedAtString(gen, "!!!notaidx")
	assert.Equal(t, "", s)
	assert.Error(t, err)
	assert.False(t, errors.Is(err, ErrIDGenMissing), "non-aidx parse failure must not be reported as ErrIDGenMissing")
}
