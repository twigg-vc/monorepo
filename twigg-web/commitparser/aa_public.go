package commitparser

type Tags struct {
	Bugs []int64
}

// Parses and returns Tags out of a commit description.
func ParseCommitDescription(desc string) Tags {
	return parseCommitDescription(desc)
}
