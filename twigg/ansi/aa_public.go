package ansi

import "regexp"

type Color string

const Reset Color = "\033[0m"
const Red Color = "\033[31m"
const Green Color = "\033[32m"
const BoldGreen Color = "\033[1;32m"
const LightGreen Color = "\033[38;5;151m"
const Yellow Color = "\033[33m"
const SoftYellow Color = "\033[38;5;187m"
const BoldYellow Color = "\033[1;33m"
const Blue Color = "\033[34m"
const Magenta Color = "\033[35m"
const Cyan Color = "\033[36m"
const Gray Color = "\033[90m"
const BoldGray Color = "\033[1;90m"
const White Color = "\033[97m"

func (c Color) String() string {
	return string(c)
}
func (c Color) S() string {
	return c.String()
}

// Returns a string that has all the codes removed.
func Remove(input string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(input, "")
}
