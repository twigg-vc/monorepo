package treestring2

import (
	"fmt"
	"monorepo/twigg/ansi"
	"testing"
)

type fakeNode struct {
	children    []Node
	msg0        string
	msg1        string
	markerColor ansi.Color
}

func newFakeNode(msg0, msg1 string, markerColor ansi.Color) fakeNode {
	return fakeNode{msg0: msg0, msg1: msg1, markerColor: markerColor}
}
func (n *fakeNode) addChild(child Node) {
	n.children = append(n.children, child)
}

func (n *fakeNode) Children() []Node {
	return n.children
}
func (n fakeNode) FirstLineMessage() string {
	return n.msg0
}
func (n fakeNode) SecondLineMessage() string {
	return n.msg1
}
func (n fakeNode) Marker() rune {
	return '*'
}
func (n fakeNode) MarkerColor() ansi.Color {
	return n.markerColor
}

func TestGetBlock(t *testing.T) {
	root := newFakeNode("root", "abc", ansi.White)
	child1 := newFakeNode("c1", "def", ansi.White)
	child2 := newFakeNode("c2", "ghi", ansi.White)
	child3 := newFakeNode("c3", "jkl", ansi.White)
	child4 := newFakeNode("c4", "mno", ansi.White)
	child5 := newFakeNode("c5", "pqr", ansi.White)
	child6 := newFakeNode("c6", "stu", ansi.White)
	root.addChild(&child1)
	root.addChild(&child2)
	root.addChild(&child3)
	child3.addChild(&child4)
	child3.addChild(&child5)
	child5.addChild(&child6)

	// child7 has height=4, so it wont be printed
	child7 := newFakeNode("c7", "too deep", ansi.Blue)
	child6.addChild(&child7)

	got := Get(&root, 3)
	expected := "  *c6 " + "\n"
	expected += "  |stu" + "\n"
	expected += "  *c5 " + "\n"
	expected += " ├╯pqr" + "\n"
	expected += " |*c4 " + "\n"
	expected += " ├╯mno" + "\n"
	expected += " *c3  " + "\n"
	expected += "├╯jkl " + "\n"
	expected += "|*c2  " + "\n"
	expected += "├╯ghi " + "\n"
	expected += "|*c1  " + "\n"
	expected += "├╯def " + "\n"
	expected += "*root " + "\n"
	expected += "|abc  " + "\n"
	if got != expected {
		t.Errorf("\nExpected:\n%v\nGot:\n%v", expected, got)
	}
}

func TestColor(t *testing.T) {
	root := newFakeNode("root", "abc", ansi.Red)

	got := Get(&root, 0)
	expected := fmt.Sprintf("%v*%vroot\n", ansi.Red, ansi.Reset)
	expected += "|abc \n"
	fmt.Println(got)
	if got != expected {
		t.Errorf("\nExpected:\n%v\nGot:\n%v", expected, got)
	}
}
