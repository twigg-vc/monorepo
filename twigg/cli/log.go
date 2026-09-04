package cli

import (
	"fmt"
	"monorepo/twigg/ansi"
	"monorepo/twigg/commit"
	"monorepo/twigg/treestring2"
	"strings"
)

const DefaultNumCommitsToGoDownInLog = 2

func (ap *app) log(args commandArgs) {
	var err error
	start := ap.s.Current

	const maxNumberOfCommitToGoDownInLog = 50
	numCommitsToGoDownInLog := args.number
	if numCommitsToGoDownInLog < 0 && numCommitsToGoDownInLog != -1 {
		ap.logError(fmt.Sprintf("invalid number %d", numCommitsToGoDownInLog))
		return
	}
	if numCommitsToGoDownInLog > maxNumberOfCommitToGoDownInLog {
		ap.logError(fmt.Sprintf("number can't be larger than %v",
			maxNumberOfCommitToGoDownInLog))
		return
	}
	if numCommitsToGoDownInLog == -1 {
		numCommitsToGoDownInLog = DefaultNumCommitsToGoDownInLog
	}
	for i := 0; i < numCommitsToGoDownInLog; i++ {
		if !start.IsDetachedOrRoot() {
			start, err = ap.ag.GetVersion(start.ParentL, start.ParentV, ap.wl)
			if err != nil {
				ap.logError(err.Error())
				return
			}
		} else {
			break
		}
	}

	var maxHeight = maxNumberOfCommitToGoDownInLog + 2
	tree, err := ap.dfs(false, start, args.all, 0, maxHeight)
	if err != nil {
		ap.logError(err.Error())
		return
	}
	if args.json {
		ap.logTreeAsJson(tree)
		return
	}
	s := treestring2.Get(tree, maxHeight)
	ap.logInfo(s)
}

// Compute the whole tree starting from the provided commit.
func (a *app) dfs(hasHiddenParent bool, c commit.Commit, showAll bool, currentRecursion, maxRecursion int) (treeNode, error) {
	children := []treestring2.Node{}
	if currentRecursion >= maxRecursion {
		return treeNode{isVisible: false}, nil
	}

	for i, childId := range c.Children {
		child, err := a.ag.GetVersion(childId, c.ChildrenVersions[i], a.wl)
		if err != nil {
			return treeNode{}, err
		}
		childNode, err := a.dfs(c.IsHidden || hasHiddenParent, child, showAll, currentRecursion+1, maxRecursion)
		if err != nil {
			return treeNode{}, err
		}
		if !childNode.isVisible {
			continue
		}
		children = append(children, childNode)
	}

	isActive := c.L == a.s.Current.L && c.Version == a.s.Current.Version
	tn := treeNode{
		l:                     c.L,
		version:               c.Version,
		hasServerL:            c.HasServerL,
		serverL:               c.ServerL,
		hasServerV:            c.HasServerV,
		serverV:               c.ServerV,
		msg:                   c.Message,
		status:                c.Status,
		isRestoreOfVersion:    c.IsRestoreOf,
		sucessor:              c.SuccessorVersion,
		isActive:              isActive,
		hasParent:             !c.IsDetachedOrRoot(),
		parentL:               c.ParentL,
		parentV:               c.ParentV,
		hasDiffData:           c.HasDiffData,
		diffDataLinesCreated:  c.DiffDataLinesCreated,
		diffDataLinesDeleted:  c.DiffDataLinesDeleted,
		diffDataLinesModified: c.DiffDataLinesModified,
		isSubmitted:           c.IsSubmitted,
		hasConflicts:          c.HasRebaseConflicts,
		visibleChildren:       children,
		isOnServer:            c.IsOnServer(),
		isManuallyHidden:      c.IsHidden,
		obsReason:             c.ObsReason,
		isVisible:             isVisible(c.IsHidden || hasHiddenParent, isActive, showAll, c, children),
		supportsHyperlinks:    a.supportsHyperlinks,
		serverUrl:             a.s.ServerUrl,
	}
	return tn, nil
}

func isVisible(hasHiddenParent, isActive, showAll bool,
	c commit.Commit, visibleChildren []treestring2.Node) bool {
	if isActive || showAll || len(visibleChildren) > 0 {
		return true
	}
	if hasHiddenParent || c.IsHidden {
		return false
	}
	return c.Status == commit.StatusLatest
}

// Represents a commit that will be logged.
type treeNode struct {
	l                     commit.LocalId
	version               uint64
	hasServerL            bool
	serverL               commit.LocalId
	hasServerV            bool
	serverV               uint64
	msg                   string
	status                commit.Status
	sucessor              uint64
	isRestoreOfVersion    uint64
	isSubmitted           bool
	isActive              bool
	isOnServer            bool
	hasConflicts          bool
	isVisible             bool
	isManuallyHidden      bool
	obsReason             commit.ObsoleteReason
	hasParent             bool
	parentL               commit.LocalId
	parentV               uint64
	hasDiffData           bool
	diffDataLinesCreated  int64
	diffDataLinesDeleted  int64
	diffDataLinesModified int64
	// visibleChildren is []treeNode, but kept as []treestring2.Node so that
	// Children() requires no coversion
	visibleChildren    []treestring2.Node
	supportsHyperlinks bool
	serverUrl          string
}

