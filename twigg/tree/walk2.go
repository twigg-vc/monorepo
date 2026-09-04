package tree

import (
	"bytes"
	"errors"
	"io"
	"monorepo/twigg/diff"
	"path/filepath"
)

func walkParallel(a, b Root) (ParallelIterator, error) {
	aTr, err := Walk(a)
	if err != nil {
		return nil, err
	}
	bTr, err := Walk(b)
	if err != nil {
		return nil, err
	}
	return newParallelIter(aTr, bTr), nil
}

func newParallelIter(a, b Iterator) ParallelIterator {
	return &iter2{
		aIter:            a,
		bIter:            b,
		willSkipChildren: false,
	}
}

type iter2 struct {
	aIter            Iterator
	bIter            Iterator
	willSkipChildren bool
	hasValues        bool
	hasA             bool
	aPath            string
	aDepth           uint32
	aSt              VisitStatus
	aTree            Tree
	hasB             bool
	bPath            string
	bDepth           uint32
	bSt              VisitStatus
	bTree            Tree
}

func (d iter2) CanGet() bool {
	return d.aIter.CanGet() || d.bIter.CanGet()
}
func (d *iter2) GetDiff() Diff {
	if !d.CanGet() {
		panic("GetDiff called with CanGet() = false")
	}
	d.setValues()
	_, aComesFirst, bComesFirst := d.comparePaths()
	if aComesFirst {
		if d.aTree.IsRemovedChild() || !d.aTree.DataIsComplete() {
			return Diff{
				Type: DiffTypeUndefined,
			}
		}
		return Diff{
			Type: DiffTypeCreated,
			Data: d.aTree.Data(),
		}
	}
	if bComesFirst {
		if d.bTree.IsRemovedChild() || !d.bTree.DataIsComplete() {
			return Diff{
				Type: DiffTypeUndefined,
			}
		}
		return Diff{
			Type: DiffTypeDeleted,
			Data: d.bTree.Data(),
		}
	}
	if !d.hasA || !d.hasB {
		// This should never happen bc if any of the trees doesn't exist,
		// one of them is always considered th be first, even if none exist
		panic("tried to get diff type without one of the trees")
	}
	diffType, treeData := getTreeDiffTypeAndData(d.aTree, d.bTree)
	return Diff{
		Type: diffType,
		Data: treeData,
	}
}
func (d *iter2) Get() (string, uint32, VisitStatus, Tree) {
	d.setValues()
	tie, aComesFirst, bComesFirst := d.comparePaths()
	if tie {
		// For tied paths, report SecondVisit when either side already
		// visited this path. Only one side can be on a scond visit in cases
		// of file<->dir changes.
		visitStatus := d.aSt
		if d.bSt == SecondVisit {
			visitStatus = SecondVisit
		}
		return d.aPath, d.aDepth, visitStatus, d.aTree
	}
	if aComesFirst {
		return d.aPath, d.aDepth, d.aSt, d.aTree
	}
	if bComesFirst {
		return d.bPath, d.bDepth, d.bSt, d.bTree
	}
	panic("neither tie, nor aFirst, nor bFirst")
}
func (d *iter2) CanGetA() bool {
	if !d.CanGet() {
		return false
	}
	d.setValues()
	tie, aComesFirst, _ := d.comparePaths()
	return tie || aComesFirst
}
func (d *iter2) GetA() (string, uint32, VisitStatus, Tree) {
	if !d.CanGet() {
		panic("GetA called with CanGet() = false")
	}
	return d.aIter.Get()
}
func (d *iter2) CanGetB() bool {
	if !d.CanGet() {
		return false
	}
	d.setValues()
	tie, _, bComesFirst := d.comparePaths()
	return tie || bComesFirst
}
func (d *iter2) GetB() (string, uint32, VisitStatus, Tree) {
	if !d.CanGet() {
		panic("GetB called with CanGet() = false")
	}
	return d.bIter.Get()
}

func (d *iter2) SkipChildrenOnNext() {
	d.willSkipChildren = true
}

func (d *iter2) setValues() {
	if d.hasValues {
		return
	}
	d.hasA = d.aIter.CanGet()
	if d.aIter.CanGet() {
		d.aPath, d.aDepth, d.aSt, d.aTree = d.aIter.Get()
	} else {
		d.aPath = ""
		d.aTree = nil
	}
	d.hasB = d.bIter.CanGet()
	if d.bIter.CanGet() {
		d.bPath, d.bDepth, d.bSt, d.bTree = d.bIter.Get()
	} else {
		d.bPath = ""
		d.bTree = nil
	}
	d.hasValues = true
}

