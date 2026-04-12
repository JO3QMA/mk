package federation

import (
	"encoding/json"
	"strings"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/repository"
)

// SuspendedSoftwareEntry represents one entry in meta.deliverSuspendedSoftware.
type SuspendedSoftwareEntry struct {
	Software     string `json:"software"`
	VersionRange string `json:"versionRange"`
}

// SuspendedChecker determines whether delivery to an instance should be
// skipped based on its software name and version.
type SuspendedChecker struct {
	entries      []SuspendedSoftwareEntry
	instanceRepo repository.InstanceRepository
}

// NewSuspendedChecker creates a checker from the meta field.
func NewSuspendedChecker(metaJSON []byte, instanceRepo repository.InstanceRepository) *SuspendedChecker {
	var entries []SuspendedSoftwareEntry
	_ = json.Unmarshal(metaJSON, &entries)
	return &SuspendedChecker{entries: entries, instanceRepo: instanceRepo}
}

// IsSuspended reports whether delivery to the given host should be skipped.
// ホストの instance レコードから softwareName/softwareVersion を取得し、
// deliverSuspendedSoftware リストとマッチする。
func (c *SuspendedChecker) IsSuspended(host string) bool {
	if len(c.entries) == 0 || host == "" {
		return false
	}

	inst, err := c.instanceRepo.FindByHost(host)
	if err != nil {
		return false
	}

	return matchesSuspendedSoftware(inst, c.entries)
}

func matchesSuspendedSoftware(inst *model.Instance, entries []SuspendedSoftwareEntry) bool {
	if inst.SoftwareName == nil {
		return false
	}
	name := strings.ToLower(*inst.SoftwareName)

	for _, e := range entries {
		if strings.ToLower(e.Software) != name {
			continue
		}
		// versionRange が "*" なら全バージョンマッチ
		if e.VersionRange == "*" {
			return true
		}
		// softwareVersion が nil なら "*" 以外ではマッチしない
		if inst.SoftwareVersion == nil {
			continue
		}
		// 完全一致 or プレフィックスマッチ (簡易 semver — "1.2" は "1.2.3" にマッチ)
		ver := *inst.SoftwareVersion
		if ver == e.VersionRange || strings.HasPrefix(ver, e.VersionRange+".") || strings.HasPrefix(ver, e.VersionRange+"-") {
			return true
		}
	}
	return false
}
