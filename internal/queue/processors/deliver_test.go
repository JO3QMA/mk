package processors_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/queue/processors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSigner records the call and returns a canned response or error.
type stubSigner struct {
	resp     *http.Response
	err      error
	gotURL   string
	gotBody  []byte
	gotKeyID string
}

func (s *stubSigner) PostSigned(url string, body []byte, key *activitypub.PrivateKey) (*http.Response, error) {
	s.gotURL = url
	s.gotBody = body
	if key != nil {
		s.gotKeyID = key.KeyID
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

// generateTestKey returns a freshly minted PEM-encoded RSA private key for
// the processor to parse via activitypub.NewPrivateKey.
func generateTestKey(t *testing.T) string {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	der := x509.MarshalPKCS1PrivateKey(priv)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	return string(pem.EncodeToMemory(block))
}

func makeTask(t *testing.T, payload queue.DeliverPayload) *asynq.Task {
	t.Helper()
	return queue.NewDeliverTask(payload)
}

func makePayload(t *testing.T) queue.DeliverPayload {
	t.Helper()
	return queue.DeliverPayload{
		Inbox:  "https://remote.example/users/alice/inbox",
		Body:   []byte(`{"type":"Create"}`),
		KeyID:  "https://example.com/users/u1#main-key",
		KeyPEM: generateTestKey(t),
	}
}

func okResponse(status int) *http.Response {
	return &http.Response{
		Status:     http.StatusText(status),
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

func TestDeliverProcessor_Success(t *testing.T) {
	signer := &stubSigner{resp: okResponse(http.StatusOK)}
	p := processors.NewDeliverProcessor(signer)

	payload := makePayload(t)
	err := p.Handle(context.Background(), makeTask(t, payload))
	require.NoError(t, err)
	assert.Equal(t, payload.Inbox, signer.gotURL)
	assert.Equal(t, payload.Body, signer.gotBody)
	assert.Equal(t, payload.KeyID, signer.gotKeyID)
}

func TestDeliverProcessor_Accepted(t *testing.T) {
	signer := &stubSigner{resp: okResponse(http.StatusAccepted)}
	p := processors.NewDeliverProcessor(signer)
	require.NoError(t, p.Handle(context.Background(), makeTask(t, makePayload(t))))
}

func TestDeliverProcessor_Gone_SkipsRetry(t *testing.T) {
	signer := &stubSigner{resp: okResponse(http.StatusGone)}
	p := processors.NewDeliverProcessor(signer)
	err := p.Handle(context.Background(), makeTask(t, makePayload(t)))
	require.Error(t, err)
	assert.ErrorIs(t, err, asynq.SkipRetry)
}

func TestDeliverProcessor_NotFound_SkipsRetry(t *testing.T) {
	signer := &stubSigner{resp: okResponse(http.StatusNotFound)}
	p := processors.NewDeliverProcessor(signer)
	err := p.Handle(context.Background(), makeTask(t, makePayload(t)))
	require.Error(t, err)
	assert.ErrorIs(t, err, asynq.SkipRetry)
}

func TestDeliverProcessor_Forbidden_SkipsRetry(t *testing.T) {
	signer := &stubSigner{resp: okResponse(http.StatusForbidden)}
	p := processors.NewDeliverProcessor(signer)
	err := p.Handle(context.Background(), makeTask(t, makePayload(t)))
	require.Error(t, err)
	assert.ErrorIs(t, err, asynq.SkipRetry)
}

func TestDeliverProcessor_ServerError_RetriesByReturningError(t *testing.T) {
	signer := &stubSigner{resp: okResponse(http.StatusInternalServerError)}
	p := processors.NewDeliverProcessor(signer)
	err := p.Handle(context.Background(), makeTask(t, makePayload(t)))
	require.Error(t, err)
	assert.NotErrorIs(t, err, asynq.SkipRetry)
}

func TestDeliverProcessor_NetworkError_Retries(t *testing.T) {
	netErr := errors.New("connection refused")
	signer := &stubSigner{err: netErr}
	p := processors.NewDeliverProcessor(signer)
	err := p.Handle(context.Background(), makeTask(t, makePayload(t)))
	require.Error(t, err)
	assert.ErrorIs(t, err, netErr)
	assert.NotErrorIs(t, err, asynq.SkipRetry)
}

func TestDeliverProcessor_BadPayload_SkipsRetry(t *testing.T) {
	p := processors.NewDeliverProcessor(&stubSigner{})
	task := asynq.NewTask(queue.TaskTypeDeliver, []byte(`{not json`))
	err := p.Handle(context.Background(), task)
	require.Error(t, err)
	assert.ErrorIs(t, err, asynq.SkipRetry)
}

func TestDeliverProcessor_BadKey_SkipsRetry(t *testing.T) {
	p := processors.NewDeliverProcessor(&stubSigner{})
	payload := queue.DeliverPayload{
		Inbox:  "https://remote.example/inbox",
		Body:   []byte(`{}`),
		KeyID:  "k",
		KeyPEM: "not a pem",
	}
	err := p.Handle(context.Background(), makeTask(t, payload))
	require.Error(t, err)
	assert.ErrorIs(t, err, asynq.SkipRetry)
}

// stubResponseHook captures host events for assertions.
type stubResponseHook struct {
	successes []string
	errors    []string
}

func (s *stubResponseHook) RecordResponseSuccess(host string) error {
	s.successes = append(s.successes, host)
	return nil
}

func (s *stubResponseHook) RecordResponseError(host string) error {
	s.errors = append(s.errors, host)
	return nil
}

func TestDeliverProcessor_ResponseHook_Success(t *testing.T) {
	signer := &stubSigner{resp: okResponse(http.StatusOK)}
	p := processors.NewDeliverProcessor(signer)
	hook := &stubResponseHook{}
	p.SetResponseHook(hook)
	require.NoError(t, p.Handle(context.Background(), makeTask(t, makePayload(t))))
	assert.Equal(t, []string{"remote.example"}, hook.successes)
	assert.Empty(t, hook.errors)
}

func TestDeliverProcessor_ResponseHook_Gone_RecordsSuccess(t *testing.T) {
	signer := &stubSigner{resp: okResponse(http.StatusGone)}
	p := processors.NewDeliverProcessor(signer)
	hook := &stubResponseHook{}
	p.SetResponseHook(hook)
	err := p.Handle(context.Background(), makeTask(t, makePayload(t)))
	require.Error(t, err)
	assert.Equal(t, []string{"remote.example"}, hook.successes)
}

func TestDeliverProcessor_ResponseHook_ClientError_RecordsSuccess(t *testing.T) {
	signer := &stubSigner{resp: okResponse(http.StatusForbidden)}
	p := processors.NewDeliverProcessor(signer)
	hook := &stubResponseHook{}
	p.SetResponseHook(hook)
	err := p.Handle(context.Background(), makeTask(t, makePayload(t)))
	require.Error(t, err)
	assert.Equal(t, []string{"remote.example"}, hook.successes)
}

func TestDeliverProcessor_ResponseHook_ServerError_RecordsError(t *testing.T) {
	signer := &stubSigner{resp: okResponse(http.StatusInternalServerError)}
	p := processors.NewDeliverProcessor(signer)
	hook := &stubResponseHook{}
	p.SetResponseHook(hook)
	err := p.Handle(context.Background(), makeTask(t, makePayload(t)))
	require.Error(t, err)
	assert.Equal(t, []string{"remote.example"}, hook.errors)
}

func TestDeliverProcessor_ResponseHook_NetworkError_RecordsError(t *testing.T) {
	signer := &stubSigner{err: errors.New("net down")}
	p := processors.NewDeliverProcessor(signer)
	hook := &stubResponseHook{}
	p.SetResponseHook(hook)
	err := p.Handle(context.Background(), makeTask(t, makePayload(t)))
	require.Error(t, err)
	assert.Equal(t, []string{"remote.example"}, hook.errors)
}

func TestDeliverProcessor_ResponseHook_UnparseableInboxSuccess(t *testing.T) {
	signer := &stubSigner{resp: okResponse(http.StatusOK)}
	p := processors.NewDeliverProcessor(signer)
	hook := &stubResponseHook{}
	p.SetResponseHook(hook)
	payload := queue.DeliverPayload{
		Inbox:  "://bad-url",
		Body:   []byte(`{}`),
		KeyID:  "https://example.com/users/u1#main-key",
		KeyPEM: generateTestKey(t),
	}
	require.NoError(t, p.Handle(context.Background(), makeTask(t, payload)))
	assert.Empty(t, hook.successes)
}

func TestDeliverProcessor_ResponseHook_UnparseableInboxError(t *testing.T) {
	signer := &stubSigner{err: errors.New("net down")}
	p := processors.NewDeliverProcessor(signer)
	hook := &stubResponseHook{}
	p.SetResponseHook(hook)
	payload := queue.DeliverPayload{
		Inbox:  "://bad-url",
		Body:   []byte(`{}`),
		KeyID:  "https://example.com/users/u1#main-key",
		KeyPEM: generateTestKey(t),
	}
	err := p.Handle(context.Background(), makeTask(t, payload))
	require.Error(t, err)
	assert.Empty(t, hook.errors)
}