func (d *iter2) Next() (err error) {
	defer func() {
		d.hasValues = false
		if err == nil {
			d.willSkipChildren = false
		}
	}()
	d.setValues()
	_, aComesFirst, bComesFirst := d.comparePaths()
	if aComesFirst {
		if d.willSkipChildren {
			d.aIter.SkipChildrenOnNext()
		}
		return d.aIter.Next()
	}
	if bComesFirst {
		if d.willSkipChildren {
			d.bIter.SkipChildrenOnNext()
		}
		return d.bIter.Next()
	}

	nChildrenA := 0
	nChildrenB := 0
	if !d.aTree.IsRemovedChild() {
		nChildrenA = len(d.aTree.Data().ChildrenBaseNames)
	}
	if !d.bTree.IsRemovedChild() {
		nChildrenB = len(d.bTree.Data().ChildrenBaseNames)
	}
	willPopA := d.willSkipChildren || nChildrenA == 0 || d.aSt == SecondVisit
	willPopB := d.willSkipChildren || nChildrenB == 0 || d.bSt == SecondVisit

	// To keep both iterators at the same depth, we only advance both if both
	// will Pop/won't Pop. Else, we advance only the one that won't pop.
	if willPopA == willPopB {
		if d.willSkipChildren {
			d.aIter.SkipChildrenOnNext()
		}
		err = d.aIter.Next()
		if err != nil {
			return
		}
		if d.willSkipChildren {
			d.bIter.SkipChildrenOnNext()
		}
		return d.bIter.Next()
	}

	if !willPopB {
		if d.willSkipChildren {
			d.bIter.SkipChildrenOnNext()
		}
		return d.bIter.Next()
	}
	if d.willSkipChildren {
		d.aIter.SkipChildrenOnNext()
	}
	return d.aIter.Next()
}

// Main logic for determinining the type of the diff.
// The returned Data will always be of aTree or a zero value
func getTreeDiffTypeAndData(aTree Tree, bTree Tree) (DiffType, Data) {
	if bTree.IsRemovedChild() || aTree.IsRemovedChild() {
		return DiffTypeUndefined, Data{}
	}

	// Dir-ness is known even for incomplete data, so dir<->file swaps
	// can be reported before the completeness check below
	if aTree.Data().IsDir != bTree.Data().IsDir {
		return DiffTypeAnyModified, aTree.Data()
	}
	// If any is incomplete, the diff type is unkown
	if !aTree.DataIsComplete() || !bTree.DataIsComplete() {
		return DiffTypeUndefined, aTree.Data()
	}

	if IsEqual(aTree, bTree) {
		return DiffTypeNoChange, aTree.Data()
	}
	return DiffTypeAnyModified, aTree.Data()
}

func isEqual(tr1, tr2 Tree) bool {
	if !tr1.DataIsComplete() || !tr2.DataIsComplete() {
		panic("called IsEqual for non complete tree data")
	}
	if tr1.Data().IsDir != tr2.Data().IsDir {
		return false
	}
	if tr1.Data().Size != tr2.Data().Size {
		return false
	}
	// ContentHash should change if IsExectuableFile/SymlinkTarget changes,
	// but we check if here anyway bc it's cheap (cheaper that hash comparison)
	if tr1.Data().IsExectuableFile != tr2.Data().IsExectuableFile {
		return false
	}
	if tr1.Data().SymlinkTarget != tr2.Data().SymlinkTarget {
		return false
	}
	return tr1.Data().ContentHash == tr2.Data().ContentHash
}

