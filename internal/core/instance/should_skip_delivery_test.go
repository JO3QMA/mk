package instance_test

import (
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestService_ShouldSkipDelivery(t *testing.T) {
	t.Run("empty host is not skipped", func(t *testing.T) {
		svc, _, _ := newService(t)
		assert.False(t, svc.ShouldSkipDelivery(""))
	})

	t.Run("blocked host is skipped", func(t *testing.T) {
		svc, _, metaRepo := newService(t)
		metaRepo.Meta.BlockedHosts = []string{"blocked.example"}
		assert.True(t, svc.ShouldSkipDelivery("blocked.example"))
	})

	t.Run("federation none denies all hosts", func(t *testing.T) {
		svc, _, metaRepo := newService(t)
		metaRepo.Meta.Federation = "none"
		assert.True(t, svc.ShouldSkipDelivery("any.example"))
	})

	t.Run("manually suspended instance is skipped", func(t *testing.T) {
		svc, repo, _ := newService(t)
		repo.Instances["sus.example"] = &model.Instance{
			ID:              "i1",
			Host:            "sus.example",
			SuspensionState: model.SuspensionStateManuallySuspended,
		}
		assert.True(t, svc.ShouldSkipDelivery("sus.example"))
	})

	t.Run("active instance is delivered", func(t *testing.T) {
		svc, repo, _ := newService(t)
		repo.Instances["ok.example"] = &model.Instance{
			ID:              "i2",
			Host:            "ok.example",
			SuspensionState: model.SuspensionStateNone,
		}
		assert.False(t, svc.ShouldSkipDelivery("ok.example"))
	})

	t.Run("unknown instance is not skipped", func(t *testing.T) {
		svc, _, _ := newService(t)
		assert.False(t, svc.ShouldSkipDelivery("unknown.example"))
	})
}
