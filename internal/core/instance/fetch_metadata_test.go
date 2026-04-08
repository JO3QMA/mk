package instance_test

import (
	"errors"
	"testing"
	"time"

	"github.com/shiroha-a/mk/internal/core/instance"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedFetcher returns canned responses keyed by call order.
type scriptedFetcher struct {
	bodies [][]byte
	errs   []error
	idx    int
}

func (s *scriptedFetcher) FetchObject(_ string) ([]byte, error) {
	if s.idx >= len(s.bodies) {
		return nil, errors.New("no more bodies")
	}
	b := s.bodies[s.idx]
	var err error
	if s.idx < len(s.errs) {
		err = s.errs[s.idx]
	}
	s.idx++
	return b, err
}

func newFetchSvc(t *testing.T, bodies [][]byte, errs []error) (*instance.FetchMetadataService, *testutil.MockInstanceRepository) {
	t.Helper()
	repo := testutil.NewMockInstanceRepository()
	fetcher := &scriptedFetcher{bodies: bodies, errs: errs}
	return instance.NewFetchMetadataService(repo, fetcher), repo
}

const discoveryBody = `{
	"links": [
		{"rel": "http://nodeinfo.diaspora.software/ns/schema/2.0", "href": "https://remote.example/nodeinfo/2.0"},
		{"rel": "http://nodeinfo.diaspora.software/ns/schema/2.1", "href": "https://remote.example/nodeinfo/2.1"}
	]
}`

const documentBody = `{
	"software": {"name": "misskey", "version": "13.14.2"},
	"openRegistrations": true,
	"metadata": {
		"nodeName": "Remote",
		"nodeDescription": "A test instance",
		"themeColor": "#abcdef"
	}
}`

func TestFetch_HappyPath(t *testing.T) {
	svc, repo := newFetchSvc(t,
		[][]byte{[]byte(discoveryBody), []byte(documentBody)}, nil)
	repo.Instances["remote.example"] = &model.Instance{
		ID: "i1", Host: "remote.example",
	}
	require.NoError(t, svc.Fetch("remote.example"))

	got := repo.Instances["remote.example"]
	require.NotNil(t, got.SoftwareName)
	assert.Equal(t, "misskey", *got.SoftwareName)
	require.NotNil(t, got.SoftwareVersion)
	assert.Equal(t, "13.14.2", *got.SoftwareVersion)
	require.NotNil(t, got.OpenRegistrations)
	assert.True(t, *got.OpenRegistrations)
	require.NotNil(t, got.Name)
	assert.Equal(t, "Remote", *got.Name)
	require.NotNil(t, got.Description)
	require.NotNil(t, got.ThemeColor)
	require.NotNil(t, got.InfoUpdatedAt)
}

func TestFetch_EmptyHost(t *testing.T) {
	svc, _ := newFetchSvc(t, nil, nil)
	err := svc.Fetch("")
	assert.Error(t, err)
}

func TestFetch_InstanceNotFound(t *testing.T) {
	svc, _ := newFetchSvc(t, nil, nil)
	err := svc.Fetch("missing.example")
	assert.ErrorIs(t, err, instance.ErrInstanceNotFound)
}

func TestFetch_DiscoveryError(t *testing.T) {
	svc, repo := newFetchSvc(t, [][]byte{nil}, []error{errors.New("net down")})
	repo.Instances["remote.example"] = &model.Instance{ID: "i1", Host: "remote.example"}
	err := svc.Fetch("remote.example")
	assert.Error(t, err)
}

func TestFetch_DiscoveryBadJSON(t *testing.T) {
	svc, repo := newFetchSvc(t, [][]byte{[]byte("{not json")}, nil)
	repo.Instances["remote.example"] = &model.Instance{ID: "i1", Host: "remote.example"}
	err := svc.Fetch("remote.example")
	assert.Error(t, err)
}

func TestFetch_NoSupportedSchema(t *testing.T) {
	svc, repo := newFetchSvc(t, [][]byte{[]byte(`{"links":[{"rel":"x","href":"y"}]}`)}, nil)
	repo.Instances["remote.example"] = &model.Instance{ID: "i1", Host: "remote.example"}
	err := svc.Fetch("remote.example")
	assert.Error(t, err)
}

func TestFetch_DocumentError(t *testing.T) {
	svc, repo := newFetchSvc(t,
		[][]byte{[]byte(discoveryBody), nil},
		[]error{nil, errors.New("doc fail")})
	repo.Instances["remote.example"] = &model.Instance{ID: "i1", Host: "remote.example"}
	err := svc.Fetch("remote.example")
	assert.Error(t, err)
}

func TestFetch_DocumentBadJSON(t *testing.T) {
	svc, repo := newFetchSvc(t,
		[][]byte{[]byte(discoveryBody), []byte("{not json")}, nil)
	repo.Instances["remote.example"] = &model.Instance{ID: "i1", Host: "remote.example"}
	err := svc.Fetch("remote.example")
	assert.Error(t, err)
}

func TestFetch_OnlyVersion20(t *testing.T) {
	disc := `{"links":[{"rel":"http://nodeinfo.diaspora.software/ns/schema/2.0","href":"https://remote.example/nodeinfo/2.0"}]}`
	svc, repo := newFetchSvc(t,
		[][]byte{[]byte(disc), []byte(documentBody)}, nil)
	repo.Instances["remote.example"] = &model.Instance{ID: "i1", Host: "remote.example"}
	require.NoError(t, svc.Fetch("remote.example"))
}

func TestFetch_DocumentMinimalFields(t *testing.T) {
	doc := `{"software":{"name":"misskey","version":""},"metadata":{}}`
	svc, repo := newFetchSvc(t,
		[][]byte{[]byte(discoveryBody), []byte(doc)}, nil)
	repo.Instances["remote.example"] = &model.Instance{ID: "i1", Host: "remote.example"}
	require.NoError(t, svc.Fetch("remote.example"))
	got := repo.Instances["remote.example"]
	require.NotNil(t, got.SoftwareName)
	assert.Nil(t, got.SoftwareVersion)
	assert.Nil(t, got.Name)
	assert.Nil(t, got.Description)
	assert.Nil(t, got.ThemeColor)
}

func TestFetch_SetClock(t *testing.T) {
	svc, repo := newFetchSvc(t,
		[][]byte{[]byte(discoveryBody), []byte(documentBody)}, nil)
	repo.Instances["remote.example"] = &model.Instance{ID: "i1", Host: "remote.example"}
	fixed := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return fixed })
	svc.SetClock(nil) // nil 渡し無視
	require.NoError(t, svc.Fetch("remote.example"))
	require.NotNil(t, repo.Instances["remote.example"].InfoUpdatedAt)
	assert.Equal(t, fixed, *repo.Instances["remote.example"].InfoUpdatedAt)
}
