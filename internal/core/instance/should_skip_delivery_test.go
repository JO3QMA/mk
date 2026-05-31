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

	t.Run("federation specified excludes non-listed host", func(t *testing.T) {
		svc, _, metaRepo := newService(t)
		metaRepo.Meta.Federation = "specified"
		metaRepo.Meta.FederationHosts = []string{"allowed.example"}
		assert.True(t, svc.ShouldSkipDelivery("other.example"))
		// allowlist に含まれる host は instance row が無ければ suspended でもないので false。
		assert.False(t, svc.ShouldSkipDelivery("allowed.example"))
	})

	t.Run("meta fetch error falls back to suspensionState only (fail-open on block)", func(t *testing.T) {
		// metaRepo.Meta == nil で Fetch が error を返す。block/federation 判定は
		// 抜けるが suspensionState 判定は後段で効くことを確認する (#1406 review)。
		svc, repo, metaRepo := newService(t)
		metaRepo.Meta = nil

		// 未登録 host は fail-open で配送される (skip しない)。
		assert.False(t, svc.ShouldSkipDelivery("unknown.example"))

		// meta 障害中でも suspensionState は DB 由来なので suspend は止まる。
		repo.Instances["sus.example"] = &model.Instance{
			ID:              "i3",
			Host:            "sus.example",
			SuspensionState: model.SuspensionStateManuallySuspended,
		}
		assert.True(t, svc.ShouldSkipDelivery("sus.example"))
	})
}
