package drive

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyseFile_Success(t *testing.T) {
	body := []byte("hello world")
	info, err := AnalyseFile(bytes.NewReader(body))
	require.NoError(t, err)
	assert.Equal(t, len(body), info.Size)
	assert.Equal(t, "5eb63bbbe01eeed093cb22bb8f5acdc3", info.MD5)
	assert.NotEmpty(t, info.MimeType)
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) { return 0, errors.New("boom") }

func TestAnalyseFile_ReadError(t *testing.T) {
	_, err := AnalyseFile(errReader{})
	assert.Error(t, err)
}
