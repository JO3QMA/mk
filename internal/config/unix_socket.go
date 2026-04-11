package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
)

// ListenUnixSocket binds to the given UNIX domain socket path, applies the
// optional chmod (given as an octal string like "770"), and returns the
// resulting net.Listener. The helper is shared between production startup
// (internal/server) and test code so that all call sites agree on stale-file
// handling and permission parsing.
//
// chmod は空文字列なら無視する。空でないのにパースに失敗した場合は
// listener をクローズした上でエラーを返す。本家 Misskey の chmodSocket と
// 同じく 8 進数文字列 (例: "770") を想定する。
//
// 既にパスに何か存在していた場合、それが本当にソケットファイル (mode で
// ModeSocket が立っている) ならば安全に削除してから bind し直す。通常の
// ファイルやディレクトリの場合は削除せずエラーを返す ― 運用者が誤って
// YAML で重要なパスを書いてしまったときに上書き消去しないための防御。
func ListenUnixSocket(path, chmod string) (net.Listener, error) {
	if path == "" {
		return nil, errors.New("config: unix socket path is empty")
	}

	if fi, err := os.Lstat(path); err == nil {
		if fi.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("config: refusing to remove non-socket path at %s", path)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("config: remove stale socket %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config: stat socket %s: %w", path, err)
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("config: listen unix %s: %w", path, err)
	}

	if chmod != "" {
		mode, parseErr := strconv.ParseUint(chmod, 8, 32)
		if parseErr != nil {
			_ = ln.Close()
			_ = os.Remove(path)
			return nil, fmt.Errorf("config: chmodSocket %q is not a valid octal mode: %w", chmod, parseErr)
		}
		if chmodErr := os.Chmod(path, os.FileMode(mode)); chmodErr != nil {
			_ = ln.Close()
			_ = os.Remove(path)
			return nil, fmt.Errorf("config: chmod %s: %w", path, chmodErr)
		}
	}
	return ln, nil
}

// RemoveUnixSocket removes the socket file at path, ignoring non-existence.
// It is used during Shutdown so that a subsequent start can re-bind cleanly.
func RemoveUnixSocket(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
