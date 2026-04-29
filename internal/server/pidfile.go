package server

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// ErrAlreadyRunning is returned by WritePidFile when the existing PID file
// names a process that is still alive — likely another mk instance.
var ErrAlreadyRunning = errors.New("server: another instance appears to be running")

// WritePidFile writes the current PID to path. If path is empty it returns
// a no-op cleanup function and nil error — pidFile is an opt-in feature.
//
// Returns a cleanup function that removes the file (best effort) on
// shutdown. When path is empty the cleanup function is a no-op, so callers
// can defer it unconditionally.
//
// 既存 PID ファイルが live なプロセスを指しているときは ErrAlreadyRunning
// を返して起動を拒否する。stale なファイル (PID は dead) なら上書きする
// (#497)。
func WritePidFile(path string) (func(), error) {
	return writePidFileWithProcCheck(path, defaultProcessAlive)
}

// writePidFileWithProcCheck is WritePidFile with the liveness probe injected
// for tests.
func writePidFileWithProcCheck(path string, alive func(pid int) bool) (func(), error) {
	if path == "" {
		return func() {}, nil
	}
	if data, err := os.ReadFile(path); err == nil {
		if pid, perr := parsePID(data); perr == nil && pid > 0 && pid != os.Getpid() && alive(pid) {
			return nil, fmt.Errorf("%w: %s -> pid %d", ErrAlreadyRunning, path, pid)
		}
	}
	pid := os.Getpid()
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		return nil, fmt.Errorf("write pid file %s: %w", path, err)
	}
	cleanup := func() {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("pidfile: cleanup failed", "path", path, "err", err)
		}
	}
	return cleanup, nil
}

func parsePID(data []byte) (int, error) {
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// defaultProcessAlive uses syscall.Kill(pid, 0) which sends no signal but
// returns ESRCH if the process doesn't exist (Linux/POSIX semantics). This
// distinguishes a stale pid file (PID recycled by another process is
// theoretically possible but extremely rare in practice for a daemon
// scenario, matching the Misskey TS approach).
func defaultProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return !errors.Is(err, syscall.ESRCH)
	}
	return true
}
