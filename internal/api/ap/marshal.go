package ap

import "encoding/json"

// mustMarshal serializes any value to JSON, panicking on error. AP types in
// this package are simple structs that always marshal successfully so the
// panic case is unreachable.
func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
