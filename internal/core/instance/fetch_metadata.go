package instance

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/shiroha-a/mk/internal/repository"
)

// HTTPFetcher abstracts the HTTP client used to fetch nodeinfo documents.
// 実装は activitypub.Client の薄いラッパで十分。
type HTTPFetcher interface {
	FetchObject(uri string) ([]byte, error)
}

// FetchMetadataService fetches /.well-known/nodeinfo for a remote host and
// updates the corresponding instance row with the parsed metadata.
type FetchMetadataService struct {
	repo    repository.InstanceRepository
	fetcher HTTPFetcher
	clock   func() time.Time
}

// NewFetchMetadataService constructs a FetchMetadataService.
func NewFetchMetadataService(repo repository.InstanceRepository, fetcher HTTPFetcher) *FetchMetadataService {
	return &FetchMetadataService{repo: repo, fetcher: fetcher, clock: time.Now}
}

// SetClock overrides the time source. Intended for tests.
func (s *FetchMetadataService) SetClock(now func() time.Time) {
	if now != nil {
		s.clock = now
	}
}

// nodeinfoDiscovery is the JSON shape of /.well-known/nodeinfo.
type nodeinfoDiscovery struct {
	Links []struct {
		Rel  string `json:"rel"`
		Href string `json:"href"`
	} `json:"links"`
}

// nodeinfoDocument is the JSON shape of nodeinfo 2.0/2.1.
type nodeinfoDocument struct {
	Software struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"software"`
	OpenRegistrations *bool `json:"openRegistrations"`
	Metadata          struct {
		NodeName        string `json:"nodeName"`
		NodeDescription string `json:"nodeDescription"`
		ThemeColor      string `json:"themeColor"`
	} `json:"metadata"`
}

// preferredRels lists the nodeinfo schema versions in order of preference.
// 2.1 → 2.0 → 1.0 の順で fallback する。
var preferredRels = []string{
	"http://nodeinfo.diaspora.software/ns/schema/2.1",
	"http://nodeinfo.diaspora.software/ns/schema/2.0",
}

// Fetch retrieves nodeinfo for the given host and applies the parsed metadata
// to the instance row. Instance row が存在しない場合は ErrInstanceNotFound。
func (s *FetchMetadataService) Fetch(host string) error {
	if host == "" {
		return errors.New("host is required")
	}
	if _, err := s.repo.FindByHost(host); err != nil {
		return ErrInstanceNotFound
	}

	disc, err := s.fetchDiscovery(host)
	if err != nil {
		return err
	}
	href := selectNodeinfoHref(disc)
	if href == "" {
		return errors.New("no supported nodeinfo schema")
	}

	doc, err := s.fetchDocument(href)
	if err != nil {
		return err
	}

	now := s.clock()
	fields := map[string]any{
		"infoUpdatedAt": &now,
	}
	if doc.Software.Name != "" {
		v := doc.Software.Name
		fields["softwareName"] = &v
	}
	if doc.Software.Version != "" {
		v := doc.Software.Version
		fields["softwareVersion"] = &v
	}
	if doc.OpenRegistrations != nil {
		fields["openRegistrations"] = doc.OpenRegistrations
	}
	if doc.Metadata.NodeName != "" {
		v := doc.Metadata.NodeName
		fields["name"] = &v
	}
	if doc.Metadata.NodeDescription != "" {
		v := doc.Metadata.NodeDescription
		fields["description"] = &v
	}
	if doc.Metadata.ThemeColor != "" {
		v := doc.Metadata.ThemeColor
		fields["themeColor"] = &v
	}
	return s.repo.UpdateFields(host, fields)
}

// fetchDiscovery fetches /.well-known/nodeinfo and decodes the link list.
func (s *FetchMetadataService) fetchDiscovery(host string) (*nodeinfoDiscovery, error) {
	body, err := s.fetcher.FetchObject("https://" + host + "/.well-known/nodeinfo")
	if err != nil {
		return nil, err
	}
	var disc nodeinfoDiscovery
	if err := json.Unmarshal(body, &disc); err != nil {
		return nil, err
	}
	return &disc, nil
}

// fetchDocument fetches the actual nodeinfo document.
func (s *FetchMetadataService) fetchDocument(href string) (*nodeinfoDocument, error) {
	body, err := s.fetcher.FetchObject(href)
	if err != nil {
		return nil, err
	}
	var doc nodeinfoDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// selectNodeinfoHref picks the highest-priority schema URL from the discovery
// document. 未知の rel しか無い場合は空文字を返す。
func selectNodeinfoHref(disc *nodeinfoDiscovery) string {
	for _, want := range preferredRels {
		for _, link := range disc.Links {
			if link.Rel == want && link.Href != "" {
				return link.Href
			}
		}
	}
	return ""
}
