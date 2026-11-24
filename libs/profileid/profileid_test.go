package profileid

import "testing"

// specRef: registration-subscription-behaviour.md #R12
func TestAlphabet_LowercaseLettersAndDigitsOnce(t *testing.T) {
	if len(Alphabet) != 36 {
		t.Fatalf("want 36 characters, got %d", len(Alphabet))
	}
	seen := map[rune]bool{}
	for _, r := range Alphabet {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			t.Fatalf("%q is not a lowercase letter or digit", r)
		}
		if seen[r] {
			t.Fatalf("%q appears twice", r)
		}
		seen[r] = true
	}
}
