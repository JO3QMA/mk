package processors_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/queue/driver"
	"github.com/shiroha-a/mk/internal/queue/processors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #534: InboxProcessor は payload decode 失敗を SkipRetry で確定 fail に
// する (壊れた payload を retry しても無限ループするため)。
func TestInboxProcessor_DecodeFailureIsSkipRetry(t *testing.T) {
	p := processors.NewInboxProcessor(nil) // processor は decode 経路で touch されない

	err := p.Handle(context.Background(), driver.RawTask{
		TypeName: queue.TaskTypeInbox,
		Body:     []byte(`{not json`),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, driver.SkipRetry),
		"malformed payload should bubble up driver.SkipRetry to suppress retries")
}

// payload 正常 + processor nil で nil dereference panic にならないこと
// は実際 nil processor を Handle に渡すと panic するので、最小限の
// 動作確認は federation/processor 統合テスト側で行う。
// 本ファイルは SkipRetry 経路だけ単体で carve out する。
