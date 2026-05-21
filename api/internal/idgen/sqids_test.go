package idgen

import (
	"sync"
	"testing"
)

func TestSquidsGenerator_SequentialUniqueness(t *testing.T) {
	gen, err := NewSqidsGenerator(10)
	if err != nil {
		t.Fatalf("NewSqidsGenerator: %v", err)
	}

	const n = 10_000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate at i=%d: %v", i, err)
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
