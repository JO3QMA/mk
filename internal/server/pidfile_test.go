package server

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWritePidFile_EmptyPathSkips(t *testing.T) {
	cleanup, err := WritePidFile("")
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	cleanup() // must be safe to call
}

func TestWritePidFile_WritesAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mk.pid")

	cleanup, err := WritePidFile(path)
	require.NoError(t, err)
	defer cleanup()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	pid, err := parsePID(data)
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), pid)

	cleanup()
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err), "pidfile should be removed by cleanup")
}

// 死んだ PID を指す pidfile (= stale) があるときは上書き起動を許す。
func TestWritePidFile_StaleFileIsOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mk.pid")
	require.NoError(t, os.WriteFile(path, []byte("99999\n"), 0o644))

	// 99999 を「存在しないプロセス」として扱う probe 注入
	cleanup, err := writePidFileWithProcCheck(path, func(pid int) bool { return pid == os.Getpid() })
	require.NoError(t, err)
	defer cleanup()

	data, _ := os.ReadFile(path)
	pid, err := parsePID(data)
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), pid)
}

// 生存中の他プロセスを指す pidfile があると ErrAlreadyRunning で起動拒否。
func TestWritePidFile_LiveOtherProcessRejects(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mk.pid")
	otherPID := os.Getpid() + 1
	require.NoError(t, os.WriteFile(path, []byte(strconv.Itoa(otherPID)+"\n"), 0o644))

	_, err := writePidFileWithProcCheck(path, func(pid int) bool {
		return pid == otherPID // any pid considered alive
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAlreadyRunning),
		"must reject double-start when existing pidfile points at a live process; got %v", err)
}

// 自分自身の PID を含む pidfile (= 再起動 / fast-restart 直後など) は上書き
// で受け入れる。これがないと shutdown 中に書き残った行で次回起動が詰まる。
func TestWritePidFile_OwnPIDIsOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mk.pid")
	require.NoError(t, os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644))

	cleanup, err := writePidFileWithProcCheck(path, func(pid int) bool {
		return true // probe says alive but we should still allow self-reuse
	})
	require.NoError(t, err)
	defer cleanup()
}

func TestWritePidFile_GarbledFileIsOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mk.pid")
	require.NoError(t, os.WriteFile(path, []byte("not-a-number"), 0o644))

	cleanup, err := writePidFileWithProcCheck(path, func(int) bool { return true })
	require.NoError(t, err, "non-numeric pidfile should be treated as stale and overwritten")
	defer cleanup()
}

func TestWritePidFile_UnwritableDirectoryReturnsError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	path := filepath.Join(dir, "mk.pid")
	_, err := WritePidFile(path)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrAlreadyRunning)
}

// defaultProcessAlive は自プロセス (= 生存) で true、不可能な PID 値で
// false になることだけ確認する (細かい挙動は OS 依存なので深追いしない)。
func TestDefaultProcessAlive(t *testing.T) {
	assert.True(t, defaultProcessAlive(os.Getpid()))
	assert.False(t, defaultProcessAlive(0))
	assert.False(t, defaultProcessAlive(-1))
}
