package chart

import "strings"

// Unflatten converts a flat dot-notation map into a nested object using
// the same rule as the upstream `nestedProperty.set()` calls in
// `getChart()`. The result is a map[string]any tree where intermediate
// nodes are themselves map[string]any and leaves are []int64 slices.
//
// Example:
//
//	{
//	  "local.total":      [1,2,3],
//	  "local.diffs.normal": [4,5,6],
//	  "remote.total":     [7,8,9],
//	}
//
//	becomes
//
//	{
//	  "local": {
//	    "total":  [1,2,3],
//	    "diffs": { "normal": [4,5,6] },
//	  },
//	  "remote": { "total": [7,8,9] },
//	}
//
// The output type is `map[string]any` so that it serializes to JSON
// matching the Misskey-js client expectations.
func Unflatten(flat Result) map[string]any {
	root := make(map[string]any)
	for key, values := range flat {
		setNested(root, strings.Split(key, "."), values)
	}
	return root
}

// setNested walks the path slice creating intermediate maps as needed
// and finally assigns the leaf slice. If a non-map value already exists
// at an intermediate path it is overwritten — this matches the upstream
// behaviour where nested-property silently replaces incompatible nodes.
func setNested(parent map[string]any, path []string, leaf []int64) {
	for i, key := range path {
		if i == len(path)-1 {
			parent[key] = leaf
			return
		}
		next, ok := parent[key].(map[string]any)
		if !ok {
			next = make(map[string]any)
			parent[key] = next
		}
		parent = next
	}
}
