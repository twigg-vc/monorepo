package links

// Returns true if current terminal supports hyperlinks
func Supports() bool {
	return supportsHyperlinks()
}

// Returns a string that is shown as a clickable url in the terminal
// (if supported)
func GetHyperlink(displayText, url string) string {
	return getHyperlink(displayText, url)
}

// RemoveHyperlinks removes OSC 8 hyperlink escape sequences from the input
// string, leaving only the visible text.
//
// OSC 8 hyperlinks have the form:
//
//	ESC ]8;;<URL> BEL
//	<VISIBLE TEXT>
//	ESC ]8;; BEL
func RemoveHyperlinks(s string) string {
	return removeHyperlinks(s)
}