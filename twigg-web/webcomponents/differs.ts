import { diff as myersDiff } from './fast-myers-diff';
import { DefaultLinesDiffComputer } from './vscode-diff';

// A character range [start, end) within a line, in UTF-16 code units.
export interface CharRange {
    Start: number;
    End: number;
}

// A pair of lists of ranges. Used to represent highlighted ranges in a line pair.
export interface CharRanges {
    LeftRanges: CharRange[]
    RightRanges: CharRange[]
}

// AlignedRow is one row, as indices into each side's lines. An index is
// undefined when that side has no line on the row.
// LeftRanges and RightRanges indicate the highlighted chunks of each side. 
// Either side can be empty.
export interface AlignedRow {
    Left: number | undefined;
    Right: number | undefined;
    LeftRanges: CharRange[];
    RightRanges: CharRange[]
}

// MyersDiffer pairs the lines with a Myers diff over the lines, then runs a
// second Myers diff over the characters of each paired row.
export class MyersDiffer {
    public Diff(leftLines: string[], rightLines: string[]): AlignedRow[] {
        // First get only the rows
        const rows = this.alignRows(leftLines, rightLines);
        // Then, compute the highlights for each row
        for (const r of rows) {
            // If there's nothing on one side, dont highlight anything
            if (r.Left === undefined || r.Right === undefined) {
                r.LeftRanges = []
                r.RightRanges = []
                continue;
            }
            // If the lines are equal, don't highlight anything
            const left = leftLines[r.Left];
            const right = rightLines[r.Right];
            if (left === right) {
                r.LeftRanges = []
                r.RightRanges = []
                continue;
            }
            // Else, compute the highlighted ranges again with myers
            const chunks = this.getLineHighlightRanges(left, right);
            r.LeftRanges = chunks.LeftRanges
            r.RightRanges = chunks.RightRanges
        }
        return rows;
    }
    // alignRows creates alined rows with only Left/Right indices.
    // LeftRanges and RightRanges are still not populated
    private alignRows(leftLines: string[], rightLines: string[]): AlignedRow[] {
        const rows: AlignedRow[] = [];
        var leftAt = 0;
        var rightAt = 0;
        for (const [leftStart, leftEnd, rightStart, rightEnd] of myersDiff(leftLines, rightLines)) {
            // Lines before a change are equal on both sides, so they pair up
            // and nothing is highlighted on either side
            while (leftAt < leftStart) {
                rows.push({ Left: leftAt, Right: rightAt, LeftRanges: [], RightRanges: [] });
                leftAt++;
                rightAt++;
            }
            this.pushChange(leftStart, leftEnd, rightStart, rightEnd, rows);
            leftAt = leftEnd;
            rightAt = rightEnd;
        }
        // Push trailing paired lines
        while (leftAt < leftLines.length) {
            rows.push({ Left: leftAt, Right: rightAt, LeftRanges: [], RightRanges: [] });
            leftAt++;
            rightAt++;
        }
        return rows;
    }
    // pushChange pushes missing aligned rows into rows
    private pushChange(leftFrom: number, leftTo: number,
        rightFrom: number, rightTo: number, rows: AlignedRow[]) {
        const nPaired = Math.min(leftTo - leftFrom, rightTo - rightFrom);
        for (var i = 0; i < nPaired; i++) {
            rows.push({ Left: leftFrom + i, Right: rightFrom + i, LeftRanges: [], RightRanges: [] });
        }
        for (var l = leftFrom + nPaired; l < leftTo; l++) {
            rows.push({ Left: l, Right: undefined, LeftRanges: [], RightRanges: [] });
        }
        for (var r = rightFrom + nPaired; r < rightTo; r++) {
            rows.push({ Left: undefined, Right: r, LeftRanges: [], RightRanges: [] });
        }
    }
    // Give up on lines longer than this bc Myers costs O((N+M)*D)
    private static maxDiffLength = 4096;
    // Computes the list of changes between the left (old) and right (new) version
    // of a line with a Myers diff. Returns [] if any input is too long.
    // Known limitation: offsets are UTF-16 code units, so a boundary can fall
    // inside a multi-unit character (e.g. between two similar emoji) and make it
    // render as replacement characters.
    private getLineHighlightRanges(left: string, right: string): CharRanges {
        if (left.length > MyersDiffer.maxDiffLength || right.length > MyersDiffer.maxDiffLength) {
            return { LeftRanges: [], RightRanges: [] };
        }
        const ranges: CharRanges = { LeftRanges: [], RightRanges: [] };
        for (const [leftStart, leftEnd, rightStart, rightEnd] of myersDiff(left, right)) {
            ranges.LeftRanges.push({ Start: leftStart, End: leftEnd });
            ranges.RightRanges.push({ Start: rightStart, End: rightEnd });
        }
        return ranges;
    }
}

// VSCodeDiffer is WIP. It will (when done) compute diffs using the vendored
// VSCode diff algorithm.
export class VSCodeDiffer {
    constructor(){}
    public Diff(leftLines: string[], rightLines: string[]): AlignedRow[] {
        // WIP - to setup the plumbing for now we just use myers diff as a mock
        const m = new MyersDiffer()
        return m.Diff(leftLines, rightLines)
    }
}