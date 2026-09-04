package treestring2

import (
	"monorepo/twigg/ansi"
	"strings"
)

type block struct {
	// rows[0] is the bottom row
	rows []chunk
}

func newBlock(c chunk) block {
	return block{
		rows: []chunk{c},
	}
}
func newRuneBlock(r rune, c ansi.Color) block {
	return newBlock(newRuneChunk(r, c))
}
func newStringBlock(s string) block {
	return newBlock(newStringChunk(s))
}

func newEmptyBlock() block {
	return block{}
}

func appendBlockRight(left, right block) block {
	nRows := max(len(left.rows), len(right.rows))
	b := block{
		rows: make([]chunk, nRows),
	}

	// Build row by row by appending the chunks
	maxLen := 0
	for i := 0; i < nRows; i++ {
		if i == len(left.rows) {
			b.rows[i] = right.rows[i]
			continue
		}
		if i == len(right.rows) {
			b.rows[i] = left.rows[i]
			continue
		}
		b.rows[i] = appendChunkRight(left.rows[i], right.rows[i])
		maxLen = max(maxLen, len(b.rows[i].r))
	}
	// Adjus the length of each row
	for i := 0; i < nRows; i++ {
		b.rows[i].growToLen(maxLen)
	}
	return b
}

func (bottom *block) appendBlockTop(top block) {
	bottomLen := bottom.width()
	topLen := top.width()
	maxLen := max(bottomLen, topLen)
	bottom.rows = append(bottom.rows, top.rows...)
	for i := 0; i < len(bottom.rows); i++ {
		bottom.rows[i].growToLen(maxLen)
	}
}

func (b block) String() string {
	var s strings.Builder
	for i := len(b.rows) - 1; i >= 0; i-- {
		s.WriteString(b.rows[i].String() + "\n")
	}
	return s.String()
}

func (b block) height() int {
	return len(b.rows)
}

func (b block) width() int {
	if len(b.rows) == 0 {
		return 0
	}
	return len(b.rows[0].r)
}
