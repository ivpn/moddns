package idgen

import (
	"sync"
	"time"

	sqids "github.com/sqids/sqids-go"
)

// SquidsGenerator emits unique IDs by encoding a monotonic millisecond
// timestamp through the sqids library. The last-emitted timestamp is tracked
// in mu-guarded `last`; if the wall clock hasn't advanced since the previous
// call, `last` is incremented by one so every Generate call produces a
// strictly-increasing value and therefore a unique ID — even under concurrent
// callers in a tight loop. Encoding a single uint64 keeps the output at the
// configured MinLength (10 chars by default).
type SquidsGenerator struct {
	Sqids *sqids.Sqids
	mu    sync.Mutex
	last  uint64
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
	now := uint64(time.Now().UnixMilli())
	if now <= gen.last {
		now = gen.last + 1
	}
	gen.last = now
	gen.mu.Unlock()
	return gen.Sqids.Encode([]uint64{now})
}
