package idgen

import (
	"sync"
	"testing"
)

// TestSquidsGenerator_IDLength asserts that every generated ID is exactly 10
// characters — the MinLength configured by NewSqidsGenerator.  Encoding a
// single uint64 (monotonic ms timestamp) keeps the output tight; the old
// two-element encoding (timestamp + counter) produced ~13-char IDs.
func TestSquidsGenerator_IDLength(t *testing.T) {
	gen, err := NewSqidsGenerator(10)
	if err != nil {
		t.Fatalf("NewSqidsGenerator: %v", err)
	}

	for i := 0; i < 1000; i++ {
		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate at i=%d: %v", i, err)
		}
		if got := len(id); got != 10 {
			t.Fatalf("Generate at i=%d: want len=10, got len=%d (id=%q)", i, got, id)
		}
	}
}

// TestSquidsGenerator_MonotonicDecoded asserts that the uint64 decoded from
// each successive ID is strictly increasing, which is the invariant that
// prevents collisions when the wall clock stalls.
func TestSquidsGenerator_MonotonicDecoded(t *testing.T) {
	gen, err := NewSqidsGenerator(10)
	if err != nil {
		t.Fatalf("NewSqidsGenerator: %v", err)
	}

	const n = 100_000
	prev := uint64(0)
	for i := 0; i < n; i++ {
		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate at i=%d: %v", i, err)
		}
		nums := gen.Sqids.Decode(id)
		if len(nums) != 1 {
			t.Fatalf("Decode at i=%d: expected 1 number, got %v", i, nums)
		}
		cur := nums[0]
		if cur <= prev {
			t.Fatalf("non-monotonic at i=%d: decoded %d <= prev %d (id=%q)", i, cur, prev, id)
		}
		prev = cur
	}
}

func TestSquidsGenerator_SequentialUniqueness(t *testing.T) {
	gen, err := NewSqidsGenerator(10)
	if err != nil {
		t.Fatalf("NewSqidsGenerator: %v", err)
	}

	const n = 100_000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate at i=%d: %v", i, err)
		}
		if got := len(id); got != 10 {
			t.Fatalf("Generate at i=%d: want len=10, got len=%d (id=%q)", i, got, id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id at i=%d: %s (seen=%d)", i, id, len(seen))
		}
		seen[id] = struct{}{}
	}
}

func TestSquidsGenerator_ConcurrentUniqueness(t *testing.T) {
	gen, err := NewSqidsGenerator(10)
	if err != nil {
		t.Fatalf("NewSqidsGenerator: %v", err)
	}

	const goroutines = 50
	const perGoroutine = 2_000

	results := make([][]string, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			ids := make([]string, 0, perGoroutine)
			for i := 0; i < perGoroutine; i++ {
				id, err := gen.Generate()
				if err != nil {
					t.Errorf("goroutine %d: Generate: %v", g, err)
					return
				}
				ids = append(ids, id)
			}
			results[g] = ids
		}()
	}
	wg.Wait()

	seen := make(map[string]struct{}, goroutines*perGoroutine)
	for g, ids := range results {
		for i, id := range ids {
			if _, dup := seen[id]; dup {
				t.Fatalf("duplicate id from goroutine %d index %d: %s", g, i, id)
			}
			seen[id] = struct{}{}
		}
	}
	if got, want := len(seen), goroutines*perGoroutine; got != want {
		t.Fatalf("expected %d unique ids, got %d", want, got)
	}
}
