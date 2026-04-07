package activitypub

import "io"

// SetRandReaderForTest replaces the package-level entropy source for tests.
// 戻り値は元の reader で、テスト終了時に呼び出して復元する。
func SetRandReaderForTest(r io.Reader) func() {
	prev := randReader
	randReader = r
	return func() { randReader = prev }
}
