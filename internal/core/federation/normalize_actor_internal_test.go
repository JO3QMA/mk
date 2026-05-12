package federation

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// upstream Misskey #17340 (triage #999) の `normalizeActor` helper を直接
// 検証する white-box test。black-box test (TestProcess_AcceptFollow_InnerActor
// EmbeddedObject) は handleAccept の error-swallow path により「inner.normalizeActor
// が消えても green」になる弱さがあるため、helper の input/output を直接 assert
// することで「embedded object → ID 展開」の hardening が本当に動いていることを
// seal する。
func TestNormalizeActor(t *testing.T) {
	// Process() は `_ = json.Unmarshal(body, &act)` で type mismatch error を
	// 故意に swallow する。embedded object 形式の actor では Go の json パッケージは
	// "cannot unmarshal object into Go struct field" を返すが、他 field は best-effort
	// で埋まる仕様。test もこの contract に従って Unmarshal error は許容する。

	t.Run("string_actor_kept_as_is", func(t *testing.T) {
		raw := []byte(`{"actor": "https://example.com/users/alice"}`)
		var act genericActivity
		require.NoError(t, json.Unmarshal(raw, &act))
		// string form は Unmarshal で直接埋まる
		assert.Equal(t, "https://example.com/users/alice", act.Actor)
		// normalizeActor は no-op (= 既に埋まっているので副作用なし)
		act.normalizeActor(raw)
		assert.Equal(t, "https://example.com/users/alice", act.Actor)
	})

	t.Run("embedded_object_actor_extracted", func(t *testing.T) {
		raw := []byte(`{"actor": {"id": "https://example.com/users/alice", "type": "Person"}}`)
		var act genericActivity
		// type mismatch で error を返すが、production と同じく swallow する。
		_ = json.Unmarshal(raw, &act)
		assert.Empty(t, act.Actor, "embedded actor object should not populate string field via Unmarshal")
		act.normalizeActor(raw)
		// normalizeActor で id field が抽出される (これが #999 fix の本体)
		assert.Equal(t, "https://example.com/users/alice", act.Actor)
	})

	t.Run("embedded_object_without_id_remains_empty", func(t *testing.T) {
		raw := []byte(`{"actor": {"type": "Person", "name": "alice"}}`)
		var act genericActivity
		_ = json.Unmarshal(raw, &act)
		act.normalizeActor(raw)
		// id field が無い object は extract できないので Actor は空のまま
		// (= Process() 側で "activity missing actor" として弾かれる動線)
		assert.Empty(t, act.Actor)
	})

	t.Run("absent_actor_field_remains_empty", func(t *testing.T) {
		raw := []byte(`{"type": "Follow", "object": "x"}`)
		var act genericActivity
		require.NoError(t, json.Unmarshal(raw, &act))
		act.normalizeActor(raw)
		assert.Empty(t, act.Actor)
	})

	t.Run("malformed_input_does_not_panic", func(t *testing.T) {
		var act genericActivity
		// 不正 JSON でも panic せず Actor が空のままになるだけ
		assert.NotPanics(t, func() {
			act.normalizeActor([]byte(`{not json`))
		})
		assert.Empty(t, act.Actor)
	})
}
