package search

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	meili "github.com/meilisearch/meilisearch-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMeiliServer spins up an httptest.Server that pretends to be a real
// Meilisearch instance. It records the last request seen on each endpoint and
// can be configured to fail.
type fakeMeiliServer struct {
	server *httptest.Server

	// last requests for assertion in tests
	lastAddBody    []byte
	lastDeletePath string
	lastSearchBody []byte
	lastSettings   []byte

	// configurable error injection
	failAdd      bool
	failDelete   bool
	failSearch   bool
	failSettings bool

	// hits returned by the next /search call
	hitsJSON string
}

func newFakeMeiliServer() *fakeMeiliServer {
	f := &fakeMeiliServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/indexes/notes/documents", func(w http.ResponseWriter, r *http.Request) {
		if f.failAdd {
			http.Error(w, `{"message":"boom","code":"x","type":"y"}`, http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(r.Body)
		f.lastAddBody = body
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"taskUid":1,"indexUid":"notes","status":"enqueued","type":"documentAdditionOrUpdate"}`))
	})
	mux.HandleFunc("/indexes/notes/documents/", func(w http.ResponseWriter, r *http.Request) {
		// /indexes/notes/documents/{id} → DELETE
		if r.Method != http.MethodDelete {
			http.NotFound(w, r)
			return
		}
		if f.failDelete {
			http.Error(w, `{"message":"boom","code":"x","type":"y"}`, http.StatusBadRequest)
			return
		}
		f.lastDeletePath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"taskUid":2,"indexUid":"notes","status":"enqueued","type":"documentDeletion"}`))
	})
	mux.HandleFunc("/indexes/notes/search", func(w http.ResponseWriter, r *http.Request) {
		if f.failSearch {
			http.Error(w, `{"message":"boom","code":"x","type":"y"}`, http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(r.Body)
		f.lastSearchBody = body
		w.WriteHeader(http.StatusOK)
		hits := f.hitsJSON
		if hits == "" {
			hits = `[]`
		}
		_, _ = w.Write([]byte(`{"hits":` + hits + `,"query":"x","processingTimeMs":1}`))
	})
	mux.HandleFunc("/indexes/notes/settings", func(w http.ResponseWriter, r *http.Request) {
		if f.failSettings {
			http.Error(w, `{"message":"boom","code":"x","type":"y"}`, http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(r.Body)
		f.lastSettings = body
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"taskUid":3,"indexUid":"notes","status":"enqueued","type":"settingsUpdate"}`))
	})
	f.server = httptest.NewServer(mux)
	return f
}

func (f *fakeMeiliServer) close() { f.server.Close() }

func (f *fakeMeiliServer) client() *MeilisearchClient {
	svc := NewMeilisearchService(f.server.URL, "")
	return OpenIndex(svc, "notes")
}

func TestMeilisearchClient_AddDocumentsHappyPath(t *testing.T) {
	srv := newFakeMeiliServer()
	defer srv.close()
	client := srv.client()
	docs := []NoteDocument{{ID: "n1", UserID: "u1"}}
	require.NoError(t, client.AddDocuments(docs))
	assert.NotEmpty(t, srv.lastAddBody)
}

func TestMeilisearchClient_AddDocumentsEmptySliceNoOp(t *testing.T) {
	srv := newFakeMeiliServer()
	defer srv.close()
	client := srv.client()
	require.NoError(t, client.AddDocuments(nil))
	assert.Empty(t, srv.lastAddBody)
}

func TestMeilisearchClient_AddDocumentsErrorPropagated(t *testing.T) {
	srv := newFakeMeiliServer()
	defer srv.close()
	srv.failAdd = true
	client := srv.client()
	require.Error(t, client.AddDocuments([]NoteDocument{{ID: "n1"}}))
}

func TestMeilisearchClient_AddDocumentsNilReceiverNoOp(t *testing.T) {
	var c *MeilisearchClient
	require.NoError(t, c.AddDocuments([]NoteDocument{{ID: "n1"}}))
}

func TestMeilisearchClient_AddDocumentsNilIndexNoOp(t *testing.T) {
	c := &MeilisearchClient{}
	require.NoError(t, c.AddDocuments([]NoteDocument{{ID: "n1"}}))
}

func TestMeilisearchClient_DeleteDocumentHappyPath(t *testing.T) {
	srv := newFakeMeiliServer()
	defer srv.close()
	require.NoError(t, srv.client().DeleteDocument("n1"))
	assert.True(t, strings.HasSuffix(srv.lastDeletePath, "/n1"))
}

func TestMeilisearchClient_DeleteDocumentErrorPropagated(t *testing.T) {
	srv := newFakeMeiliServer()
	defer srv.close()
	srv.failDelete = true
	require.Error(t, srv.client().DeleteDocument("n1"))
}

func TestMeilisearchClient_DeleteDocumentNilSafe(t *testing.T) {
	var c *MeilisearchClient
	require.NoError(t, c.DeleteDocument("n1"))
	require.NoError(t, (&MeilisearchClient{}).DeleteDocument("n1"))
}

func TestMeilisearchClient_SearchHappyPath(t *testing.T) {
	srv := newFakeMeiliServer()
	defer srv.close()
	srv.hitsJSON = `[{"id":"n1","createdAt":1700000000000}]`
	resp, err := srv.client().Search("hello", IndexSearchRequest{
		Filter:               "userId = 'u1'",
		Sort:                 []string{"createdAt:desc"},
		Limit:                5,
		AttributesToRetrieve: []string{"id", "createdAt"},
		MatchingStrategy:     "all",
	})
	require.NoError(t, err)
	require.Len(t, resp.Hits, 1)
	assert.Equal(t, "n1", resp.Hits[0].ID)
	assert.Equal(t, int64(1700000000000), resp.Hits[0].CreatedAt)

	// 送られた SearchRequest の Filter が伝わっていること。
	var sent map[string]any
	require.NoError(t, json.Unmarshal(srv.lastSearchBody, &sent))
	assert.Equal(t, "userId = 'u1'", sent["filter"])
}

func TestMeilisearchClient_SearchErrorPropagated(t *testing.T) {
	srv := newFakeMeiliServer()
	defer srv.close()
	srv.failSearch = true
	_, err := srv.client().Search("x", IndexSearchRequest{Limit: 1})
	require.Error(t, err)
}

func TestMeilisearchClient_SearchNilSafe(t *testing.T) {
	var c *MeilisearchClient
	resp, err := c.Search("x", IndexSearchRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Hits)

	resp, err = (&MeilisearchClient{}).Search("x", IndexSearchRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.Hits)
}

func TestMeilisearchClient_SearchHitDecodeIDError(t *testing.T) {
	srv := newFakeMeiliServer()
	defer srv.close()
	// id is supposed to be a string; sending an array forces a decode error
	srv.hitsJSON = `[{"id":["bad"]}]`
	_, err := srv.client().Search("x", IndexSearchRequest{Limit: 1})
	require.Error(t, err)
}

func TestMeilisearchClient_SearchHitWithoutCreatedAtIsZero(t *testing.T) {
	srv := newFakeMeiliServer()
	defer srv.close()
	srv.hitsJSON = `[{"id":"n1"}]`
	resp, err := srv.client().Search("x", IndexSearchRequest{Limit: 1})
	require.NoError(t, err)
	require.Len(t, resp.Hits, 1)
	assert.Equal(t, int64(0), resp.Hits[0].CreatedAt)
}

func TestMeilisearchClient_SearchHitInvalidCreatedAtIsZero(t *testing.T) {
	srv := newFakeMeiliServer()
	defer srv.close()
	// createdAt as a string parses with json.Unmarshal-int → error → zero.
	srv.hitsJSON = `[{"id":"n1","createdAt":"bad"}]`
	resp, err := srv.client().Search("x", IndexSearchRequest{Limit: 1})
	require.NoError(t, err)
	require.Len(t, resp.Hits, 1)
	assert.Equal(t, int64(0), resp.Hits[0].CreatedAt)
}

func TestMeilisearchClient_UpdateSettingsHappyPath(t *testing.T) {
	srv := newFakeMeiliServer()
	defer srv.close()
	require.NoError(t, srv.client().UpdateSettings(IndexSettings{
		SearchableAttributes: []string{"text"},
		PaginationMaxHits:    100,
		TypoToleranceEnabled: false,
	}))
	assert.Contains(t, string(srv.lastSettings), "searchableAttributes")
	assert.Contains(t, string(srv.lastSettings), "pagination")
}

func TestMeilisearchClient_UpdateSettingsErrorPropagated(t *testing.T) {
	srv := newFakeMeiliServer()
	defer srv.close()
	srv.failSettings = true
	require.Error(t, srv.client().UpdateSettings(IndexSettings{}))
}

func TestMeilisearchClient_UpdateSettingsNilSafe(t *testing.T) {
	var c *MeilisearchClient
	require.NoError(t, c.UpdateSettings(IndexSettings{}))
	require.NoError(t, (&MeilisearchClient{}).UpdateSettings(IndexSettings{}))
}

func TestMeilisearchClient_NewMeilisearchServiceWithAPIKey(t *testing.T) {
	// Just exercise the apiKey branch — we are not validating headers here.
	svc := NewMeilisearchService("http://localhost:7700", "secret")
	require.NotNil(t, svc)
	assert.NotNil(t, OpenIndex(svc, "notes"))
}

func TestMeilisearchClient_NewMeilisearchServiceWithoutAPIKey(t *testing.T) {
	svc := NewMeilisearchService("http://localhost:7700", "")
	require.NotNil(t, svc)
	// Confirm we can also wrap a real meilisearch.IndexManager directly.
	idx := svc.Index("notes")
	require.NotNil(t, idx)
	c := &MeilisearchClient{index: idx}
	// Operations against an unreachable server return errors but exercise the
	// non-nil branch.
	_ = c
	var _ MeilisearchIndex = c
	// Sanity check the SDK type imports compile.
	_ = meili.All
}
