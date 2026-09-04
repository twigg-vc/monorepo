// Implement tests for the `ignore` library
package gitignore

import (
	"os"
	"path/filepath"

	"fmt"
	"runtime"
	"testing"
)

const (
	TEST_DIR = "test_fixtures"
)

////////////////////////////////////////////////////////////

// Helper function to setup a test fixture dir and write to
// it a file with the name "fname" and content "content"
func writeFileToTestDir(fname, content string) {
	testDirPath := "." + string(filepath.Separator) + TEST_DIR
	testFilePath := testDirPath + string(filepath.Separator) + fname
	_ = os.MkdirAll(testDirPath, 0755)
	_ = os.WriteFile(testFilePath, []byte(content), os.ModePerm)
}

func cleanupTestDir() {
	_ = os.RemoveAll(fmt.Sprintf(".%s%s", string(filepath.Separator), TEST_DIR))
}

////////////////////////////////////////////////////////////

// Validate "CompileIgnoreLines()"
func TestCompileIgnoreLines(t *testing.T) {
	lines := []string{"abc/def", "a/b/c", "b"}
	object := CompileIgnoreLines(lines...)

	// MatchesPath
	// Paths which are targeted by the above "lines"
	checkIsTrue(t, object.MatchesPath("abc/def/child"), "abc/def/child should match")
	checkIsTrue(t, object.MatchesPath("a/b/c/d"), "a/b/c/d should match")

	// Paths which are not targeted by the above "lines"
	checkIsFalse(t, object.MatchesPath("abc"), "abc should not match")
	checkIsFalse(t, object.MatchesPath("def"), "def should not match")
	checkIsFalse(t, object.MatchesPath("bd"), "bd should not match")

	object = CompileIgnoreLines("abc/def", "a/b/c", "b")

	// Paths which are targeted by the above "lines"
	checkIsTrue(t, object.MatchesPath("abc/def/child"), "abc/def/child should match")
	checkIsTrue(t, object.MatchesPath("a/b/c/d"), "a/b/c/d should match")

	// Paths which are not targeted by the above "lines"
	checkIsFalse(t, object.MatchesPath("abc"), "abc should not match")
	checkIsFalse(t, object.MatchesPath("def"), "def should not match")
	checkIsFalse(t, object.MatchesPath("bd"), "bd should not match")
}

// Validate the invalid files
func TestCompileIgnoreFile_InvalidFile(t *testing.T) {
	object, err := CompileIgnoreFile("./test_fixtures/invalid.file")
	checkIsNilGitignore(t, object, "object should be nil")
	checkIsNotNilErr(t, err, "err should be unknown file / dir")
}

// Validate the an empty files
func TestCompileIgnoreLines_EmptyFile(t *testing.T) {
	writeFileToTestDir("test.gitignore", ``)
	defer cleanupTestDir()

	object, err := CompileIgnoreFile("./test_fixtures/test.gitignore")
	checkIsNilErr(t, err, "err should be nil")
	checkIsNotNilGitignore(t, object, "object should not be nil")

	checkIsFalse(t, object.MatchesPath("a"), "should not match any path")
	checkIsFalse(t, object.MatchesPath("a/b"), "should not match any path")
	checkIsFalse(t, object.MatchesPath(".foobar"), "should not match any path")
}

// Validate the correct handling of the negation operator "!"
func TestCompileIgnoreLines_HandleIncludePattern(t *testing.T) {
	writeFileToTestDir("test.gitignore", `
# exclude everything except directory foo/bar
/*
!/foo
/foo/*
!/foo/bar
`)
	defer cleanupTestDir()

	object, err := CompileIgnoreFile("./test_fixtures/test.gitignore")
	checkIsNilErr(t, err, "err should be nil")
	checkIsNotNilGitignore(t, object, "object should not be nil")

	checkIsTrue(t, object.MatchesPath("a"), "a should match")
	checkIsTrue(t, object.MatchesPath("foo/baz"), "foo/baz should match")
	checkIsFalse(t, object.MatchesPath("foo"), "foo should not match")
	checkIsFalse(t, object.MatchesPath("/foo/bar"), "/foo/bar should not match")
}

