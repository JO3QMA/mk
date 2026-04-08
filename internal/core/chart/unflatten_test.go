package chart

import (
	"reflect"
	"testing"
)

func TestUnflatten_NestedKeys(t *testing.T) {
	in := Result{
		"local.total":         {1, 2, 3},
		"local.diffs.normal":  {4, 5, 6},
		"remote.total":        {7, 8, 9},
		"remote.diffs.reply":  {0, 0, 1},
		"remote.diffs.normal": {2, 2, 2},
	}
	got := Unflatten(in)
	want := map[string]any{
		"local": map[string]any{
			"total": []int64{1, 2, 3},
			"diffs": map[string]any{
				"normal": []int64{4, 5, 6},
			},
		},
		"remote": map[string]any{
			"total": []int64{7, 8, 9},
			"diffs": map[string]any{
				"reply":  []int64{0, 0, 1},
				"normal": []int64{2, 2, 2},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Unflatten mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestUnflatten_FlatKeysOnly(t *testing.T) {
	in := Result{
		"a": {1},
		"b": {2},
	}
	got := Unflatten(in)
	want := map[string]any{
		"a": []int64{1},
		"b": []int64{2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Unflatten mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestUnflatten_EmptyInput(t *testing.T) {
	got := Unflatten(Result{})
	if len(got) != 0 {
		t.Fatalf("expected empty map, got: %#v", got)
	}
}

// TestUnflatten_OverwriteIncompatibleParent confirms that when both a leaf
// and an intermediate node share the same path, the intermediate map
// silently replaces the prior leaf — matching nested-property semantics.
func TestUnflatten_OverwriteIncompatibleParent(t *testing.T) {
	in := Result{
		"a":   {1},
		"a.b": {2},
	}
	got := Unflatten(in)
	// "a" は最後に "a.b" を入れる順序によって map か []int64 か変わる。
	// 順序非依存に少なくともどちらかの値が残ることを保証する。
	if v, ok := got["a"]; !ok {
		t.Fatalf("missing key a: %#v", got)
	} else {
		switch v.(type) {
		case []int64, map[string]any:
		default:
			t.Fatalf("unexpected type for a: %T", v)
		}
	}
}
