import { expect } from '@open-wc/testing';
import { ParseUnifiedDiff, LinesSet } from './unified-diff';

// Renders a LinesSet as "<number>:<line>"
function linesSetToStr(s: LinesSet): string[] {
    return s.Raw.map((raw, i) => s.Nums[i] + ":" + raw);
}

describe('ParseUnifiedDiff', () => {
    it('returns empty for random text', () => {
        const d = ParseUnifiedDiff("this is not a diff");
        expect(linesSetToStr(d.Left)).to.deep.equal([]);
        expect(linesSetToStr(d.Right)).to.deep.equal([]);
    });

    it('reads back a file that did not change', () => {
        const diff = [
            "diff a.txt a.txt",
            "--- a.txt",
            "+++ a.txt",
            "@@ -1,2 +1,2 @@",
            " one",
            " two"
        ].join("\n");
        const d = ParseUnifiedDiff(diff);
        expect(linesSetToStr(d.Left)).to.deep.equal(["1:one", "2:two"]);
        expect(linesSetToStr(d.Right)).to.deep.equal(["1:one", "2:two"]);
    });

    it('puts a rewritten line on both sides', () => {
        const diff = [
            "diff a.txt a.txt",
            "--- a.txt",
            "+++ a.txt",
            "@@ -1,3 +1,3 @@",
            " one",
            "-two",
            "+TWO",
            " three"
        ].join("\n");
        const d = ParseUnifiedDiff(diff);
        expect(linesSetToStr(d.Left)).to.deep.equal(["1:one", "2:two", "3:three"]);
        expect(linesSetToStr(d.Right)).to.deep.equal(["1:one", "2:TWO", "3:three"]);
    });

    it('leaves an inserted line out of the left side', () => {
        const diff = [
            "diff a.txt a.txt",
            "--- a.txt",
            "+++ a.txt",
            "@@ -1,3 +1,3 @@",
            " one",
            "+TWO",
            " three"
        ].join("\n");
        const d = ParseUnifiedDiff(diff);
        expect(linesSetToStr(d.Left)).to.deep.equal(["1:one", "2:three"]);
        expect(linesSetToStr(d.Right)).to.deep.equal(["1:one", "2:TWO", "3:three"]);
    });

    it('leaves a removed line out of the right side', () => {
        const diff = [
            "diff a.txt a.txt",
            "--- a.txt",
            "+++ a.txt",
            "@@ -1,3 +1,3 @@",
            " one",
            "-two",
            " three"
        ].join("\n");
        const d = ParseUnifiedDiff(diff);
        expect(linesSetToStr(d.Left)).to.deep.equal(["1:one", "2:two", "3:three"]);
        expect(linesSetToStr(d.Right)).to.deep.equal(["1:one", "2:three"]);
    });

    it('numbers the lines from the hunk header', () => {
        const diff = [
            "diff a.txt a.txt",
            "--- a.txt",
            "+++ a.txt",
            "@@ -10,2 +20,2 @@",
            " one",
            " two",
        ].join("\n");
        const d = ParseUnifiedDiff(diff);
        expect(linesSetToStr(d.Left)).to.deep.equal(["10:one", "11:two"]);
        expect(linesSetToStr(d.Right)).to.deep.equal(["20:one", "21:two"]);
    });

    it('counts the two sides on their own', () => {
        // The right side is a line ahead after the insert, and the left one a
        // line behind after the delete.
        const diff = [
            "diff a.txt a.txt",
            "--- a.txt",
            "+++ a.txt",
            "@@ -1,4 +1,4 @@",
            " a",
            "+b",
            " c",
            "-d",
            " e"
        ].join("\n");
        const d = ParseUnifiedDiff(diff);
        expect(linesSetToStr(d.Left)).to.deep.equal(["1:a", "2:c", "3:d", "4:e"]);
        expect(linesSetToStr(d.Right)).to.deep.equal(["1:a", "2:b", "3:c", "4:e"]);
    });

    it('keeps the blank lines of the file', () => {
        const diff = [
            "diff a.txt a.txt",
            "--- a.txt",
            "+++ a.txt",
            "@@ -1,3 +1,3 @@",
            " one",
            "  ",
            " three",
        ].join("\n");
        const d = ParseUnifiedDiff(diff);
        expect(linesSetToStr(d.Left)).to.deep.equal(["1:one", "2: ", "3:three"]);
    });

    it('ignores the headers and the no-newline marker', () => {
        const diff = [
            "diff a.txt a.txt",
            "--- a.txt",
            "+++ a.txt",
            "@@ -1,1 +1,1 @@",
            "-one",
            "\\ No newline at end of file",
            "+ONE",
            "\\ No newline at end of file",
        ].join("\n");
        const d = ParseUnifiedDiff(diff);
        expect(linesSetToStr(d.Left)).to.deep.equal(["1:one"]);
        expect(linesSetToStr(d.Right)).to.deep.equal(["1:ONE"]);
    });

    it('reads a file that is only added or only removed', () => {
        const diffA = [
            "diff a.txt a.txt",
            "--- a.txt",
            "+++ a.txt",
            "@@ -1,2 +1,2 @@",
            "+one",
            "+two",
        ].join("\n");
        const add = ParseUnifiedDiff(diffA);
        expect(linesSetToStr(add.Left)).to.deep.equal([]);
        expect(linesSetToStr(add.Right)).to.deep.equal(["1:one", "2:two"]);

        const diffR = [
            "diff a.txt a.txt",
            "--- a.txt",
            "+++ a.txt",
            "@@ -1,2 +1,2 @@",
            "-one",
            "-two",
        ].join("\n");
        const removed = ParseUnifiedDiff(diffR);
        expect(linesSetToStr(removed.Left)).to.deep.equal(["1:one", "2:two"]);
        expect(linesSetToStr(removed.Right)).to.deep.equal([]);
    });

    it('gives back two empty sides for an empty diff', () => {
        const d = ParseUnifiedDiff("");
        expect(d.Left.Raw).to.deep.equal([]);
        expect(d.Right.Raw).to.deep.equal([]);
    });

    it('throws on many hunks', () => {
        const diff = [
            "diff a.txt a.txt",
            "--- a.txt",
            "+++ a.txt",
            "@@ -1,1 +1,1 @@",
            " one",
            "@@ -9,1 +9,1 @@",
            " nine",
        ].join("\n");
        expect(() => ParseUnifiedDiff(diff)).to.throw();
    });
});