// Validate the correct handling of comments and empty lines
func TestCompileIgnoreLines_HandleSpaces(t *testing.T) {
	writeFileToTestDir("test.gitignore", `
#
# A comment

# Another comment


    # Invalid Comment

abc/def
`)
	defer cleanupTestDir()

	object, err := CompileIgnoreFile("./test_fixtures/test.gitignore")
	checkIsNilErr(t, err, "err should be nil")
	checkIsNotNilGitignore(t, object, "object should not be nil")

	checkN(t, 2, len(object.patterns), "should have two regex pattern")
	checkIsFalse(t, object.MatchesPath("abc/abc"), "/abc/abc should not match")
	checkIsTrue(t, object.MatchesPath("abc/def"), "/abc/def should match")
}

// Validate the correct handling of leading / chars
func TestCompileIgnoreLines_HandleLeadingSlash(t *testing.T) {
	writeFileToTestDir("test.gitignore", `
/a/b/c
d/e/f
/g
`)
	defer cleanupTestDir()

	object, err := CompileIgnoreFile("./test_fixtures/test.gitignore")
	checkIsNilErr(t, err, "err should be nil")
	checkIsNotNilGitignore(t, object, "object should not be nil")

	checkN(t, 3, len(object.patterns), "should have 3 regex patterns")
	checkIsTrue(t, object.MatchesPath("a/b/c"), "a/b/c should match")
	checkIsTrue(t, object.MatchesPath("a/b/c/d"), "a/b/c/d should match")
	checkIsTrue(t, object.MatchesPath("d/e/f"), "d/e/f should match")
	checkIsTrue(t, object.MatchesPath("g"), "g should match")
}

// Validate the correct handling of files starting with # or !
func TestCompileIgnoreLines_HandleLeadingSpecialChars(t *testing.T) {
	writeFileToTestDir("test.gitignore", `
# Comment
\#file.txt
\!file.txt
file.txt
`)
	defer cleanupTestDir()

	object, err := CompileIgnoreFile("./test_fixtures/test.gitignore")
	checkIsNilErr(t, err, "err should be nil")
	checkIsNotNilGitignore(t, object, "object should not be nil")

	checkIsTrue(t, object.MatchesPath("#file.txt"), "#file.txt should match")
	checkIsTrue(t, object.MatchesPath("!file.txt"), "!file.txt should match")
	checkIsTrue(t, object.MatchesPath("a/!file.txt"), "a/!file.txt should match")
	checkIsTrue(t, object.MatchesPath("file.txt"), "file.txt should match")
	checkIsTrue(t, object.MatchesPath("a/file.txt"), "a/file.txt should match")
	checkIsFalse(t, object.MatchesPath("file2.txt"), "file2.txt should not match")

}

// Validate the correct handling matching files only within a given folder
func TestCompileIgnoreLines_HandleAllFilesInDir(t *testing.T) {
	writeFileToTestDir("test.gitignore", `
Documentation/*.html
`)
	defer cleanupTestDir()

	object, err := CompileIgnoreFile("./test_fixtures/test.gitignore")
	checkIsNilErr(t, err, "err should be nil")
	checkIsNotNilGitignore(t, object, "object should not be nil")

	checkIsTrue(t, object.MatchesPath("Documentation/git.html"), "Documentation/git.html should match")
	checkIsFalse(t, object.MatchesPath("Documentation/ppc/ppc.html"), "Documentation/ppc/ppc.html should not match")
	checkIsFalse(t, object.MatchesPath("tools/perf/Documentation/perf.html"), "tools/perf/Documentation/perf.html should not match")
}

// Validate the correct handling of "**"
func TestCompileIgnoreLines_HandleDoubleStar(t *testing.T) {
	writeFileToTestDir("test.gitignore", `
**/foo
bar
`)
	defer cleanupTestDir()

	object, err := CompileIgnoreFile("./test_fixtures/test.gitignore")
	checkIsNilErr(t, err, "err should be nil")
	checkIsNotNilGitignore(t, object, "object should not be nil")

	checkIsTrue(t, object.MatchesPath("foo"), "foo should match")
	checkIsTrue(t, object.MatchesPath("baz/foo"), "baz/foo should match")
	checkIsTrue(t, object.MatchesPath("bar"), "bar should match")
	checkIsTrue(t, object.MatchesPath("baz/bar"), "baz/bar should match")
}

