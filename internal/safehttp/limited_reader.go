package safehttp

import (
	"errors"
	"io"
	"math"
)

// ErrResponseTooLarge is returned when a read exceeds the configured maximum
// size. Callers should treat this as a protocol-level failure.
var ErrResponseTooLarge = errors.New("safehttp: response body exceeds size limit")

// DefaultAPBodyLimit caps AP object fetches and WebFinger responses.
// 1 MiB 程度。本家 TS も同オーダー。攻撃者制御 host からの memory
// exhaustion を防ぎつつ典型的な AP/JRD payload には十分なサイズ。
const DefaultAPBodyLimit int64 = 1 << 20

// ReadAllLimit reads r up to max bytes. If r contains more than max bytes,
// ErrResponseTooLarge is returned. max <= 0 disables the cap (matches
// io.ReadAll). max >= math.MaxInt64 is treated as unlimited to avoid
// overflow when computing max+1.
//
// 実装上は io.LimitReader(r, max+1) から読み、返り値長が max を超えたら
// ErrResponseTooLarge。合計バイト数ちょうど max の場合は成功。
func ReadAllLimit(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 || max == math.MaxInt64 {
		return io.ReadAll(r)
	}
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, ErrResponseTooLarge
	}
	return data, nil
}
