package filter

import (
	"strings"
	"testing"
)

// Benchmarks for the subdomain candidate-building strategies used by
// filterBlocklists. "Join" is the previous implementation (strings.Join per
// suffix), "Prepend" is the current one (incremental prepending). Both emit
// the same candidate set: every parent domain excluding the TLD and the full
// FQDN. In production each candidate is followed by a blocklist cache lookup,
// which dominates the cost of this loop; these benchmarks isolate the string
// construction itself.

var subdomainBenchDomains = []struct {
	name string
	fqdn string
}{
	{"4_Labels", "a.b.c.com"},
	{"7_Labels", "a.b.c.d.e.f.com"},
	{"11_Labels", "a.b.c.d.e.f.g.h.i.j.com"},
}

var benchCandidateSink string

func joinCandidates(fqdn string, visit func(string)) {
	parts := strings.Split(fqdn, ".")
	for i := 1; i < len(parts)-1; i++ {
		visit(strings.Join(parts[i:], "."))
	}
}

func prependCandidates(fqdn string, visit func(string)) {
	parts := strings.Split(fqdn, ".")
	var candidate string
	for i := len(parts) - 2; i >= 1; i-- {
		if i == len(parts)-2 {
			candidate = parts[i] + "." + parts[i+1]
		} else {
			candidate = parts[i] + "." + candidate
		}
		visit(candidate)
	}
}

func BenchmarkSubdomainCandidatesJoin(b *testing.B) {
	for _, tc := range subdomainBenchDomains {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				joinCandidates(tc.fqdn, func(c string) { benchCandidateSink = c })
			}
		})
	}
}

func BenchmarkSubdomainCandidatesPrepend(b *testing.B) {
	for _, tc := range subdomainBenchDomains {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				prependCandidates(tc.fqdn, func(c string) { benchCandidateSink = c })
			}
		})
	}
}

// TestSubdomainCandidatesEquivalence guards the refactoring: both strategies
// must produce the identical candidate set, in reverse order of each other.
func TestSubdomainCandidatesEquivalence(t *testing.T) {
	fqdns := []string{"com", "b.com", "a.b.com", "a.b.c.com", "a.b.c.d.e.f.g.h.i.j.com"}
	for _, fqdn := range fqdns {
		var joined, prepended []string
		joinCandidates(fqdn, func(c string) { joined = append(joined, c) })
		prependCandidates(fqdn, func(c string) { prepended = append(prepended, c) })

		for i, j := 0, len(prepended)-1; i < len(prepended)/2; i, j = i+1, j-1 {
			prepended[i], prepended[j] = prepended[j], prepended[i]
		}
		if len(joined) != len(prepended) {
			t.Fatalf("%s: candidate count mismatch: %v vs %v", fqdn, joined, prepended)
		}
		for i := range joined {
			if joined[i] != prepended[i] {
				t.Fatalf("%s: candidate mismatch at %d: %q vs %q", fqdn, i, joined[i], prepended[i])
			}
		}
	}
}
