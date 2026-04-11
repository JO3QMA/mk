package config

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listenAndLeaveStale binds a UDS at path and closes it without unlinking the
// socket file, leaving a "stale" socket on disk for tests that need to verify
// the recovery path. Go の *net.UnixListener.Close() は通常ソケットファイルを
// unlink してしまうので、SetUnlinkOnClose(false) で抑止している。
func listenAndLeaveStale(t *testing.T, path string) {
	t.Helper()
	addr, err := net.ResolveUnixAddr("unix", path)
	require.NoError(t, err)
	ln, err := net.ListenUnix("unix", addr)
	require.NoError(t, err)
	ln.SetUnlinkOnClose(false)
	require.NoError(t, ln.Close())
	fi, statErr := os.Stat(path)
	require.NoError(t, statErr, "expected stale socket file to remain on disk")
	require.True(t, fi.Mode()&os.ModeSocket != 0, "expected %s to be a socket", path)
}

func TestListenUnixSocket_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mk.sock")

	ln, err := ListenUnixSocket(path, "")
	require.NoError(t, err)
	defer ln.Close()

	fi, statErr := os.Stat(path)
	require.NoError(t, statErr)
	assert.True(t, fi.Mode()&os.ModeSocket != 0, "expected %s to be a socket", path)
}

func TestListenUnixSocket_WithChmod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mk.sock")

	ln, err := ListenUnixSocket(path, "660")
	require.NoError(t, err)
	defer ln.Close()

	fi, statErr := os.Stat(path)
	require.NoError(t, statErr)
	// chmod の結果 0660 が反映されていること。実 perm ビットだけを比較する。
	assert.Equal(t, os.FileMode(0660), fi.Mode().Perm())
}

func TestListenUnixSocket_EmptyPath(t *testing.T) {
	_, err := ListenUnixSocket("", "")
	assert.Error(t, err)
}

func TestListenUnixSocket_StaleSocketRemoved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mk.sock")

	// 前回 bind したまま unlink されずに残ったソケットファイルを模擬する
	listenAndLeaveStale(t, path)

	// stale を踏んでも成功しなければならない
	ln2, err := ListenUnixSocket(path, "")
	require.NoError(t, err)
	defer ln2.Close()
}

func TestListenUnixSocket_RefusesNonSocketPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "regular-file")
	// 普通のファイルを置く
	require.NoError(t, os.WriteFile(path, []byte("important data"), 0600))

	_, err := ListenUnixSocket(path, "")
	assert.Error(t, err)

	// エラー時でも元ファイルが消えていないこと (重要: 本番で誤設定された
	// YAML が大事なファイルを吹き飛ばさないための保険)。
	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "important data", string(data))
}

func TestListenUnixSocket_InvalidChmod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mk.sock")

	_, err := ListenUnixSocket(path, "not-octal")
	assert.Error(t, err)

	// 失敗時は listener を閉じ、ソケットファイルも削除されていること
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr))
}

func TestListenUnixSocket_ListenFails(t *testing.T) {
	// 存在しないディレクトリ配下を指定すると net.Listen が失敗する
	_, err := ListenUnixSocket("/nonexistent-dir-mk-go-test/mk.sock", "")
	assert.Error(t, err)
}

func TestListenUnixSocket_StaleRemoveFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("skipping remove-error test when running as root")
	}

	dir := t.TempDir()
	subdir := filepath.Join(dir, "locked")
	require.NoError(t, os.Mkdir(subdir, 0700))
	t.Cleanup(func() { _ = os.Chmod(subdir, 0700) })

	path := filepath.Join(subdir, "mk.sock")
	// stale なソケットを残した上で親ディレクトリの書き込み権を落とし、
	// unlink が失敗する状態を作る。Lstat は r+x が残っているので通る。
	listenAndLeaveStale(t, path)
	require.NoError(t, os.Chmod(subdir, 0500))

	_, err := ListenUnixSocket(path, "")
	assert.Error(t, err)
}

func TestListenUnixSocket_StatError(t *testing.T) {
	// Lstat が EACCES を返すケースをシミュレートする。root では 000 でも
	// stat が通ってしまうので root 実行時は skip。
	if os.Geteuid() == 0 {
		t.Skip("skipping stat-error test when running as root")
	}

	dir := t.TempDir()
	subdir := filepath.Join(dir, "no-access")
	require.NoError(t, os.Mkdir(subdir, 0700))
	t.Cleanup(func() { _ = os.Chmod(subdir, 0700) })

	path := filepath.Join(subdir, "mk.sock")
	// stale なソケットファイルを置いた上で親ディレクトリを 000 にする。
	// 親が x 権限を失うと子 inode への stat も失敗するので Lstat 段で
	// ErrNotExist 以外のエラーが返る経路を通せる。
	listenAndLeaveStale(t, path)

	require.NoError(t, os.Chmod(subdir, 0))

	_, err := ListenUnixSocket(path, "")
	assert.Error(t, err)
}

func TestRemoveUnixSocket_Empty(t *testing.T) {
	err := RemoveUnixSocket("")
	assert.NoError(t, err)
}

func TestRemoveUnixSocket_Exists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mk.sock")

	listenAndLeaveStale(t, path)

	require.NoError(t, RemoveUnixSocket(path))

	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr))
}

func TestRemoveUnixSocket_NotExist(t *testing.T) {
	// 存在しないパスに対しても no-op で nil を返すこと
	err := RemoveUnixSocket("/nonexistent/mk-go-test.sock")
	assert.NoError(t, err)
}

func TestRemoveUnixSocket_StatOtherError(t *testing.T) {
	// 親ディレクトリに書き込み権限が無いときは EACCES / EPERM が返る。
	// root 実行時はテストできないので skip。
	if os.Geteuid() == 0 {
		t.Skip("skipping permission-error test when running as root")
	}

	dir := t.TempDir()
	subdir := filepath.Join(dir, "locked")
	require.NoError(t, os.Mkdir(subdir, 0700))
	t.Cleanup(func() { _ = os.Chmod(subdir, 0700) })

	path := filepath.Join(subdir, "mk.sock")
	// ソケットファイルを残した状態で親ディレクトリを read+exec only にすると、
	// unlink が EACCES で失敗する経路を通せる。
	listenAndLeaveStale(t, path)

	require.NoError(t, os.Chmod(subdir, 0500))

	err := RemoveUnixSocket(path)
	assert.Error(t, err)
}
