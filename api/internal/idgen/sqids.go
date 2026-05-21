package idgen

import (
	"fmt"
	"sync"
	"time"

	sqids "github.com/sqids/sqids-go"
)

// SquidsGenerator emits unique IDs by encoding (timestampMs, counter) through
// the sqids library. The counter is a per-instance monotonic sequence guarded
// by mu; combined with the timestamp it makes Generate collision-free for any
// number of calls within a single process — sqids.Encode is purely
// deterministic on its input, so a same-ms call on its own would otherwise
// repeat the previous ID.
type SquidsGenerator struct {
	Sqids   *sqids.Sqids
	mu      sync.Mutex
	counter uint64
}

func NewSqidsGenerator(minLength int) (*SquidsGenerator, error) {
	if minLength <= 0 {
		minLength = 10
	}
	// Guard against overflow when casting to uint8
	if minLength > 255 {
		minLength = 255
	}
	// Convert safely after clamping (minLength now within 1..255)
	// minLength is clamped to <=255 above, safe conversion
	minLenUint8 := uint8(minLength) // #nosec G115
	s, err := sqids.New(sqids.Options{
		MinLength: minLenUint8,
		Alphabet:  "abcdefghijklmnopqrstuxyz1234567890",
	})
	if err != nil {
		return nil, err
	}

	return &SquidsGenerator{
		Sqids: s,
	}, nil
}

func (gen *SquidsGenerator) Generate() (string, error) {
	gen.mu.Lock()
	gen.counter++
	c := gen.counter
	gen.mu.Unlock()

	now := time.Now().UnixMilli()
	if now < 0 {
		return "", fmt.Errorf("negative timestamp: %d", now)
	}
	return gen.Sqids.Encode([]uint64{uint64(now), c})
}