// Returns exactly one "true“
func (d iter2) comparePaths() (tie bool, aComesFirst bool, bComesFirst bool) {
	if !d.hasValues {
		panic("called comparePaths before setting values")
	}
	defer func() {
		trueCount := 0
		if tie {
			trueCount += 1
		}
		if aComesFirst {
			trueCount += 1
		}
		if bComesFirst {
			trueCount += 1
		}
		if trueCount != 1 {
			panic("expected always one true")
		}
	}()

	if d.hasA && !d.hasB {
		aComesFirst = true
		bComesFirst = false
		return
	}
	if !d.hasA && d.hasB {
		aComesFirst = false
		bComesFirst = true
		return
	}

	// We always start at root node, with depth=0.
	// We iterate walking down.
	// We only get to a depth larger on A than B if at some point the node A
	// had some children but the B node didn't exist or didn't have children.
	// In that case, B is behind and we must process all A first before
	// continuing with B.
	// Same thing applies for bDepth>aDepth (but for B).
	if d.aDepth > d.bDepth {
		aComesFirst = true
		bComesFirst = false
		return
	}
	if d.aDepth < d.bDepth {
		aComesFirst = false
		bComesFirst = true
		return
	}

	// When the depth is the same, the dirPath (i.e. the path minus the last
	// part) must be the same.
	if filepath.Dir(d.aPath) != filepath.Dir(d.bPath) {
		panic("nodes are at the same depth but different dir paths")
	}
	// Since both are at the same depth, we first process the "smallest" (
	// lexicographically sorted) paths.
	if d.aPath < d.bPath {
		aComesFirst = true
		bComesFirst = false
		return
	}
	if d.aPath > d.bPath {
		aComesFirst = false
		bComesFirst = true
		return
	}

	tie = true
	aComesFirst = false
	bComesFirst = false
	return
}
func (d iter2) GetTextDiff() ([]byte, int64, int64, int64, bool, error) {
	if !d.CanGet() {
		panic("GetTextDiff called with CanGet() = false")
	}
	if d.GetDiff().Type == DiffTypeUndefined {
		return nil, 0, 0, 0, false, nil
	}
	hasFileA := false
	var fileA io.WriterTo
	var fileAData Data
	if d.CanGetA() {
		_, _, _, aTr := d.GetA()
		fileAData = aTr.Data()
		hasFileA = !fileAData.IsDir
		if hasFileA {
			var err error
			fileA, err = aTr.GetFile()
			if err != nil {
				return nil, 0, 0, 0, false, err
			}
		}
	}

	hasFileB := false
	var fileB io.WriterTo
	var fileBData Data
	if d.CanGetB() {
		_, _, _, bTr := d.GetB()
		fileBData = bTr.Data()
		hasFileB = !fileBData.IsDir
		if hasFileB {
			var err error
			fileB, err = bTr.GetFile()
			if err != nil {
				return nil, 0, 0, 0, false, err
			}
		}
	}
	if !hasFileA && !hasFileB {
		return nil, 0, 0, 0, false, nil
	}
	filePath, _, _, _ := d.Get()
	diffBytes, nAdd, nRemove, nChange, err := getDiffBytes(filePath, hasFileA, fileAData, fileA, hasFileB, fileBData, fileB)
	return diffBytes, nAdd, nRemove, nChange, err == nil, err
}

func getDiffBytes(filePath string,
	hasFileA bool, fileAData Data, fileA io.WriterTo,
	hasFileB bool, fileBData Data, fileB io.WriterTo) (diffBytes []byte, nAdd, nRemove, nChange int64, err error) {
	shouldDiff := true
	if hasFileA {
		shouldDiff = shouldDiff &&
			fileAData.IsText && fileAData.Size < MaxFileSizeToDiff
	}
	if hasFileB {
		shouldDiff = shouldDiff &&
			fileBData.IsText && fileBData.Size < MaxFileSizeToDiff
	}
	if !shouldDiff {
		diffBytes = FakeDiffForBinaryFiles(filePath)
		if hasFileA && hasFileB {
			if fileAData.ContentHash == fileBData.ContentHash {
				nAdd = 0
				nRemove = 0
				nChange = 0
			} else {
				nAdd = 0
				nRemove = 0
				nChange = 1
			}
		} else {
			if hasFileA {
				nAdd = 1
				nRemove = 0
				nChange = 0
			} else {
				nAdd = 0
				nRemove = 1
				nChange = 0
			}
		}
	} else {
		aBuff := bytes.NewBuffer(nil)
		bBuff := bytes.NewBuffer(nil)
		if hasFileA {
			_, err := fileA.WriteTo(aBuff)
			if err != nil {
				return nil, 0, 0, 0, err
			}
		}
		if hasFileB {
			_, err := fileB.WriteTo(bBuff)
			if err != nil {
				return nil, 0, 0, 0, err
			}
		}
		diffBytes, nAdd, nRemove, nChange = diff.ComputeTextDiff(
			aBuff.Bytes(),
			filePath,
			bBuff.Bytes(),
			filePath)
	}
	return
}

