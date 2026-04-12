package serverstats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollect(t *testing.T) {
	s := Collect()
	assert.NotEmpty(t, s.Machine.Name)
	assert.Greater(t, s.CPU.Cores, 0)
	assert.Greater(t, s.Mem.Total, uint64(0))
	assert.Greater(t, s.FS.Total, uint64(0))
}

func TestCpuModel(t *testing.T) {
	m := cpuModel()
	assert.NotEqual(t, "", m)
}

func TestTotalMemory(t *testing.T) {
	mem := totalMemory()
	assert.Greater(t, mem, uint64(0))
}

func TestCollect_CPUModelNotEmpty(t *testing.T) {
	s := Collect()
	assert.NotEqual(t, "?", s.CPU.Model, "CPU model should be resolved on Linux")
}

func TestEmpty(t *testing.T) {
	s := Empty()
	assert.Equal(t, "?", s.Machine.Name)
	assert.Equal(t, "?", s.CPU.Model)
	assert.Equal(t, 0, s.CPU.Cores)
	assert.Equal(t, uint64(0), s.Mem.Total)
}
