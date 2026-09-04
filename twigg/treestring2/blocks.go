package treestring2

import "monorepo/twigg/ansi"

const kLeftElbow = '╯'
const kVerticalLine = '|'
const kRightJunction = '├'

// Returns the base block of a node.
// It consists of a leftBlock that shows the elbow/line and a marker, followed
// by the message block on the right:
//
// * msg1
// ╯ msg0
func getBaseBlock(node Node, verticalLine bool) block {
	baseRune := kLeftElbow
	if verticalLine {
		baseRune = kVerticalLine
	}
	leftBlock := newRuneBlock(baseRune, ansi.White)
	leftBlock.appendBlockTop(newRuneBlock(node.Marker(), node.MarkerColor()))

	msgBlock := newStringBlock(node.SecondLineMessage())
	msgBlock.appendBlockTop(newStringBlock(node.FirstLineMessage()))

	return appendBlockRight(leftBlock, msgBlock)
}

// Returns the left block that has to be placed left to a child:
// |
// |
// |
func getLeftForChild(childBlock block, isLastChild bool) block {
	leftBlock := newEmptyBlock()
	for i := range childBlock.height() {
		if i == 0 {
			leftBlock.appendBlockTop(newRuneBlock(kRightJunction, ansi.White))
		} else {
			if isLastChild {
				leftBlock.appendBlockTop(newRuneBlock(kEmptyRune, ansi.White))
			} else {
				leftBlock.appendBlockTop(newRuneBlock(kVerticalLine, ansi.White))
			}
		}
	}
	return leftBlock
}

func getBlock(node Node, verticalLineAtBase bool,
	recursionDepth int, maxRecursionDepth int) block {
	if recursionDepth > maxRecursionDepth && maxRecursionDepth != -1 {
		return newEmptyBlock()
	}
	baseBlock := getBaseBlock(node, verticalLineAtBase)
	topBlock := newEmptyBlock()

	if len(node.Children()) == 1 {
		topBlock = getBlock(node.Children()[0], true,
			recursionDepth+1, maxRecursionDepth)
		baseBlock.appendBlockTop(topBlock)
		return baseBlock
	}

	for c, child := range node.Children() {
		childBlock := getBlock(child, false,
			recursionDepth+1, maxRecursionDepth)

		leftBlock := getLeftForChild(childBlock, c == len(node.Children())-1)

		topBlock.appendBlockTop(appendBlockRight(leftBlock, childBlock))
	}
	baseBlock.appendBlockTop(topBlock)
	return baseBlock
}
