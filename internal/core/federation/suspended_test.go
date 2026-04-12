package federation

import (
	"testing"

	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestMatchesSuspendedSoftware_WildcardVersion(t *testing.T) {
	name := "mastodon"
	ver := "4.2.0"
	inst := &model.Instance{SoftwareName: &name, SoftwareVersion: &ver}
	entries := []SuspendedSoftwareEntry{{Software: "Mastodon", VersionRange: "*"}}
	assert.True(t, matchesSuspendedSoftware(inst, entries))
}

func TestMatchesSuspendedSoftware_ExactVersion(t *testing.T) {
	name := "pixelfed"
	ver := "0.12.4"
	inst := &model.Instance{SoftwareName: &name, SoftwareVersion: &ver}
	entries := []SuspendedSoftwareEntry{{Software: "Pixelfed", VersionRange: "0.12.4"}}
	assert.True(t, matchesSuspendedSoftware(inst, entries))
}

func TestMatchesSuspendedSoftware_PrefixMatch(t *testing.T) {
	name := "pleroma"
	ver := "2.6.3-beta1"
	inst := &model.Instance{SoftwareName: &name, SoftwareVersion: &ver}
	entries := []SuspendedSoftwareEntry{{Software: "Pleroma", VersionRange: "2.6.3"}}
	assert.True(t, matchesSuspendedSoftware(inst, entries))
}

func TestMatchesSuspendedSoftware_NoMatch(t *testing.T) {
	name := "mastodon"
	ver := "4.3.0"
	inst := &model.Instance{SoftwareName: &name, SoftwareVersion: &ver}
	entries := []SuspendedSoftwareEntry{{Software: "Mastodon", VersionRange: "4.2.0"}}
	assert.False(t, matchesSuspendedSoftware(inst, entries))
}

func TestMatchesSuspendedSoftware_DifferentSoftware(t *testing.T) {
	name := "misskey"
	ver := "2024.1.0"
	inst := &model.Instance{SoftwareName: &name, SoftwareVersion: &ver}
	entries := []SuspendedSoftwareEntry{{Software: "Mastodon", VersionRange: "*"}}
	assert.False(t, matchesSuspendedSoftware(inst, entries))
}

func TestMatchesSuspendedSoftware_NilSoftwareName(t *testing.T) {
	inst := &model.Instance{SoftwareName: nil}
	entries := []SuspendedSoftwareEntry{{Software: "Mastodon", VersionRange: "*"}}
	assert.False(t, matchesSuspendedSoftware(inst, entries))
}

func TestMatchesSuspendedSoftware_NilVersion_Wildcard(t *testing.T) {
	name := "mastodon"
	inst := &model.Instance{SoftwareName: &name, SoftwareVersion: nil}
	entries := []SuspendedSoftwareEntry{{Software: "Mastodon", VersionRange: "*"}}
	assert.True(t, matchesSuspendedSoftware(inst, entries))
}

func TestMatchesSuspendedSoftware_NilVersion_Specific(t *testing.T) {
	name := "mastodon"
	inst := &model.Instance{SoftwareName: &name, SoftwareVersion: nil}
	entries := []SuspendedSoftwareEntry{{Software: "Mastodon", VersionRange: "4.2.0"}}
	assert.False(t, matchesSuspendedSoftware(inst, entries))
}

func TestMatchesSuspendedSoftware_EmptyEntries(t *testing.T) {
	name := "mastodon"
	ver := "4.2.0"
	inst := &model.Instance{SoftwareName: &name, SoftwareVersion: &ver}
	assert.False(t, matchesSuspendedSoftware(inst, nil))
}

func TestNewSuspendedChecker_InvalidJSON(t *testing.T) {
	c := NewSuspendedChecker([]byte("invalid"), nil)
	assert.Empty(t, c.entries)
}

func TestSuspendedChecker_EmptyHost(t *testing.T) {
	c := NewSuspendedChecker([]byte(`[{"software":"mastodon","versionRange":"*"}]`), nil)
	assert.False(t, c.IsSuspended(""))
}

func TestSuspendedChecker_EmptyEntries(t *testing.T) {
	c := NewSuspendedChecker([]byte(`[]`), nil)
	assert.False(t, c.IsSuspended("example.com"))
}