// Validate the correct handling of leading slash
func TestCompileIgnoreLines_HandleLeadingSlashPath(t *testing.T) {
	writeFileToTestDir("test.gitignore", `
/*.c
`)
	defer cleanupTestDir()

	object, err := CompileIgnoreFile("./test_fixtures/test.gitignore")
	checkIsNilErr(t, err, "err should be nil")
	checkIsNotNilGitignore(t, object, "object should not be nil")

	checkIsTrue(t, object.MatchesPath("hello.c"), "hello.c should match")
	checkIsFalse(t, object.MatchesPath("foo/hello.c"), "foo/hello.c should not match")
}

func TestCompileIgnoreFileAndLines(t *testing.T) {
	writeFileToTestDir("test.gitignore", `
/*.c
`)
	defer cleanupTestDir()

	object, err := CompileIgnoreFileAndLines("./test_fixtures/test.gitignore", "**/foo", "bar")
	checkIsNilErr(t, err, "err should be nil")
	checkIsNotNilGitignore(t, object, "object should not be nil")

	checkIsTrue(t, object.MatchesPath("hello.c"), "hello.c should match")
	checkIsFalse(t, object.MatchesPath("baz/hello.c"), "baz/hello.c should not match")

	checkIsTrue(t, object.MatchesPath("foo"), "foo should match")
	checkIsTrue(t, object.MatchesPath("baz/foo"), "baz/foo should match")
	checkIsTrue(t, object.MatchesPath("bar"), "bar should match")
	checkIsTrue(t, object.MatchesPath("baz/bar"), "baz/bar should match")
}

func TestCompileIgnoreFileAndLines_InvalidFile(t *testing.T) {
	object, err := CompileIgnoreFileAndLines("./test_fixtures/invalid.file")
	checkIsNilGitignore(t, object, "object should be nil")
	checkIsNotNilErr(t, err, "err should be unknown file / dir")
}

func ExampleCompileIgnoreLines() {
	ignoreObject := CompileIgnoreLines([]string{"node_modules", "*.out", "foo/*.c"}...)

	// You can test the ignoreObject against various paths using the
	// "MatchesPath()" interface method. This pretty much is up to
	// the users interpretation. In the case of a ".gitignore" file,
	// a "match" would indicate that a given path would be ignored.
	fmt.Println(ignoreObject.MatchesPath("node_modules/test/foo.js"))
	fmt.Println(ignoreObject.MatchesPath("node_modules2/test.out"))
	fmt.Println(ignoreObject.MatchesPath("test/foo.js"))

	// Output:
	// true
	// true
	// false
}

func TestCompileIgnoreLines_CheckNestedDotFiles(t *testing.T) {
	lines := []string{
		"**/external/**/*.md",
		"**/external/**/*.json",
		"**/external/**/*.gzip",
		"**/external/**/.*ignore",

		"**/external/foobar/*.css",
		"**/external/barfoo/less",
		"**/external/barfoo/scss",
	}
	object := CompileIgnoreLines(lines...)
	checkIsNotNilGitignore(t, object, "returned object should not be nil")

	checkIsTrue(t, object.MatchesPath("external/foobar/angular.foo.css"), "external/foobar/angular.foo.css")
	checkIsTrue(t, object.MatchesPath("external/barfoo/.gitignore"), "external/barfoo/.gitignore")
	checkIsTrue(t, object.MatchesPath("external/barfoo/.bower.json"), "external/barfoo/.bower.json")
}

