package diff

// Performs three way merge and returns the contents of the merge
// with the conflict markers when needed.
func Merge(
	base []byte,
	v1 []byte,
	v1Label string,
	v2 []byte,
	v2Label string) (v1v2 []byte, conflict bool) {
	return merge(base, v1, v1Label, v2, v2Label)
}

// Returns (v2 - v1). Returns nil if files are the same.
func ComputeTextDiff(v2 []byte, v2Name string,
	v1 []byte, v1Name string) (diffBytes []byte, nAdded int64, nRemoved int64, nChanged int64) {
	return computeTextDiff(v2, v2Name, v1, v1Name)
}