const binaryFileDiffMessage = "(diff is not computed for large/non-text files)"

func fakeDiffForBinaryFiles(name string) []byte {
	fakeDiff, _, _, _ := diff.ComputeTextDiff(
		[]byte("."+binaryFileDiffMessage+".\n"),
		name,
		[]byte("*"+binaryFileDiffMessage+"*\n"),
		name)
	return fakeDiff
}

func writeUnifiedDiff(a Root, b Root, w io.Writer) error {
	di, err := Walk2(a, b)
	if err != nil {
		return err
	}
	for di.CanGet() {
		_, _, visit, _ := di.Get()
		if visit == SecondVisit {
			err = di.Next()
			if err != nil {
				return err
			}
			continue
		}
		diff := di.GetDiff()
		if diff.Type == DiffTypeNoChange {
			di.SkipChildrenOnNext()
			err = di.Next()
			if err != nil {
				return err
			}
			continue
		}

		textDiffBytes, _, _, _, ok, err := di.GetTextDiff()
		if err != nil {
			return err
		}
		if ok {
			_, err = w.Write(textDiffBytes)
			if err != nil {
				return err
			}
		}
		err = di.Next()
		if err != nil {
			return err
		}
	}
	return nil
}

func getPathUnifiedDiff(path string, a Root, b Root) ([]byte, error) {
	aTr, err := a.Tree(path)
	if err != nil && !errors.Is(err, ErrTreeNotFound) {
		return nil, err
	}
	hasA := !errors.Is(err, ErrTreeNotFound)
	var aData Data
	var aFile io.WriterTo
	if hasA {
		aData = aTr.Data()
		if aData.IsDir {
			hasA = false
		} else {
			aFile, err = aTr.GetFile()
			if err != nil {
				return nil, err
			}
		}
	}

	bTr, err := b.Tree(path)
	if err != nil && !errors.Is(err, ErrTreeNotFound) {
		return nil, err
	}
	hasB := !errors.Is(err, ErrTreeNotFound)
	var bData Data
	var bFile io.WriterTo
	if hasB {
		bData = bTr.Data()
		if bData.IsDir {
			hasB = false
		} else {
			bFile, err = bTr.GetFile()
			if err != nil {
				return nil, err
			}
		}
	}
	if !hasA && !hasB {
		return nil, ErrTreeNotFound
	}
	diffBytes, _, _, _, err := getDiffBytes(path, hasA, aData, aFile, hasB, bData, bFile)
	return diffBytes, err
}

func countDiffs(A, B Root) (d TotalDiffCounts, err error) {
	iter, err := Walk2(A, B)
	if err != nil {
		return
	}

	for iter.CanGet() {
		_, _, visit, _ := iter.Get()
		if visit == SecondVisit {
			err = iter.Next()
			if err != nil {
				return
			}
			continue
		}
		diff := iter.GetDiff()
		if diff.Type == DiffTypeUndefined {
			err = iter.Next()
			if err != nil {
				return
			}
			continue
		}
		if diff.Type == DiffTypeNoChange {
			iter.SkipChildrenOnNext()
			err = iter.Next()
			if err != nil {
				return
			}
			continue
		}
		hasFileA := false
		if iter.CanGetA() {
			_, _, _, tr := iter.GetA()
			hasFileA = !tr.Data().IsDir
		}
		hasFileB := false
		if iter.CanGetB() {
			_, _, _, tr := iter.GetB()
			hasFileB = !tr.Data().IsDir
		}

		if !hasFileA && !hasFileB {
			err = iter.Next()
			if err != nil {
				return
			}
			continue
		}
		if hasFileA && !hasFileB {
			d.FilesCreated += 1
		}
		if !hasFileA && hasFileB {
			d.FilesDeleted += 1
		}
		if hasFileA && hasFileB {
			// Note we checked diff.Type!=DiffTypeNoChange
			d.FilesModified += 1
		}

		var nLinesAdd int64
		var nLinesRemoved int64
		var nLinesChanged int64
		var ok bool
		_, nLinesAdd, nLinesRemoved, nLinesChanged, ok, err = iter.GetTextDiff()
		if err != nil {
			return
		}
		if ok {
			d.LinesCreated += nLinesAdd
			d.LinesDeleted += nLinesRemoved
			d.LinesModified += nLinesChanged
		}
		err = iter.Next()
		if err != nil {
			return
		}
	}

	return
}
