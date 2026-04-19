package safehttp

import (
	"errors"
	"io"
	"math"
)

// ErrResponseTooLarge is returned when a read exceeds the configured maximum
// size. Callers should treat this as a protocol-level failure.
var ErrResponseTooLarge = errors.New("safehttp: response body exceeds size limit")

// Default caps used by outbound fetchers. Chosen to be comfortably larger
// than typical ActivityPub / WebFinger payloads yet small enough to bound
// attacker-controlled memory usage.
const (
	// DefaultAPBodyLimit caps AP object fetches and WebFinger responses.
	// 1 MiB 程度。本家 TS も同オーダー。
	DefaultAPBodyLimit int64 = 1 << 20
	// DefaultURLPreviewBodyLimit caps URL preview HTML fetches.
	// OGP の meta タグ探索が目的なので 512 KiB に抑える。
	DefaultURLPreviewBodyLimit int64 = 512 << 10
)

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
