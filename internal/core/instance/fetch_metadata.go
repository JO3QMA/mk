package instance

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/shiroha-a/mk/internal/repository"
	"golang.org/x/net/html"
)

// HTTPFetcher abstracts the HTTP clients used to fetch nodeinfo documents and
// remote HTML. 実装は activitypub.Client の薄いラッパで十分。
type HTTPFetcher interface {
	FetchObject(uri string) ([]byte, error)
	FetchHTML(uri string) ([]byte, error)
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

	// nodeinfoはicon/faviconを含まないため、リモートトップページHTMLから
	// 抽出する。取得失敗は致命ではない (nodeinfo 情報のpersistは継続する)。
	// DB側 varchar(256) 制約に引っかかるとUPDATE全体が失敗してnodeinfoまで
	// 失うため長さチェックを必ずかける (攻撃者制御の長いCDN URLを想定)。
	iconURL, faviconURL := s.fetchIcons(host)
	if iconURL != "" && len(iconURL) <= maxInstanceURLLen {
		fields["iconUrl"] = &iconURL
	}
	if faviconURL != "" && len(faviconURL) <= maxInstanceURLLen {
		fields["faviconUrl"] = &faviconURL
	}

	return s.repo.UpdateFields(host, fields)
}

// maxInstanceURLLen は instance.iconUrl / faviconUrl カラムの varchar(256)
// 制約と一致させる。これを超えるURLは無視する (DB エラーで他 field まで
// 失わないため)。
const maxInstanceURLLen = 256

// fetchIcons attempts to extract iconUrl (from <link rel="icon">) and set
// faviconUrl to the conventional /favicon.ico path. 本家Misskey の
// FetchInstanceMetadataService と同様、faviconは実在検証せず `https://host/favicon.ico`
// を決め打ちで保存する (frontendが404時は非表示になるだけなので害は小さい)。
func (s *FetchMetadataService) fetchIcons(host string) (iconURL, faviconURL string) {
	rootURL := "https://" + host + "/"
	// 画面に表示される favicon は frontend で fallback されるので、HTMLが
	// 取得できなくても /favicon.ico は必ずセットする。
	faviconURL = "https://" + host + "/favicon.ico"

	body, err := s.fetcher.FetchHTML(rootURL)
	if err != nil {
		return "", faviconURL
	}
	iconURL = parseIconFromHTML(body, rootURL)
	return iconURL, faviconURL
}

// parseIconFromHTML finds the first <link rel="icon"> or <link rel="apple-touch-icon">
// in doc and returns its href resolved against pageURL. 解析失敗や該当なしは空文字。
func parseIconFromHTML(body []byte, pageURL string) string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	base, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	var icon, appleTouchIcon string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "link" {
			rel := strings.ToLower(attrValue(n, "rel"))
			href := attrValue(n, "href")
			if href != "" {
				switch rel {
				case "icon", "shortcut icon":
					if icon == "" {
						icon = href
					}
				case "apple-touch-icon", "apple-touch-icon-precomposed":
					if appleTouchIcon == "" {
						appleTouchIcon = href
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// 本家優先順位に倣い、appleTouchIconの方が高解像度で綺麗なため優先する。
	chosen := firstNonEmptyStr(appleTouchIcon, icon)
	if chosen == "" {
		return ""
	}
	ref, err := url.Parse(chosen)
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).String()
}

func attrValue(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
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
