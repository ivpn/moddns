// Command announcements-validate parses an announcements feed file with the
// same parser the modDNS API uses at runtime and exits non-zero if it is
// invalid. It is used by the announcements content branch's CI so authors get
// parser-accurate feedback before merge.
package main

import (
	"fmt"
	"os"

	"github.com/ivpn/dns/api/internal/announcements"
)

// warnBytes is the soft threshold (80% of the runtime feed cap) above which the
// file is large enough to warrant pruning before it risks truncation.
const warnBytes = announcements.MaxBodyBytes * 8 / 10

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: announcements-validate <path-to-announcements.md>")
		os.Exit(2)
	}
	path := os.Args[1]

	data, err := os.ReadFile(path) //nolint:gosec // G703: path is a trusted CLI/CI argument (the announcements file to validate), not untrusted input
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot read %s: %v\n", path, err)
		os.Exit(1)
	}

	anns, err := announcements.Parse(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid announcements file %s:\n  %v\n", path, err)
		os.Exit(1)
	}

	fmt.Printf("OK: %s parsed cleanly (%d announcement(s))\n", path, len(anns))

	// Soft size check: the API truncates the feed at MaxBodyBytes, so bytes past
	// that point silently disappear. Warn (but don't fail) as the file
	// approaches the cap, giving authors a heads-up to prune expired entries.
	if size := int64(len(data)); size >= warnBytes {
		fmt.Fprintf(os.Stderr,
			"warning: %s is %d KB, %.0f%% of the %d KB feed limit — bytes past the limit are silently dropped at runtime; prune expired announcements soon\n",
			path, size/1024, float64(size)/float64(announcements.MaxBodyBytes)*100, announcements.MaxBodyBytes/1024)
	}
}
