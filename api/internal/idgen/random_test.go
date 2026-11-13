package idgen

import (
	"strings"
	"testing"
)

// specRef: registration-subscription-behaviour.md #R12
func TestRandomGenerator_LengthAndAlphabet(t *testing.T) {
	gen := NewRandomGenerator()
	for i := 0; i < 10_000; i++ {
		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate at i=%d: %v", i, err)
		}
		if len(id) != RandomIDLength {
			t.Fatalf("Generate at i=%d: want len=%d, got %d (id=%q)", i, RandomIDLength, len(id), id)
		}
		for _, r := range id {
			if !strings.ContainsRune(RandomIDAlphabet, r) {
				t.Fatalf("Generate at i=%d: %q not in alphabet (id=%q)", i, r, id)
			}
		}
	}
}

// specRef: registration-subscription-behaviour.md #R12
func TestRandomGenerator_Unique(t *testing.T) {
	gen := NewRandomGenerator()
	const n = 200_000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate at i=%d: %v", i, err)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate at i=%d: %q", i, id)
		}
		seen[id] = struct{}{}
	}
}

// specRef: registration-subscription-behaviour.md #R12
// Every character must be drawn with equal probability; a modulo-biased draw
// over 256 byte values would favour the first 256 % 36 = 4 characters by ~14%.
func TestRandomGenerator_UniformCharacters(t *testing.T) {
	gen := NewRandomGenerator()
	counts := make(map[rune]int, len(RandomIDAlphabet))
	const ids = 72_000
	for i := 0; i < ids; i++ {
		id, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		for _, r := range id {
			counts[r]++
		}
	}
	want := ids * RandomIDLength / len(RandomIDAlphabet) // 20,000 per character
	for _, r := range RandomIDAlphabet {
		got := counts[r]
		if got < want*95/100 || got > want*105/100 {
			t.Errorf("character %q drawn %d times, want %d ±5%%", r, got, want)
		}
	}
}
