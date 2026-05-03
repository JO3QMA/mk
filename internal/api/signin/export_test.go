package signin

// SwapReadRandomBytes lets external tests stub the random source for the
// passkey-context generator. Returns the previous value so callers can
// restore it.
//
// このシンボルは `_test.go` にしか存在しないので production binary には
// 含まれない (Go の test build 時のみ コンパイルされる)。test seam を
// production に export しないための慣用パターン。
func SwapReadRandomBytes(fn func([]byte) (int, error)) func([]byte) (int, error) {
	old := readRandom
	readRandom = fn
	return old
}
