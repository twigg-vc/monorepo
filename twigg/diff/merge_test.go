package diff

import (
	"strings"
	"testing"
)

func TestMerge(t *testing.T) {

	v1 := []byte("LINE 1\nLine 2")
	base := []byte("Line 1\nLine 2")
	v2 := []byte("Line 1\nLine 2\nLINE 3\nLINE 4")

	merged, conflict := Merge(base, v1, "v1", v2, "v2")
	if conflict {
		t.Error("got conflict")
	}
	if string(merged) != "LINE 1\nLine 2\nLINE 3\nLINE 4" {
		t.Error("wrong merged")
	}

}

func TestConflict(t *testing.T) {

	v1 := []byte("ABCDE\nLine 2")
	v2 := []byte("Line 1\nLine 2")
	base := []byte("FGHIJ\nLine 2")

	merged, conflict := Merge(base, v1, "v1", v2, "v2")
	if !conflict {
		t.Error("expected conflict")
	}
	expected := "<<<<<<<<< v1\nABCDE\n=========\nLine 1\n>>>>>>>>> v2\nLine 2"
	if string(merged) != expected {
		t.Errorf("wrong merge result. Expected %s got %s", expected, string(merged))
	}

}

func TestDiffSameContent(t *testing.T) {

	v1 := []byte("a\nb\nc\n")
	v2 := []byte("a\nb\nc\n")

	diffBytes, nAdd, nRemove, nChange := ComputeTextDiff(v2, "v2", v1, "v1")
	diffText := string(diffBytes)
	if nAdd != 0 {
		t.Fatalf("nAdd=%d", nAdd)
	}
	if nRemove != 0 {
		t.Fatalf("nRemove=%d", nRemove)
	}
	if nChange != 0 {
		t.Fatalf("nChange=%d", nChange)
	}

	expected := "diff v1 v2\n--- v1\n+++ v2\n@@ -1,3 +1,3 @@"
	expected += "\n a"
	expected += "\n b"
	expected += "\n c\n"
	if diffText != expected {
		t.Errorf("wrong diff of same content. Expected %q got %q",
			expected, diffText)
	}

}

func TestDiffLineCounts(t *testing.T) {
	testCases := []struct {
		desc    string
		v1      string
		v2      string
		nAdd    int64
		nRemove int64
		nChange int64
	}{
		{
			desc:    "same text",
			v1:      "a\nb\nc\nd\ne\nf\ng\nh\n",
			v2:      "a\nb\nc\nd\ne\nf\ng\nh\n",
			nAdd:    0,
			nRemove: 0,
			nChange: 0,
		},

		{
			desc:    "change one line (c)",
			v1:      "a\nb\nc\nd\ne\nf\ng\nh\n",
			v2:      "a\nb\nC\nd\ne\nf\ng\nh\n",
			nAdd:    0,
			nRemove: 0,
			nChange: 1,
		},
		{
			desc:    "change some lines (c, e, f, h)",
			v1:      "a\nb\nc\nd\ne\nf\ng\nh\n",
			v2:      "a\nb\nC\nd\nE\nF\ng\nH\n",
			nAdd:    0,
			nRemove: 0,
			nChange: 4,
		},

		{
			desc:    "delete one line (c)",
			v1:      "a\nb\nc\nd\ne\nf\ng\nh\n",
			v2:      "a\nb\nd\ne\nf\ng\nh\n",
			nAdd:    0,
			nRemove: 1,
			nChange: 0,
		},
		{
			desc:    "delete some lines (c, e, f, h)",
			v1:      "a\nb\nc\nd\ne\nf\ng\nh\n",
			v2:      "a\nb\nd\ng\n",
			nAdd:    0,
			nRemove: 4,
			nChange: 0,
		},

		{
			desc:    "add one line (C)",
			v1:      "a\nb\nc\nd\ne\nf\ng\nh\n",
			v2:      "a\nb\nc\nC\nd\ne\nf\ng\nh\n",
			nAdd:    1,
			nRemove: 0,
			nChange: 0,
		},
		{
			desc:    "add one some lines (C, G, X)",
			v1:      "a\nb\nc\nd\ne\nf\ng\nh\n",
			v2:      "a\nb\nc\nC\nd\ne\nf\ng\nG\nh\nX\n",
			nAdd:    3,
			nRemove: 0,
			nChange: 0,
		},

		{
			desc:    "add 2 lines (AAA, BBB), remove 3 lines (i, j, b), change 4 lines (X, Y, Z, W)",
			v1:      "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\no\n",
			v2:      "X\nY\nd\ne\nf\ng\nAAA\nBBB\nh\nk\nl\nZ\nW\n",
			nAdd:    2,
			nRemove: 3,
			nChange: 4,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			_, nAdd, nRemove, nChange := ComputeTextDiff([]byte(tC.v2), "v2", []byte(tC.v1), "v1")
			if nAdd != tC.nAdd {
				t.Fatalf("%s:\ngot nAdd=%d, expected %d", tC.desc, nAdd, tC.nAdd)
			}
			if nRemove != tC.nRemove {
				t.Fatalf("%s:\ngot nRemove=%d, expected %d", tC.desc, nRemove, tC.nRemove)
			}
			if nChange != tC.nChange {
				t.Fatalf("%s:\ngot nChange=%d, expected %d", tC.desc, nChange, tC.nChange)
			}
		})
	}
}