func TestCompileIgnoreLines_CarriageReturn(t *testing.T) {
	lines := []string{"abc/def\r", "a/b/c\r", "b\r"}
	object := CompileIgnoreLines(lines...)

	checkIsTrue(t, object.MatchesPath("abc/def/child"), "abc/def/child should match")
	checkIsTrue(t, object.MatchesPath("a/b/c/d"), "a/b/c/d should match")

	checkIsFalse(t, object.MatchesPath("abc"), "abc should not match")
	checkIsFalse(t, object.MatchesPath("def"), "def should not match")
	checkIsFalse(t, object.MatchesPath("bd"), "bd should not match")
}

func TestCompileIgnoreLines_WindowsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		return
	}
	lines := []string{"abc/def", "a/b/c", "b"}
	object := CompileIgnoreLines(lines...)

	checkIsTrue(t, object.MatchesPath("abc\\def\\child"), "abc\\def\\child should match")
	checkIsTrue(t, object.MatchesPath("a\\b\\c\\d"), "a\\b\\c\\d should match")
}

func TestWildCardFiles(t *testing.T) {
	gitIgnore := []string{"*.swp", "/foo/*.wat", "bar/*.txt"}
	object := CompileIgnoreLines(gitIgnore...)

	// Paths which are targeted by the above "lines"
	checkIsTrue(t, object.MatchesPath("yo.swp"), "should ignore all swp files")
	checkIsTrue(t, object.MatchesPath("something/else/but/it/hasyo.swp"), "should ignore all swp files in other directories")

	checkIsTrue(t, object.MatchesPath("foo/bar.wat"), "should ignore all wat files in foo - nonpreceding /")
	checkIsTrue(t, object.MatchesPath("/foo/something.wat"), "should ignore all wat files in foo - preceding /")

	checkIsTrue(t, object.MatchesPath("bar/something.txt"), "should ignore all txt files in bar - nonpreceding /")
	checkIsTrue(t, object.MatchesPath("/bar/somethingelse.txt"), "should ignore all txt files in bar - preceding /")

	// Paths which are not targeted by the above "lines"
	checkIsFalse(t, object.MatchesPath("something/not/infoo/wat.wat"), "wat files should only be ignored in foo")
	checkIsFalse(t, object.MatchesPath("something/not/infoo/wat.txt"), "txt files should only be ignored in bar")
}

func TestPrecedingSlash(t *testing.T) {
	gitIgnore := []string{"/foo", "bar/"}
	object := CompileIgnoreLines(gitIgnore...)

	checkIsTrue(t, object.MatchesPath("foo/bar.wat"), "should ignore all files in foo - nonpreceding /")
	checkIsTrue(t, object.MatchesPath("/foo/something.txt"), "should ignore all files in foo - preceding /")

	checkIsTrue(t, object.MatchesPath("bar/something.txt"), "should ignore all files in bar - nonpreceding /")
	checkIsTrue(t, object.MatchesPath("/bar/somethingelse.go"), "should ignore all files in bar - preceding /")
	checkIsTrue(t, object.MatchesPath("/boo/something/bar/boo.txt"), "should block all files if bar is a sub directory")

	checkIsFalse(t, object.MatchesPath("something/foo/something.txt"), "should only ignore top level foo directories- not nested")
}

