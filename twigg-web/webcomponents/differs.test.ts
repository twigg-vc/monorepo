import { expect } from '@open-wc/testing';
import { MyersDiffer } from './differs';
// rowsToStr renders the rows as "left|right", with a dot for a missing side.
function rowsToStr(left: string[], right: string[]): string[] {
    return new MyersDiffer().Diff(left, right)
        .map(r => lineAt(left, r.Left) + "|" + lineAt(right, r.Right));
}
function lineAt(lines: string[], i: number | undefined): string {
    if (i === undefined) {
        return ".";
    }
    return lines[i];
}
describe('MyersDiffer', () => {
    it('pairs up the lines of an unchanged file', () => {
        const a = ["one", "two", "three"];
        expect(rowsToStr(a, a)).to.deep.equal(["one|one", "two|two", "three|three"]);
    });
    it('gives an inserted line a row of its own', () => {
        expect(rowsToStr(["one", "three"], ["one", "two", "three"]))
            .to.deep.equal(["one|one", ".|two", "three|three"]);
    });
    it('marks what changed between the two lines of a row', () => {
        const rows = new MyersDiffer().Diff(["abcdef"], ["abXdef"]);
        expect(rows.length).to.equal(1);
        expect(rows[0].LeftRanges).to.deep.equal([{ Start: 2, End: 3 }]);
        expect(rows[0].RightRanges).to.deep.equal([{ Start: 2, End: 3 }]);
    });
    it('marks nothing on a pair of identical lines', () => {
        const rows = new MyersDiffer().Diff(["same"], ["same"]);
        expect(rows[0].LeftRanges).to.deep.equal([]);
        expect(rows[0].RightRanges).to.deep.equal([]);
    });
    it('marks nothing on a row with a line on one side only', () => {
        const rows = new MyersDiffer().Diff(["one"], []);
        expect(rows[0].Right).to.equal(undefined);
        expect(rows[0].LeftRanges).to.deep.equal([]);
    });
    it('counts the Ranges from the start of the line', () => {
        const l = ["pad", "pad", "abcdef"];
        const r = ["pad", "pad", "abXdef"];
        const rows = new MyersDiffer().Diff(l, r);
        const last = rows[rows.length - 1];
        expect(last.Left).to.equal(2);
        expect(last.LeftRanges).to.deep.equal([{ Start: 2, End: 3 }]);
    });
    it('gives up on lines too long to diff', () => {
        // Past MyersDiffer's own budget, which is 4096 characters a side.
        const left = "a".repeat(5000) + "X";
        const right = "a".repeat(5000) + "Y";
        const rows = new MyersDiffer().Diff([left], [right]);
        expect(rows[0].LeftRanges).to.deep.equal([]);
        expect(rows[0].RightRanges).to.deep.equal([]);
    });
});