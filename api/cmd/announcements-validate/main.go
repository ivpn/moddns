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

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: announcements-validate <path-to-announcements.md>")
		os.Exit(2)
	}
	path := os.Args[1]

	data, err := os.ReadFile(path)
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
}