func TestMatchesLineNumbers(t *testing.T) {
	gitIgnore := []string{"/foo", "bar/", "*.swp"}
	object := CompileIgnoreLines(gitIgnore...)

	var matchesPath bool
	var reason *IgnorePattern

	// /foo
	matchesPath, reason = object.MatchesPathHow("foo/bar.wat")
	checkIsTrue(t, matchesPath, "should ignore all files in foo - nonpreceding /")
	checkIsNotNilIgnorePattern(t, reason, "reason should not be nil")
	checkN(t, 1, reason.LineNo, "should match with line 1")
	checkString(t, gitIgnore[0], reason.Line, "should match with line /foo")

	matchesPath, reason = object.MatchesPathHow("/foo/something.txt")
	checkIsTrue(t, matchesPath, "should ignore all files in foo - preceding /")
	checkIsNotNilIgnorePattern(t, reason, "reason should not be nil")
	checkN(t, 1, reason.LineNo, "should match with line 1")
	checkString(t, gitIgnore[0], reason.Line, "should match with line /foo")

	// bar/
	matchesPath, reason = object.MatchesPathHow("bar/something.txt")
	checkIsTrue(t, matchesPath, "should ignore all files in bar - nonpreceding /")
	checkIsNotNilIgnorePattern(t, reason, "reason should not be nil")
	checkN(t, 2, reason.LineNo, "should match with line 2")
	checkString(t, gitIgnore[1], reason.Line, "should match with line bar/")

	matchesPath, reason = object.MatchesPathHow("/bar/somethingelse.go")
	checkIsTrue(t, matchesPath, "should ignore all files in bar - preceding /")
	checkIsNotNilIgnorePattern(t, reason, "reason should not be nil")
	checkN(t, 2, reason.LineNo, "should match with line 2")
	checkString(t, gitIgnore[1], reason.Line, "should match with line bar/")

	matchesPath, reason = object.MatchesPathHow("/boo/something/bar/boo.txt")
	checkIsTrue(t, matchesPath, "should block all files if bar is a sub directory")
	checkIsNotNilIgnorePattern(t, reason, "reason should not be nil")
	checkN(t, 2, reason.LineNo, "should match with line 2")
	checkString(t, gitIgnore[1], reason.Line, "should match with line bar/")

	// *.swp
	matchesPath, reason = object.MatchesPathHow("yo.swp")
	checkIsTrue(t, matchesPath, "should ignore all swp files")
	checkIsNotNilIgnorePattern(t, reason, "reason should not be nil")
	checkN(t, 3, reason.LineNo, "should match with line 3")
	checkString(t, gitIgnore[2], reason.Line, "should match with line *.swp")

	matchesPath, reason = object.MatchesPathHow("something/else/but/it/hasyo.swp")
	checkIsTrue(t, matchesPath, "should ignore all swp files in other directories")
	checkIsNotNilIgnorePattern(t, reason, "reason should not be nil")
	checkN(t, 3, reason.LineNo, "should match with line 3")
	checkString(t, gitIgnore[2], reason.Line, "should match with line *.swp")

	// other
	matchesPath, reason = object.MatchesPathHow("something/foo/something.txt")
	checkIsFalse(t, matchesPath, "should only ignore top level foo directories- not nested")
	checkIsNilIgnorePattern(t, reason, "reason should be nil as no match should happen")
}

func TestSimple(test *testing.T) {
	lines := []string{"foo"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, "foo")
	shouldMatch(test, object, "foo/")
	shouldMatch(test, object, "/foo")
	shouldNotMatch(test, object, "fooo")
	shouldNotMatch(test, object, "ofoo")
}

func TestAnywhere(test *testing.T) {
	lines := []string{"**/foo"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, "foo")
	shouldMatch(test, object, "foo/")
	shouldMatch(test, object, "/foo")
	shouldNotMatch(test, object, "fooo")
	shouldNotMatch(test, object, "ofoo")
}

func TestAnywhereFromRoot(test *testing.T) {
	lines := []string{"/**/foo"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, "foo")
	shouldMatch(test, object, "foo/")
	shouldMatch(test, object, "/foo")
	shouldNotMatch(test, object, "fooo")
	shouldNotMatch(test, object, "ofoo")
}

func TestSimpleDir(test *testing.T) {
	lines := []string{"foo/"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, "foo/")
	shouldMatch(test, object, "foo/a")
	shouldMatch(test, object, "/foo/")
	shouldNotMatch(test, object, "foo")
	shouldNotMatch(test, object, "/foo")
}

func TestRootExtensionOnly(test *testing.T) {
	lines := []string{"/.js"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, ".js")
	shouldMatch(test, object, ".js/")
	shouldMatch(test, object, ".js/a")
	// ???
	// shouldNotMatch(test, object, "/.js")
	shouldNotMatch(test, object, ".jsa")
}

func TestRootExtension(test *testing.T) {
	lines := []string{"/*.js"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, ".js")
	shouldMatch(test, object, ".js/")
	shouldMatch(test, object, ".js/a")
	shouldMatch(test, object, "a.js/a")
	shouldMatch(test, object, "a.js/a.js")
	// ???
	// shouldNotMatch(test, object, "/.js")
	shouldNotMatch(test, object, ".jsa")
}

