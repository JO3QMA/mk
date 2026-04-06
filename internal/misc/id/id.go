package id

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"
)

// time2000 is the epoch used by AID/AIDX (2000-01-01T00:00:00Z in milliseconds).
const time2000 int64 = 946684800000

// Generator defines the interface for ID generation.
type Generator interface {
	// Generate creates a new ID for the given timestamp.
	Generate(t time.Time) string
	// ParseTime extracts the timestamp from an ID.
	ParseTime(id string) (time.Time, error)
}

// NewGenerator creates a Generator based on the method name.
// Supported: "aid", "aidx", "meid", "meidg", "ulid", "objectid".
func NewGenerator(method string) (Generator, error) {
	switch strings.ToLower(method) {
	case "aid":
		return newAID(), nil
	case "aidx":
		return newAIDX(), nil
	case "meid":
		return newMEID(), nil
	case "objectid":
		return newObjectID(), nil
	case "ulid":
		return newULID(), nil
	default:
		return nil, fmt.Errorf("unknown id method: %s", method)
	}
}

// --- AID ---

type aidGen struct {
	mu      sync.Mutex
	counter uint16
}

func newAID() *aidGen {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	return &aidGen{counter: uint16(b[0]) | uint16(b[1])<<8}
}

func (g *aidGen) Generate(t time.Time) string {
	ms := t.UnixMilli() - time2000
	timePart := padLeft(strconv.FormatInt(ms, 36), 8, '0')

	g.mu.Lock()
	g.counter++
	c := g.counter
	g.mu.Unlock()

	counterPart := padLeft(strconv.FormatUint(uint64(c), 36), 2, '0')
	counterPart = counterPart[len(counterPart)-2:]

	return timePart + counterPart
}

func (g *aidGen) ParseTime(id string) (time.Time, error) {
	if len(id) < 8 {
		return time.Time{}, fmt.Errorf("invalid aid: %s", id)
	}
	ms, err := strconv.ParseInt(id[:8], 36, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(ms + time2000), nil
}

// --- AIDX ---

type aidxGen struct {
	mu      sync.Mutex
	counter uint32
	nodeID  string
}

func newAIDX() *aidxGen {
	return &aidxGen{
		nodeID: randomBase36(4),
	}
}

func (g *aidxGen) Generate(t time.Time) string {
	ms := t.UnixMilli() - time2000
	timePart := padLeft(strconv.FormatInt(ms, 36), 8, '0')
	timePart = timePart[len(timePart)-8:]

	g.mu.Lock()
	g.counter++
	c := g.counter
	g.mu.Unlock()

	counterPart := padLeft(strconv.FormatUint(uint64(c), 36), 4, '0')
	counterPart = counterPart[len(counterPart)-4:]

	return timePart + g.nodeID + counterPart
}

func (g *aidxGen) ParseTime(id string) (time.Time, error) {
	if len(id) < 8 {
		return time.Time{}, fmt.Errorf("invalid aidx: %s", id)
	}
	ms, err := strconv.ParseInt(id[:8], 36, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(ms + time2000), nil
}

// --- MEID ---

type meidGen struct{}

func newMEID() *meidGen { return &meidGen{} }

const meidOffset int64 = 0x800000000000

func (g *meidGen) Generate(t time.Time) string {
	ms := t.UnixMilli()
	timePart := padLeft(strconv.FormatInt(ms+meidOffset, 16), 12, '0')
	return timePart + randomHex(12)
}

func (g *meidGen) ParseTime(id string) (time.Time, error) {
	if len(id) < 12 {
		return time.Time{}, fmt.Errorf("invalid meid: %s", id)
	}
	ms, err := strconv.ParseInt(id[:12], 16, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(ms - meidOffset), nil
}

// --- ObjectID ---

type objectIDGen struct{}

func newObjectID() *objectIDGen { return &objectIDGen{} }

func (g *objectIDGen) Generate(t time.Time) string {
	sec := t.Unix()
	timePart := padLeft(strconv.FormatInt(sec, 16), 8, '0')
	return timePart + randomHex(16)
}

func (g *objectIDGen) ParseTime(id string) (time.Time, error) {
	if len(id) < 8 {
		return time.Time{}, fmt.Errorf("invalid objectid: %s", id)
	}
	sec, err := strconv.ParseInt(id[:8], 16, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, 0), nil
}

// --- ULID ---

type ulidGen struct {
	mu      sync.Mutex
	entropy *monotonic
}

func newULID() *ulidGen {
	return &ulidGen{entropy: newMonotonic()}
}

// Crockford's Base32
const crockfordBase32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func (g *ulidGen) Generate(t time.Time) string {
	ms := uint64(t.UnixMilli())

	// Encode timestamp (10 chars)
	var ts [10]byte
	for i := 9; i >= 0; i-- {
		ts[i] = crockfordBase32[ms&0x1F]
		ms >>= 5
	}

	// Encode randomness (16 chars)
	g.mu.Lock()
	rnd := g.entropy.next()
	g.mu.Unlock()

	var rs [16]byte
	for i := 15; i >= 0; i-- {
		rs[i] = crockfordBase32[rnd[i%len(rnd)]&0x1F]
	}

	return string(ts[:]) + string(rs[:])
}

func (g *ulidGen) ParseTime(id string) (time.Time, error) {
	if len(id) < 10 {
		return time.Time{}, fmt.Errorf("invalid ulid: %s", id)
	}
	var ms uint64
	for i := 0; i < 10; i++ {
		idx := strings.IndexByte(crockfordBase32, id[i])
		if idx < 0 {
			return time.Time{}, fmt.Errorf("invalid ulid character: %c", id[i])
		}
		ms = ms*32 + uint64(idx)
	}
	return time.UnixMilli(int64(ms)), nil
}

// monotonic provides monotonically increasing random bytes for ULID.
type monotonic struct {
	last [10]byte
}

func newMonotonic() *monotonic {
	m := &monotonic{}
	_, _ = rand.Read(m.last[:])
	return m
}

func (m *monotonic) next() [10]byte {
	// Increment the random part
	for i := len(m.last) - 1; i >= 0; i-- {
		m.last[i]++
		if m.last[i] != 0 {
			break
		}
	}
	return m.last
}

// --- Helpers ---

func padLeft(s string, length int, pad byte) string {
	for len(s) < length {
		s = string(pad) + s
	}
	return s
}

func randomBase36(n int) string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, n)
	for i := range result {
		idx, _ := rand.Int(rand.Reader, big.NewInt(36))
		result[i] = chars[idx.Int64()]
	}
	return string(result)
}

func randomHex(n int) string {
	const chars = "0123456789abcdef"
	result := make([]byte, n)
	for i := range result {
		idx, _ := rand.Int(rand.Reader, big.NewInt(16))
		result[i] = chars[idx.Int64()]
	}
	return string(result)
}
