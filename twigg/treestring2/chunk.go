package treestring2

import (
	"monorepo/twigg/ansi"
	"monorepo/twigg/cli/links"
)

type chunk struct {
	r   []rune // Contains only the visible characters
	raw string
}

func newRuneChunk(r rune, color ansi.Color) chunk {
	if color.S() == ansi.White.S() {
		return chunk{
			r:   []rune{r},
			raw: string(r),
		}
	}
	return chunk{
		r:   []rune{r},
		raw: color.S() + string(r) + ansi.Reset.S(),
	}
}

func newStringChunk(s string) chunk {
	return chunk{
		r:   []rune(links.RemoveHyperlinks(ansi.Remove(s))),
		raw: s,
	}
}

const kEmptyRune = ' '

// If the chunk is smaller than the desired length, append empty runes to it
func (c *chunk) growToLen(desiredLen int) {
	for len(c.r) < desiredLen {
		c.r = append(c.r, kEmptyRune)
		c.raw += " "
	}
}

func (c chunk) String() string {
	return c.raw
}

func appendChunkRight(left, right chunk) chunk {
	return chunk{
		r:   append(left.r, right.r...),
		raw: left.raw + right.raw,
	}
}
