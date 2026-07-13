// semver-compare prints the higher of two Go module versions using
// golang.org/x/mod/semver, which correctly handles pre-release and
// pseudo-version ordering.
//
// Usage: semver-compare <v1> <v2>
// Prints the greater version to stdout. Exits 1 on invalid input.
package main

import (
	"fmt"
	"os"

	"golang.org/x/mod/semver"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: semver-compare v1 v2\n")
		os.Exit(1)
	}

	a, b := os.Args[1], os.Args[2]

	if !semver.IsValid(a) {
		fmt.Fprintf(os.Stderr, "invalid semver: %s\n", a)
		os.Exit(1)
	}
	if !semver.IsValid(b) {
		fmt.Fprintf(os.Stderr, "invalid semver: %s\n", b)
		os.Exit(1)
	}

	if semver.Compare(a, b) >= 0 {
		fmt.Println(a)
	} else {
		fmt.Println(b)
	}
}
