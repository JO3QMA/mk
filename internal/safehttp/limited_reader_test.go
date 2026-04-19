package safehttp

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadAllLimit_UnderCap(t *testing.T) {
	data, err := ReadAllLimit(strings.NewReader("hello"), 1024)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), data)
}

func TestReadAllLimit_ExactCap(t *testing.T) {
	data, err := ReadAllLimit(bytes.NewReader(make([]byte, 100)), 100)
	require.NoError(t, err)
	assert.Len(t, data, 100)
}

func TestReadAllLimit_OverCap(t *testing.T) {
	_, err := ReadAllLimit(bytes.NewReader(make([]byte, 101)), 100)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrResponseTooLarge))
}

func TestReadAllLimit_ZeroOrNegativeDisablesCap(t *testing.T) {
	// max <= 0 は io.ReadAll と等価。
	data, err := ReadAllLimit(strings.NewReader("unbounded"), 0)
	require.NoError(t, err)
	assert.Equal(t, []byte("unbounded"), data)

	data, err = ReadAllLimit(strings.NewReader("unbounded"), -1)
	require.NoError(t, err)
	assert.Equal(t, []byte("unbounded"), data)
}

type errReader struct{ err error }

func (r *errReader) Read(_ []byte) (int, error) { return 0, r.err }

func TestReadAllLimit_PropagatesReadError(t *testing.T) {
	sentinel := errors.New("io boom")
	_, err := ReadAllLimit(&errReader{err: sentinel}, 1024)
	require.Error(t, err)
	assert.True(t, errors.Is(err, sentinel))
}