// Implements treestring.Node interface
var _ treestring2.Node = treeNode{}

func (n treeNode) Children() []treestring2.Node {
	return n.visibleChildren
}

const (
	submittedSuffix               = "Submitted"
	conflictsSuffix               = "Conflicts"
	pushedSufffix                 = "Pushed"
	obsoleteByAmendSuffix         = "--[amend]-->"
	obsoleteByPullOverwriteSuffix = "--[pull]-->"
	obsoleteByManualRebaseSuffix  = "--[rebase]-->"
	obsoleteByAutoRebaseSuffix    = "--[auto rebase]-->"
	obsoleteBySubmitSuffix        = "--[submit]-->"
	activeCommitMarker            = '@'
	inactiveCommitMarker          = '*'
)

func obsoleteByRestoreSuffix(restoredVersion uint64) string {
	return fmt.Sprintf("--[restore v%d]-->", restoredVersion)
}

// Substring that can be used in tests to identify commits that were restored
const obsoleteByRestoreSuffixSubstring = "--[restore v"

// Call function just to check obsoleteByRestoreSuffixSubstring at
// compile time
var _ = func() bool {
	if !strings.Contains(obsoleteByRestoreSuffix(1),
		obsoleteByRestoreSuffixSubstring) {
		panic("obsoleteByRestoreSuffixSubstring is wrong")
	}
	return true
}()

func (n treeNode) FirstLineMessage() string {
	suffixes := []string{}
	idIsColored := true
	if n.isSubmitted {
		suffixes = []string{ansi.BoldGreen.S() + submittedSuffix}
	} else {
		if n.hasConflicts {
			suffixes = append(suffixes, ansi.Red.S()+conflictsSuffix)
		}
		if n.isOnServer {
			suffixes = append(suffixes, ansi.LightGreen.S()+pushedSufffix)
		}
		if n.isManuallyHidden {
			idIsColored = false
			suffixes = append(
				suffixes,
				ansi.SoftYellow.S()+"Hidden")
		}
		if n.status == commit.StatusObsolete {
			idIsColored = false

			var obsReasonSuffix string
			switch n.obsReason {
			case commit.ObsoleteReasonAmend:
				obsReasonSuffix = obsoleteByAmendSuffix
			case commit.ObsoleteReasonPullOverwrite:
				obsReasonSuffix = obsoleteByPullOverwriteSuffix
			case commit.ObsoleteReasonManualRebase:
				obsReasonSuffix = obsoleteByManualRebaseSuffix
			case commit.ObsoleteReasonAutoRebaseOfChildren:
				obsReasonSuffix = obsoleteByAutoRebaseSuffix
			case commit.ObsoleteReasonSubmit:
				obsReasonSuffix = obsoleteBySubmitSuffix
			case commit.ObsoleteReasonRestored:
				obsReasonSuffix = obsoleteByRestoreSuffix(n.isRestoreOfVersion)
			default:
				panic(fmt.Sprintf(
					"commit %d is obsolete by %d reason",
					n.l, n.obsReason))
			}

			suffixes = append(
				suffixes,
				ansi.SoftYellow.S()+obsReasonSuffix+" "+
					commitStringByLAndV(n.l, n.sucessor, false,
						0, false, 0, false,
						n.supportsHyperlinks, n.serverUrl,
						/*onlyShowServerId*/ false))
		}
	}
	var suffix string
	if len(suffixes) == 1 {
		suffix = suffixes[0]
	}
	if len(suffixes) > 1 {
		suffix = ansi.Reset.S() + "[" +
			strings.Join(suffixes, ansi.Reset.S()+", ") +
			ansi.Reset.S() + "]"
	}
	prefix := ""
	if !idIsColored {
		prefix = ansi.Gray.S()
	}
	return fmt.Sprintf(
		" %s%s %v%v", prefix, commitStringByLAndV(n.l, n.version,
			n.hasServerL, n.serverL, n.hasServerV, n.serverV, idIsColored,
			n.supportsHyperlinks, n.serverUrl,
			/*onlyShowServerId*/ false),
		suffix, ansi.Reset)
}
func (n treeNode) SecondLineMessage() string {
	if n.l == 0 {
		n.msg = firstCommitMsg
	}
	if n.status == commit.StatusObsolete {
		return ansi.Gray.S() + " " + n.msg + ansi.Reset.S()
	}
	return " " + n.msg
}
func (n treeNode) Marker() rune {
	if n.isActive {
		return activeCommitMarker
	}
	return inactiveCommitMarker
}
func (n treeNode) MarkerColor() ansi.Color {
	if n.isActive {
		return ansi.BoldYellow
	}
	return ansi.White
}