func FuzzDiffLineCounts(f *testing.F) {
	for _, tc := range [][2]string{
		{"a\nb\nc\nd\ne\nf\ng\nh\n", "a\nb\nC\nd\nE\nF\ng\nH\n"},
		{"a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk\nl\nm\no\n", "X\nY\nd\ne\nf\ng\nAAA\nBBB\nh\nk\nl\nZ\nW\n"},
		{"a\nb\nc\n", "a\nX\nY\nZ\nc\n"}, // unequal region: 1 change + 2 adds
		{"}\n}\n}\n}\n", "}\n}\n}\n"},    // repeated lines are never anchors
		{"a\nb", "a\nb\n"},               // missing trailing newline
		{"", "a\nb\n"},
		{"", ""},
	} {
		f.Add(tc[0], tc[1])
	}

	f.Fuzz(func(t *testing.T, v1, v2 string) {
		// v1 is old, v2 is new, matching TestDiffLineCounts.
		out, add, remove, change := ComputeTextDiff([]byte(v2), "v2", []byte(v1), "v1")

		wantAdd, wantRemove, wantChange := countDiffLines(t, string(out))
		if add != wantAdd || remove != wantRemove || change != wantChange {
			t.Fatalf("got add=%d remove=%d change=%d; diff text has add=%d remove=%d change=%d\nv1=%q\nv2=%q\n%s",
				add, remove, change, wantAdd, wantRemove, wantChange, v1, v2, out)
		}
	})
}

// countDiffLines walks a unified diff one line at a time.
// The idea is:
// all "diff" chunks start either with "-" or "+".
// a "modifications" chunk has some "-" lines and then some "+" lines.
// a "deletion" only chunk has "-" lines followed by a " " line.
// a "aditions" only chunk has "+" lines followed by a " " line.
//
// So we count the changes by keeping a cound of the unpaired minus
func countDiffLines(t *testing.T, out string) (add, remove, change int64) {
	// Trim the "diff/---/+++" header; only hunk headers start with "@@".
	headerIndex := strings.Index(out, "\n@@")
	if headerIndex < 0 {
		t.Fatalf("bad headerIndex")
	}
	trimmedOut := out[headerIndex:]

	unpairedMinusCount := int64(0)
	lastLineWasPlus := false
	for _, line := range strings.Split(trimmedOut, "\n") {
		// Once we find a - line, add to the unpairedMinusCount.
		// We'll keep incrementing unpairedMinusCount for as long as we keep seeing - lines
		if strings.HasPrefix(line, "-") {
			if lastLineWasPlus {
				t.Fatalf("%q: - line found right after + line", out)
			}
			lastLineWasPlus = false
			unpairedMinusCount += 1
			continue
		}
		// Once we found a + line, decrement the unpaired minus bc this line
		// "pairs" with a previous unpaired minus line -> those two represent a "change".
		// If unpairedMinus count is zero, this is just an adition
		if strings.HasPrefix(line, "+") {
			lastLineWasPlus = true
			if unpairedMinusCount > 0 {
				unpairedMinusCount--
				change++
			} else {
				add++
			}
			continue
		}
		// When we find a " "/"@@" line, adjust the "remove" count and reset
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "@@") {
			remove += unpairedMinusCount

			lastLineWasPlus = false
			unpairedMinusCount = 0
			continue
		}
	}
	// Add any trailing count
	remove += unpairedMinusCount

	return add, remove, change
}
