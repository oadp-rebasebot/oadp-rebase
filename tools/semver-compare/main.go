// semver-compare prints the higher of two Go module versions using
// golang.org/x/mod/semver, which correctly handles pre-release and
// pseudo-version ordering.
//
// Non-semver inputs (e.g., Go directive versions like "1.22") are
// normalized by prepending "v" and padding to three components before
// comparison, so that 1.9 < 1.10 is handled correctly.
//
// Usage: semver-compare <v1> <v2>
// Prints the greater version to stdout.
package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/mod/semver"
)

// normalize attempts to turn a non-semver string into valid semver.
// Go directive versions like "1.22" become "v1.22.0".
func normalize(s string) string {
	v := s
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	// Pad to three components (v1.22 -> v1.22.0)
	parts := strings.SplitN(v[1:], ".", 3)
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	return "v" + strings.Join(parts, ".")
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: semver-compare v1 v2\n")
		os.Exit(1)
	}

	a, b := os.Args[1], os.Args[2]

	// Try direct semver comparison first.
	if semver.IsValid(a) && semver.IsValid(b) {
		if semver.Compare(a, b) >= 0 {
			fmt.Println(a)
		} else {
			fmt.Println(b)
		}
		return
	}

	// Normalize non-semver inputs (e.g., Go directive "1.22").
	na, nb := normalize(a), normalize(b)
	if semver.IsValid(na) && semver.IsValid(nb) {
		if semver.Compare(na, nb) >= 0 {
			fmt.Println(a)
		} else {
			fmt.Println(b)
		}
		return
	}

	// Last resort: lexicographic (should not happen in practice).
	if a >= b {
		fmt.Println(a)
	} else {
		fmt.Println(b)
	}
}
