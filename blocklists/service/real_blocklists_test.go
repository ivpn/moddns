package service

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivpn/dns/blocklists/internal/extractor"
)

// fixtureDir holds frozen, truncated snapshots of real upstream blocklists.
// Regenerate with: REFRESH_FIXTURES=1 go test ./service -run TestRefreshFixtures -count=1
const fixtureDir = "testdata/real"

// realFixture describes one committed real-world blocklist snapshot. The set
// below covers every distinct extractor and notable format variant.
type realFixture struct {
	file        string // file under fixtureDir
	blocklistID string // selects the extractor (by prefix)
	extractor   string // human label
	url         string // upstream source (used only by the refresher)
	strictMeta  bool   // ExtractMetadata must find a real Last-Modified header
	minDomains  int    // absolute floor for the validated domain count
}

var realFixtures = []realFixture{
	{file: "adguard.txt", blocklistID: "adguard_dns_filter", extractor: "AdGuard", url: "https://adguardteam.github.io/AdGuardSDNSFilter/Filters/filter.txt", strictMeta: true, minDomains: 500},
	{file: "hagezi_domains.txt", blocklistID: "hagezi_multi_light", extractor: "Hagezi/domains", url: "https://raw.githubusercontent.com/hagezi/dns-blocklists/main/domains/light.txt", strictMeta: true, minDomains: 1000},
	{file: "hagezi_wildcard.txt", blocklistID: "hagezi_gambling", extractor: "Hagezi/wildcard", url: "https://raw.githubusercontent.com/hagezi/dns-blocklists/main/wildcard/gambling-onlydomains.txt", strictMeta: true, minDomains: 1000},
	{file: "oisd.txt", blocklistID: "oisd_small", extractor: "OISD", url: "https://small.oisd.nl/domainswild2", strictMeta: true, minDomains: 1000},
	{file: "steven_black.txt", blocklistID: "steven_black_ads_malware", extractor: "StevenBlack", url: "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts", strictMeta: true, minDomains: 1000},
	{file: "blp.txt", blocklistID: "blp_gambling", extractor: "Domains/blp", url: "https://blocklistproject.github.io/Lists/alt-version/gambling-nl.txt", strictMeta: false, minDomains: 500},
	{file: "blp_fakenews.txt", blocklistID: "blp_fakenews", extractor: "Domains/blp(hosts)", url: "https://raw.githubusercontent.com/marktron/fakenews/master/fakenews", strictMeta: false, minDomains: 500},
	{file: "ut1.txt", blocklistID: "ut1_gaming", extractor: "Domains/ut1", url: "https://raw.githubusercontent.com/olbat/ut1-blacklists/master/blacklists/games/domains", strictMeta: false, minDomains: 500},
	{file: "shadowwhisperer.txt", blocklistID: "shadowwhisperer_dating", extractor: "Domains/shadowwhisperer", url: "https://raw.githubusercontent.com/ShadowWhisperer/BlockLists/master/RAW/Dating", strictMeta: false, minDomains: 500},
}

// TestRealBlocklists runs each committed real-world snapshot through the exact
// production parse path (NewExtractor -> ExtractMetadata -> Convert ->
// scanValidatedDomains) and asserts that the result is sane. It is offline and
// deterministic against the frozen fixtures.
//
// specRef: #A1 #A2 #A3 #A4 #A5 — every extractor exercised on real content.
func TestRealBlocklists(t *testing.T) {
	for _, fx := range realFixtures {
		fx := fx
		t.Run(fx.file, func(t *testing.T) {
			path := filepath.Join(fixtureDir, fx.file)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing fixture %s — regenerate with "+
					"REFRESH_FIXTURES=1 go test ./service -run TestRefreshFixtures: %v", path, err)
			}
			raw = bytes.TrimPrefix(raw, []byte("\uFEFF"))

			extr, err := extractor.NewExtractor(fx.blocklistID)
			if err != nil {
				t.Fatalf("NewExtractor(%s): %v", fx.blocklistID, err)
			}

			lastModified, _, _, err := extr.ExtractMetadata(raw)
			if err != nil {
				t.Fatalf("ExtractMetadata: %v", err)
			}
			if fx.strictMeta && lastModified.IsZero() {
				t.Errorf("%s: expected a non-zero Last-Modified from the header block", fx.extractor)
			}

			converted, err := extr.Convert(raw)
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			domains, err := scanValidatedDomains(bytes.NewReader(converted))
			if err != nil {
				t.Fatalf("scanValidatedDomains: %v", err)
			}

			// Floor: catches catastrophic regressions (validation dropping
			// everything / would-be gate abort). blp_fakenews fails here until
			// the Domains hosts-tolerance fix lands.
			if len(domains) < fx.minDomains {
				t.Errorf("%s: got %d valid domains, want >= %d", fx.extractor, len(domains), fx.minDomains)
			}

			// Per-domain invariants: the pipeline output must be clean.
			for _, d := range domains {
				switch {
				case strings.ContainsAny(d, " \t"):
					t.Fatalf("%s: domain %q contains whitespace (hosts-format leak?)", fx.extractor, d)
				case strings.ContainsAny(d, "#!*"):
					t.Fatalf("%s: domain %q contains comment/wildcard chars", fx.extractor, d)
				case d != strings.ToLower(d):
					t.Fatalf("%s: domain %q is not lowercased", fx.extractor, d)
				case d != extractor.NormalizeDomain(d):
					t.Fatalf("%s: domain %q is not fully normalized", fx.extractor, d)
				case !extractor.ValidDomain(d):
					t.Fatalf("%s: domain %q failed ValidDomain", fx.extractor, d)
				}
			}
		})
	}
}
