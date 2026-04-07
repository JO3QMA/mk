package drive

import "io"

// SetRandReaderForTest replaces the package-level random source for tests.
// 戻り値は元のreaderで、テスト終了時に呼び出して復元する。
func SetRandReaderForTest(r io.Reader) func() {
	prev := randReader
	randReader = r
	return func() { randReader = prev }
}