func TestExtension(test *testing.T) {
	lines := []string{"*.js"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, ".js")
	shouldMatch(test, object, ".js/")
	shouldMatch(test, object, ".js/a")
	shouldMatch(test, object, "a.js/a")
	shouldMatch(test, object, "a.js/a.js")
	shouldMatch(test, object, "/.js")
	shouldNotMatch(test, object, ".jsa")
}

func TestStarExtension(test *testing.T) {
	lines := []string{".js*"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, ".js")
	shouldMatch(test, object, ".js/")
	shouldMatch(test, object, ".js/a")
	shouldNotMatch(test, object, "a.js/a")
	shouldNotMatch(test, object, "a.js/a.js")
	shouldMatch(test, object, "/.js")
	shouldMatch(test, object, ".jsa")
}

func TestDoubleStar(test *testing.T) {
	lines := []string{"foo/**/"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, "foo/")
	shouldMatch(test, object, "foo/abc/")
	shouldMatch(test, object, "foo/x/y/z/")
	shouldNotMatch(test, object, "foo")
	shouldNotMatch(test, object, "/foo")
}

func TestStars(test *testing.T) {
	lines := []string{"foo/**/*.bar"}
	object := CompileIgnoreLines(lines...)

	shouldNotMatch(test, object, "foo/")
	shouldNotMatch(test, object, "abc.bar")
	shouldMatch(test, object, "foo/abc.bar")
	shouldMatch(test, object, "foo/abc.bar/")
	shouldMatch(test, object, "foo/x/y/z.bar")
	shouldMatch(test, object, "foo/x/y/z.bar/")
}

func TestCases_Comment(test *testing.T) {
	lines := []string{"#abc"}
	object := CompileIgnoreLines(lines...)

	shouldNotMatch(test, object, "#abc")
}

func TestCases_EscapedComment(test *testing.T) {
	lines := []string{`\#abc`}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, "#abc")
}

func TestCases_CouldFilterPaths(test *testing.T) {
	lines := []string{"abc", "!abc/b"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, "abc/a.js")
	shouldNotMatch(test, object, "abc/b/b.js")
}

func TestCases_IgnoreSelect(test *testing.T) {
	lines := []string{"abc", "!abc/b", "#e", `\#f`}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, "abc/a.js")
	shouldNotMatch(test, object, "abc/b/b.js")
	shouldNotMatch(test, object, "#e")
	shouldMatch(test, object, "#f")
}

func TestCases_EscapeRegexMetacharacters(test *testing.T) {
	lines := []string{"*.js", `!\*.js`, "!a#b.js", "!?.js", "#abc", `\#abc`}
	object := CompileIgnoreLines(lines...)

	shouldNotMatch(test, object, "*.js")
	shouldMatch(test, object, "abc.js")
	shouldNotMatch(test, object, "a#b.js")
	shouldNotMatch(test, object, "abc")
	shouldMatch(test, object, "#abc")
	shouldNotMatch(test, object, "?.js")
}

func TestCases_QuestionMark(test *testing.T) {
	lines := []string{"/.project", "thumbs.db", "*.swp", ".sonar/*", ".*.sw?"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, ".project")
	shouldNotMatch(test, object, "abc/.project")
	shouldNotMatch(test, object, ".a.sw")
	shouldMatch(test, object, ".a.sw?")
	shouldMatch(test, object, "thumbs.db")
}

func TestCases_DirEndedWithStar(test *testing.T) {
	lines := []string{"abc/*"}
	object := CompileIgnoreLines(lines...)

	shouldNotMatch(test, object, "abc")
}

func TestCases_FileEndedWithStar(test *testing.T) {
	lines := []string{"abc.js*"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, "abc.js/")
	shouldMatch(test, object, "abc.js/abc")
	shouldMatch(test, object, "abc.jsa/")
	shouldMatch(test, object, "abc.jsa/abc")
}

