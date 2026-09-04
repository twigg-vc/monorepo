import { expect } from '@open-wc/testing';
import { AlignmentPoint, LayoutRows } from './align-layout';
import { AlignedRow, MyersDiffer } from './differs';
// rowsToStr renders the rows as "left|right", with a dot for a missing side.
function rowsToStr(left: string[], right: string[], rows: AlignedRow[]): string[] {
    return rows.map(r => lineAt(left, r.Left) + "|" + lineAt(right, r.Right));
}
// lineAt is a helper to rowsToStr for rearing the i-th line or a placeholder
function lineAt(lines: string[], i: number | undefined): string {
    if (i === undefined) {
        return ".";
    }
    return lines[i];
}
// checkLayoutRows is a check helper that asserts left/right expected layout.
// see rowsToStr for the layour string format.
function checkLayoutRows(left: string[], right: string[], expectedRowsStr: string[]) {
    checkPinnedLayoutRows(left, right, [], expectedRowsStr);
}
// checkPinnedLayoutRows is checkLayoutRows for a layout with alignment points.
function checkPinnedLayoutRows(left: string[], right: string[],
    points: AlignmentPoint[], expectedRowsStr: string[]) {
    const gotRowsStr = rowsToStr(left, right, LayoutRows(left, right, points, new MyersDiffer()));
    expect(gotRowsStr).to.deep.equal(expectedRowsStr);
}
describe('LayoutRows', () => {
    it('handles both empty', () => {
        expect(LayoutRows([], [], [], new MyersDiffer())).to.deep.equal([]);
    });

    it('pairs up the lines of an unchanged file', () => {
        const a = ["one", "two", "three"];
        checkLayoutRows(a, a, ["one|one", "two|two", "three|three"])
    });

    it('puts a rewritten line facing the one it replaced', () => {
        const l = ["one", "two", "three"];
        const r = ["one", "TWO", "three"];
        checkLayoutRows(l, r, ["one|one", "two|TWO", "three|three"]);
    });

    it('gives an inserted line a row of its own', () => {
        const l = ["one", "three"];
        const r = ["one", "two", "three"];
        checkLayoutRows(l, r, ["one|one", ".|two", "three|three"]);
    });

    it('gives a removed line a row of its own', () => {
        const l = ["one", "two", "three"];
        const r = ["one", "three"];
        checkLayoutRows(l, r, ["one|one", "two|.", "three|three"]);
    });

    it('faces off as many lines as it can in an uneven change', () => {
        const l = ["a", "x1", "x2", "x3", "b"];
        const r = ["a", "y1", "b"];
        checkLayoutRows(l, r, ["a|a", "x1|y1", "x2|.", "x3|.", "b|b"]);
        checkLayoutRows(r, l, ["a|a", "y1|x1", ".|x2", ".|x3", "b|b"]);
    });

    it('handles one side being empty', () => {
        checkLayoutRows([], ["one"], [".|one"])
        checkLayoutRows(["one"], [], ["one|."])
    });

    it('puts a pinned pair on the same row', () => {
        // "moved" sits at a different place on each side, so nothing else can
        // stay paired once it is pinned.
        const l = ["moved", "a", "b"];
        const r = ["a", "b", "moved"];
        checkPinnedLayoutRows(l, r, [{ Left: 0, Right: 2 }],
            [".|a", ".|b", "moved|moved", "a|.", "b|."]);
    });

    it('honours several pins at once', () => {
        const l = ["a", "gap", "b"];
        const r = ["a", "b"];
        checkPinnedLayoutRows(l, r, [{ Left: 0, Right: 0 }, { Left: 2, Right: 1 }],
            ["a|a", "gap|.", "b|b"]);
    });

    it('pads the shorter side between two pins', () => {
        const l = ["a", "b"];
        const r = ["a", "x", "y", "b"];
        checkPinnedLayoutRows(l, r, [{ Left: 0, Right: 0 }, { Left: 1, Right: 3 }],
            ["a|a", ".|x", ".|y", "b|b"]);
    });

    it('ignores a pin naming a line that is not there', () => {
        const l = ["one", "two"];
        const r = ["one", "TWO"];
        checkPinnedLayoutRows(l, r,
            [{ Left: 5, Right: 0 }, { Left: 0, Right: 5 }, { Left: -1, Right: -1 }],
            ["one|one", "two|TWO"]);
    });

    it('ignores a pin that crosses an earlier one', () => {
        const l = ["a", "b"];
        const r = ["a", "b"];
        checkPinnedLayoutRows(l, r, [{ Left: 0, Right: 1 }, { Left: 1, Right: 0 }],
            [".|a", "a|b", "b|."]);
    });
});