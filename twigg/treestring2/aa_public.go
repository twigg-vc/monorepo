package treestring2

import "monorepo/twigg/ansi"

type Node interface {
	Children() []Node
	FirstLineMessage() string
	SecondLineMessage() string
	Marker() rune
	MarkerColor() ansi.Color
}

// Returns the string representation of the tree starting at `root`.
// Children which have height > maxHeight will not be considered.
// Use -1 for maxHeight to consider all heights.
func Get(root Node, maxHeight int) string {
	b := getBlock(root, true, 0, maxHeight)
	return b.String()
}
