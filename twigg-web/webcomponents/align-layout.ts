
import { AlignedRow, CharRanges, MyersDiffer, VSCodeDiffer } from "./differs";

// AlignmentPoint is a pair of lines that must share a row, as an index into
// each side's lines.
export interface AlignmentPoint {
    Left: number;
    Right: number;
}

// Differ computes aligned rows from a list of lines.
interface Differ {
    Diff(leftLines: string[], rightLines: string[]): AlignedRow[]
}
export type DiffAlgorithm = "myers" | "vscode"
function getDiffer(d: DiffAlgorithm): Differ{
    switch (d) {
        case "vscode":
            return new VSCodeDiffer()
        case "myers":
            return new MyersDiffer()
        default:
            // we need this bc typescript doesnt REEEALLY exist
            console.log("unexpected DiffAlgorithm", d)
            throw new Error("unexpected DiffAlgorithm")
    }
}

// LayoutRows pairs up the lines with a Myers diff. Each point forces its two
// lines onto one row: the sides are cut there and diffed piece by piece.
export function LayoutRows(leftLines: string[], rightLines: string[],
    points: AlignmentPoint[], diff: DiffAlgorithm): AlignedRow[] {
    const differ = getDiffer(diff)
    const alignPoints = filterPoints(leftLines.length, rightLines.length, points)
    if (alignPoints.length == 0){
        return differ.Diff(leftLines, rightLines)
    }
    const rows: AlignedRow[] = [];
    var leftAt = 0;
    var rightAt = 0;
    for (const p of alignPoints) {
        // Get the lines and chop them from the current position to the
        // aling point. Note that the align point itself is not included.
        // I.e. we're getting the lines ABOVE the align point
        const leftTo = p.Left
        const rightTo = p.Right
        const left = leftLines.slice(leftAt, leftTo);
        const right = rightLines.slice(rightAt, rightTo);
        // Get the layout for this "chopped" segment
        const segmentRows = differ.Diff(left, right)
        // The alignment need to be adjusted because layoutRowsSegment
        // thinks the first line of left/right is zero
        pushShifted(segmentRows, leftAt, rightAt, rows)
        // Push the alignment point itself. We just need to compute the line highlights,
        // else aligned rows would never show character ranges higlights.
        const ranges = getRangesOfPinnedRow(differ, leftLines[p.Left], rightLines[p.Right])
        rows.push({ Left: p.Left, Right: p.Right, LeftRanges: ranges.LeftRanges, RightRanges: ranges.RightRanges })
        // Increment current location
        leftAt = leftTo + 1;
        rightAt = rightTo + 1;
    }
    // Lay out what is left after the last point
    const tail = differ.Diff(leftLines.slice(leftAt), rightLines.slice(rightAt))
    pushShifted(tail, leftAt, rightAt, rows)
    return rows;
}

// The pin already decided the two lines share a row, so only the differ's
// opinion on the characters is kept.
function getRangesOfPinnedRow(d: Differ, left: string, right: string): CharRanges {
    for (const r of d.Diff([left], [right])) {
        if (r.Left !== undefined && r.Right !== undefined) {
            return { LeftRanges: r.LeftRanges, RightRanges: r.RightRanges };
        }
    }
    return { LeftRanges: [], RightRanges: [] };
}


// pushShifted appends a segment's rows. The differ counts a segment's indices
// from the start of the segment, so they are moved to where it sits. The
// ranges are offsets within a line, so they are carried over as they are.
function pushShifted(segmentRows: AlignedRow[], leftAt: number, rightAt: number,
    rows: AlignedRow[]) {
    for (const r of segmentRows) {
        rows.push({
            Left: shiftBy(r.Left, leftAt), Right: shiftBy(r.Right, rightAt),
            LeftRanges: r.LeftRanges, RightRanges: r.RightRanges
        });
    }
}
// shiftBy increments a number if defined; else returns undefined
function shiftBy(i: number | undefined, by: number): number | undefined {
    if (i === undefined) {
        return undefined;
    }
    return i + by;
}

// filterPoints sorts the pairs and drops the ones that can't be honoured: a
// pair naming a line that doesn't exist, a second pair on a line already
// taken, and a pair crossing an earlier one, which would need rows to swap
// places. So [{2,2}, {1,5}] keeps only {1,5}.
function filterPoints(nLeft: number, nRight: number,
    points: AlignmentPoint[]): AlignmentPoint[] {
    const inRange = points.filter(function (p) {
        return p.Left >= 0 && p.Left < nLeft && p.Right >= 0 && p.Right < nRight;
    });
    const sorted = [...inRange].sort(function (a, b) {
        return a.Left - b.Left;
    });
    const filtered: AlignmentPoint[] = [];
    var lastLeft = -1;
    var lastRight = -1;
    for (const p of sorted) {
        if (p.Left <= lastLeft || p.Right <= lastRight) {
            continue;
        }
        filtered.push(p);
        lastLeft = p.Left;
        lastRight = p.Right;
    }
    return filtered;
}
