package timeline

import (
	"github.com/shiroha-a/mk/internal/repository"
)

// metaRepoCacheLimits is the production MetaCacheLimitsProvider that reads
// the four cache-cap columns from the meta singleton on every call. The 4
// integer columns are cheap to fetch and the call site (FanoutHook.OnNoteCreated)
// is invoked once per note creation, so we accept the per-note round-trip
// rather than maintain an explicit cache that would need invalidation when
// the meta row is updated.
type metaRepoCacheLimits struct {
	repo repository.MetaRepository
}

// NewMetaRepoCacheLimits constructs a MetaCacheLimitsProvider backed by the
// given MetaRepository.
func NewMetaRepoCacheLimits(repo repository.MetaRepository) MetaCacheLimitsProvider {
	return &metaRepoCacheLimits{repo: repo}
}

// CacheLimits implements MetaCacheLimitsProvider. Errors are swallowed and
// returned as zero values; resolveCap then falls back to the documented
// Misskey defaults.
func (m *metaRepoCacheLimits) CacheLimits() CacheLimits {
	if m == nil || m.repo == nil {
		return CacheLimits{}
	}
	meta, err := m.repo.Fetch()
	if err != nil || meta == nil {
		return CacheLimits{}
	}
	return CacheLimits{
		LocalUserUserTimeline:  meta.PerLocalUserUserTimelineCacheMax,
		RemoteUserUserTimeline: meta.PerRemoteUserUserTimelineCacheMax,
		UserHomeTimeline:       meta.PerUserHomeTimelineCacheMax,
		UserListTimeline:       meta.PerUserListTimelineCacheMax,
	}
}
