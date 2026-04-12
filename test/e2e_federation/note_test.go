// note_test.go ports test-federation/test/note.test.ts
package e2e_federation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFederation_ResolveRemoteNote(t *testing.T) {
	resetDB(t, serverA)
	resetDB(t, serverB)

	alice := signup(t, serverA, "alice", nil)
	bob := signup(t, serverB, "bob", nil)

	// alice がサーバーAでノートを作成
	noteResult := srvAPICall(t, serverA, "notes/create", map[string]any{
		"i":    alice.Token,
		"text": "Hello from server A!",
	})
	createdNote, ok := noteResult["createdNote"].(map[string]any)
	require.True(t, ok, "createdNote should be a map")
	noteID, ok := createdNote["id"].(string)
	require.True(t, ok, "note id should be a string")

	t.Run("resolve remote note via ap/show", func(t *testing.T) {
		uri := noteURI(serverA, noteID)
		resolved := resolveRemoteNote(t, serverB, uri, bob)

		assert.Equal(t, "Hello from server A!", resolved["text"])
		assert.Equal(t, "public", resolved["visibility"])
	})

	t.Run("resolve same note twice returns consistent result", func(t *testing.T) {
		uri := noteURI(serverA, noteID)
		first := resolveRemoteNote(t, serverB, uri, bob)
		second := resolveRemoteNote(t, serverB, uri, bob)

		assert.Equal(t, first["id"], second["id"])
		assert.Equal(t, first["text"], second["text"])
	})
}

func TestFederation_NoteVisibility(t *testing.T) {
	resetDB(t, serverA)
	resetDB(t, serverB)

	alice := signup(t, serverA, "alice", nil)
	bob := signup(t, serverB, "bob", nil)

	t.Run("public note is resolvable", func(t *testing.T) {
		noteResult := srvAPICall(t, serverA, "notes/create", map[string]any{
			"i":          alice.Token,
			"text":       "public note",
			"visibility": "public",
		})
		createdNote := noteResult["createdNote"].(map[string]any)
		noteID := createdNote["id"].(string)

		uri := noteURI(serverA, noteID)
		resolved := resolveRemoteNote(t, serverB, uri, bob)
		assert.Equal(t, "public note", resolved["text"])
	})

	t.Run("home note is resolvable via direct URI", func(t *testing.T) {
		noteResult := srvAPICall(t, serverA, "notes/create", map[string]any{
			"i":          alice.Token,
			"text":       "home note",
			"visibility": "home",
		})
		createdNote := noteResult["createdNote"].(map[string]any)
		noteID := createdNote["id"].(string)

		// home visibility のノートもAP URIで直接取得可能
		// (GET /notes/:id はローカルの公開判定を行うが、home は閲覧可)
		uri := noteURI(serverA, noteID)
		resolved := resolveRemoteNote(t, serverB, uri, bob)
		assert.Equal(t, "home note", resolved["text"])
	})
}

func TestFederation_NoteContentConsistency(t *testing.T) {
	resetDB(t, serverA)
	resetDB(t, serverB)

	alice := signup(t, serverA, "alice", nil)
	bob := signup(t, serverB, "bob", nil)

	// CW付きノートを作成
	noteResult := srvAPICall(t, serverA, "notes/create", map[string]any{
		"i":    alice.Token,
		"text": "spoiler content",
		"cw":   "content warning",
	})
	createdNote := noteResult["createdNote"].(map[string]any)
	noteID := createdNote["id"].(string)

	// bob がリモートノートを解決
	uri := noteURI(serverA, noteID)
	resolved := resolveRemoteNote(t, serverB, uri, bob)

	// テキスト一貫性
	assert.Equal(t, "spoiler content", resolved["text"])
}
