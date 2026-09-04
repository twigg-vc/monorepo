// This package is basically a copy of termlink
// (https://github.com/savioxavier/termlink, MIT license, see LICENSE.txt)
// with some stuff removed

package links

import (
	"fmt"
	"os"
	"regexp"
)

// environmentVariables represent the set of standalone environment variables
// ie, those which do not require any special handling or version checking
var environmentVariables = []string{
	"DOMTERM",
	"WT_SESSION",
	"KONSOLE_VERSION",
}

// version_ struct represents a semver version (usually, with some exceptions)
// with major, minor, and patch segments
type version_ struct {
	major int
	minor int
	patch int
}

// parseVersion takes a string "version" number and returns
// a Version struct with the major, minor, and patch
// segments parsed from the string.
// If a version number is not provided
func parseVersion(version string) version_ {
	var major, minor, patch int
	fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch)
	return version_{
		major: major,
		minor: minor,
		patch: patch,
	}
}

// hasEnvironmentVariables returns true if the environment variable "name"
// is present in the environment, false otherwise
func hasEnv(name string) bool {
	_, envExists := os.LookupEnv(name)

	return envExists
}

// checkAllEnvs returns true if any of the environment variables in the "vars"
// string slice are actually present in the environment, false otherwise
func checkAllEnvs(vars []string) bool {
	for _, v := range vars {
		if hasEnv(v) {
			return true
		}
	}

	return false
}

// getEnv returns the value of the environment variable, if it exists
func getEnv(name string) string {
	envValue, _ := os.LookupEnv(name)

	return envValue
}

// matchesEnv returns true if the environment variable "name" matches any
// of the given values in the "values" string slice, false otherwise
func matchesEnv(name string, values []string) bool {
	if hasEnv(name) {
		for _, value := range values {
			if getEnv(name) == value {
				return true
			}
		}
	}
	return false
}

func supportsHyperlinks() bool {
	// Allow hyperlinks to be forced, independent of any environment variables
	// Instead of checking whether it is equal to anything other than "0",
	// a set of allowed values are provided, as something like
	// FORCE_HYPERLINK="do-not-enable-it" wouldn't make sense if it returned true
	if matchesEnv("FORCE_HYPERLINK", []string{"1", "true", "always", "enabled"}) {
		return true
	}

	// VTE-based terminals (Gnome Terminal, Guake, ROXTerm, etc)
	// VTE_VERSION is rendered as four-digit version string
	// eg: 0.52.2 => 5202
	// parseVersion will parse it with a standalone major segment
	// with minor and patch segments set to 0
	// 0.50.0 (parsed as 5000) was supposed to support hyperlinks, but throws a segfault
	// so we check if the "major" version is greater than 5000 (5000 exclusive)
	if hasEnv("VTE_VERSION") {
		v := parseVersion(getEnv("VTE_VERSION"))
		return v.major > 5000
	}

	// Terminals which have a TERM_PROGRAM variable set
	// This is the most versatile environment variable as it also provides another
	// variable called TERM_PROGRAM_VERSION, which helps us to determine
	// the exact version of the program, and allow for stricter variable checks
	if hasEnv("TERM_PROGRAM") {
		v := parseVersion(getEnv("TERM_PROGRAM_VERSION"))

		switch term := getEnv("TERM_PROGRAM"); term {
		case "iTerm.app":
			if v.major == 3 {
				return v.minor >= 1
			}
			return v.major > 3
		case "WezTerm":
			// Even though WezTerm's version is something like 20200620-160318-e00b076c
			// parseVersion will still parse it with a standalone major segment (ie: 20200620)
			// with minor and patch segments set to 0
			return v.major >= 20200620
		case "vscode":
			return v.major > 1 || (v.major == 1 && v.minor >= 72)
		case "ghostty":
			// It is unclear when during the private beta that ghostty started supporting hyperlinks,
			// so we'll start from the public release.
			return v.major >= 1

			// Hyper Terminal used to be included in this list, and it even supports hyperlinks
			// but the hyperlinks are pseudo-hyperlinks and are actually not clickable
		}
	}

	// Terminals which have a TERM variable set
	if matchesEnv("TERM", []string{"xterm-kitty", "alacritty", "alacritty-direct", "xterm-ghostty"}) {
		return true
	}

	// Terminals which have a COLORTERM variable set
	if matchesEnv("COLORTERM", []string{"xfce4-terminal"}) {
		return true
	}

	// Terminals in JetBrains IDEs
	if matchesEnv("TERMINAL_EMULATOR", []string{"JetBrains-JediTerm"}) {
		return true
	}

	// Match standalone environment variables
	// ie, those which do not require any special handling
	// or version checking
	if checkAllEnvs(environmentVariables) {
		return true
	}

	return false
}

// Link returns a clickable link, which can be used in the terminal
//
// The function takes two required parameters: text and url
// and one optional parameter: shouldForce
//
// The text parameter is the text to be displayed.
// The url parameter is the URL to be opened when the link is clicked.
// The shouldForce is an optional parameter indicates whether to force the non-hyperlink supported behavior (i.e., text (url))
//
// The function returns the clickable link.

const openHyperlink = "\x1b]8;;"
const closeHyperlinkUrl = "\x07"
const closeHyperlink = "\x1b]8;;\x07"

func getHyperlink(text string, url string) string {
	return openHyperlink + url + closeHyperlinkUrl +
		text + closeHyperlink
}

var (
	// Matches: ESC ]8;; <any chars> BEL
	openLinkRe = regexp.MustCompile(`\x1b]8;;[^\x07]*\x07`)
	// Matches: ESC ]8;; BEL
	closeLinkRe = regexp.MustCompile(`\x1b]8;;\x07`)
)

func removeHyperlinks(s string) string {
	// Remove opening sequences (keep only the text that follows)
	s = openLinkRe.ReplaceAllString(s, "")
	// Remove closing sequences
	s = closeLinkRe.ReplaceAllString(s, "")
	return s
}