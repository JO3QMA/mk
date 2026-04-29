package processors_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/core/federation"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/queue/driver"
	"github.com/shiroha-a/mk/internal/queue/processors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubFedProcessor は processors.FederationProcessor を満たす最小スタブ。
// federation.NewProcessor の重い依存 chain (resolver / userRepo / noteRepo)
// を組まずに InboxProcessor の dispatch 経路を unit-test するため切る。
type stubFedProcessor struct {
	calls    [][]byte
	returnFn func(body []byte) error
}

func (s *stubFedProcessor) Process(body []byte) error {
	s.calls = append(s.calls, body)
	if s.returnFn != nil {
		return s.returnFn(body)
	}
	return nil
}

func mustEncode(t *testing.T, p queue.InboxPayload) []byte {
	t.Helper()
	b, err := json.Marshal(p)
	require.NoError(t, err)
	return b
}

// #534: InboxProcessor は payload decode 失敗を SkipRetry で確定 fail に
// する (壊れた payload を retry しても無限ループするため)。
func TestInboxProcessor_DecodeFailureIsSkipRetry(t *testing.T) {
	p := processors.NewInboxProcessor(&stubFedProcessor{})

	err := p.Handle(context.Background(), driver.RawTask{
		TypeName: queue.TaskTypeInbox,
		Body:     []byte(`{not json`),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, driver.SkipRetry),
		"malformed payload should bubble up driver.SkipRetry to suppress retries")
}

// 正常 path: federation.Processor.Process が nil を返せば Handle も nil。
func TestInboxProcessor_HappyPathDelegatesToFederation(t *testing.T) {
	stub := &stubFedProcessor{}
	p := processors.NewInboxProcessor(stub)

	body := []byte(`{"type":"Follow","actor":"https://e/u/a"}`)
	require.NoError(t, p.Handle(context.Background(), driver.RawTask{
		TypeName: queue.TaskTypeInbox,
		Body:     mustEncode(t, queue.InboxPayload{Body: body, Host: "e"}),
	}))
	require.Len(t, stub.calls, 1)
	assert.JSONEq(t, string(body), string(stub.calls[0]))
}

// ErrUnsupportedActivity は HTTP 同期処理時と同じく成功扱い。worker が
// 永続 retry に持ち込まないようにするため。
func TestInboxProcessor_UnsupportedActivityIsSwallowed(t *testing.T) {
	stub := &stubFedProcessor{returnFn: func(_ []byte) error {
		return federation.ErrUnsupportedActivity
	}}
	p := processors.NewInboxProcessor(stub)

	err := p.Handle(context.Background(), driver.RawTask{
		TypeName: queue.TaskTypeInbox,
		Body:     mustEncode(t, queue.InboxPayload{Body: []byte(`{}`)}),
	})
	assert.NoError(t, err, "unsupported type should not fail the worker")
}

// 任意 error は driver の retry policy (inboxJobMaxAttempts) に任せるため
// そのまま返す (SkipRetry を付けない)。
func TestInboxProcessor_GenericErrorPropagatesForRetry(t *testing.T) {
	boom := errors.New("transient db error")
	stub := &stubFedProcessor{returnFn: func(_ []byte) error { return boom }}
	p := processors.NewInboxProcessor(stub)

	err := p.Handle(context.Background(), driver.RawTask{
		TypeName: queue.TaskTypeInbox,
		Body:     mustEncode(t, queue.InboxPayload{Body: []byte(`{}`)}),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, boom), "underlying error must be wrapped, not swallowed")
	assert.False(t, errors.Is(err, driver.SkipRetry),
		"transient errors should NOT carry SkipRetry — driver retry handles them")
}