func TestCases_WildcardAsFilename(test *testing.T) {
	lines := []string{"*.b"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, "b/a.b")
	shouldMatch(test, object, "b/.b")
	shouldNotMatch(test, object, "b/.ba")
	shouldMatch(test, object, "b/c/a.b")
}

func TestCases_SlashAtBeginningAndComeWithWildcard(test *testing.T) {
	lines := []string{"/*.c"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, ".c")
	shouldMatch(test, object, "c.c")
	shouldNotMatch(test, object, "c/c.c")
	shouldNotMatch(test, object, "c/d")
}

func TestCases_DotFile(test *testing.T) {
	lines := []string{".d"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, ".d")
	shouldNotMatch(test, object, ".dd")
	shouldNotMatch(test, object, "d.d")
	shouldMatch(test, object, "d/.d")
	shouldNotMatch(test, object, "d/d.d")
	shouldNotMatch(test, object, "d/e")
}

func TestCases_DotDir(test *testing.T) {
	lines := []string{".e"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, ".e/")
	shouldNotMatch(test, object, ".ee/")
	shouldNotMatch(test, object, "e.e/")
	shouldMatch(test, object, ".e/e")
	shouldMatch(test, object, "e/.e")
	shouldNotMatch(test, object, "e/e.e")
	shouldNotMatch(test, object, "e/f")
}

func TestCases_PatternOnce(test *testing.T) {
	lines := []string{"node_modules/"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, "node_modules/gulp/node_modules/abc.md")
	shouldMatch(test, object, "node_modules/gulp/node_modules/abc.json")
}

func TestCases_PatternTwice(test *testing.T) {
	lines := []string{"node_modules/", "node_modules/"}
	object := CompileIgnoreLines(lines...)

	shouldMatch(test, object, "node_modules/gulp/node_modules/abc.md")
	shouldMatch(test, object, "node_modules/gulp/node_modules/abc.json")
}

func checkIsNilErr(t *testing.T, err error, errMsg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("checkIsNilErr failed for %s", errMsg)
	}
}
func checkIsNotNilErr(t *testing.T, err error, errMsg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("checkIsNotNilErr failed for %s", errMsg)
	}
}
func checkIsTrue(t *testing.T, x bool, errMsg string) {
	t.Helper()
	if !x {
		t.Fatalf("checkIsTrue failed for %s", errMsg)
	}
}
func checkIsFalse(t *testing.T, x bool, errMsg string) {
	t.Helper()
	if x {
		t.Fatalf("checkIsFalse failed for %s", errMsg)
	}
}
func checkString(t *testing.T, got string, expected string, errMsg string) {
	t.Helper()
	if got != expected {
		t.Fatalf("expected %q got %q: %s", expected, got, errMsg)
	}
}
func checkN(t *testing.T, got int, expected int, errMsg string) {
	if got != expected {
		t.Fatalf("expected %d got %d: %s", expected, got, errMsg)
	}
}
func checkIsNilGitignore(t *testing.T, g *GitIgnore, errMsg string) {
	t.Helper()
	if g != nil {
		t.Fatalf("checkIsNilGitignore failed for %s", errMsg)
	}
}
func checkIsNotNilGitignore(t *testing.T, g *GitIgnore, errMsg string) {
	t.Helper()
	if g == nil {
		t.Fatalf("checkIsNotNilGitignore failed for %s", errMsg)
	}
}
func checkIsNilIgnorePattern(t *testing.T, g *IgnorePattern, errMsg string) {
	t.Helper()
	if g != nil {
		t.Fatalf("checkIsNilIgnorePattern failed for %s", errMsg)
	}
}
func checkIsNotNilIgnorePattern(t *testing.T, g *IgnorePattern, errMsg string) {
	t.Helper()
	if g == nil {
		t.Fatalf("checkIsNotNilIgnorePattern failed for %s", errMsg)
	}
}
func shouldMatch(test *testing.T, object *GitIgnore, path string) {
	checkIsTrue(test, object.MatchesPath(path), path+" should match")
}
func shouldNotMatch(test *testing.T, object *GitIgnore, path string) {
	checkIsFalse(test, object.MatchesPath(path), path+" should not match")
}