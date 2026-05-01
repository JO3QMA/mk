package admin

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/lib/pq"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/stretchr/testify/assert"
)

// TestMetaArrayColumns_CoversAllStringArrayFields は metaArrayColumns set が
// model.Meta の全 pq.StringArray (= varchar[]) 列を漏れなく被覆していること
// を reflection で担保する (#592)。
//
// 背景: #590 で追加した coerceMetaArrayFields は metaArrayColumns に列挙
// された列のみを処理するため、新しい varchar[] 列を model.Meta に追加した
// ときに set の更新を忘れると、admin/update-meta から保存しても永続化
// されない / null で UPDATE 全体が rollback する Bug A.1 が静かに再発する。
//
// 双方向の差分を assert することで、漏れ (varchar[] 列なのに set 未登録)
// と過剰 (set にあるが model に存在しない列、typo 等) を両方検出する。
func TestMetaArrayColumns_CoversAllStringArrayFields(t *testing.T) {
	metaType := reflect.TypeOf(model.Meta{})
	pqStringArrayType := reflect.TypeOf(pq.StringArray{})

	// model.Meta から pq.StringArray 型のフィールドの gorm column 名を集める。
	expected := map[string]struct{}{}
	for i := 0; i < metaType.NumField(); i++ {
		f := metaType.Field(i)
		if f.Type != pqStringArrayType {
			continue
		}
		col := extractGormColumn(f.Tag.Get("gorm"))
		if col == "" {
			t.Fatalf("model.Meta.%s に column タグが無い", f.Name)
		}
		expected[col] = struct{}{}
	}

	// 漏れ: model.Meta に varchar[] 列があるのに metaArrayColumns に登録なし。
	missing := diffSorted(expected, metaArrayColumns)
	assert.Empty(t, missing,
		"metaArrayColumns に未登録の varchar[] 列がある。新しい列を追加した際は internal/api/admin/handler.go の metaArrayColumns も更新してください")

	// 過剰: metaArrayColumns にあるが model.Meta に対応する pq.StringArray が無い。
	// typo / 列削除し忘れの検出用。
	extra := diffSorted(metaArrayColumns, expected)
	assert.Empty(t, extra,
		"metaArrayColumns に存在しない列が混入している (typo もしくは model.Meta から削除済の残骸の可能性)")
}

// extractGormColumn は GORM の struct tag (`gorm:"column:foo;type:..."`) から
// column 名 ("foo") のみを抽出する。
func extractGormColumn(tag string) string {
	for _, kv := range strings.Split(tag, ";") {
		if v, ok := strings.CutPrefix(kv, "column:"); ok {
			return v
		}
	}
	return ""
}

// diffSorted returns the keys in `a` that are not in `b`, sorted.
func diffSorted(a, b map[string]struct{}) []string {
	out := []string{}
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
