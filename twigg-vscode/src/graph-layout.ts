import { Commit } from './tw';

// One row of the graph: a commit and the lines drawn around it.
//
//     lane:  0     1
//            *            2v0   childLanes []     crossingLanes []
//            |     *      2v1   childLanes []     crossingLanes [0]
//            *-----+      1v0   childLanes [0,1]  crossingLanes []
//            |
//            *            0v0   childLanes [0]    crossingLanes []
//
// 2v0 and 2v1 are both children of 1v0, so each of them took a lane and both
// lines end at 1v0. The line from 2v0 to 1v0 crosses the row of 2v1 without
// touching it.
export interface Row {
    commit: Commit;
    // Lane the commit is drawn on. Lane 0 is the leftmost one.
    lane: number;
    // Lanes of the children drawn above, whose lines end at this commit.
    childLanes: number[];
    // Lanes whose line crosses the row without touching the commit.
    crossingLanes: number[];
    // Whether a line leaves the commit downwards. The parent it leads to may
    // be below the last row, in which case the line runs off the bottom.
    hasParent: boolean;
}

// Places the commits on lanes. The commits must be ordered as `tw log --json`
// prints them, which is a commit before the parent it descends from.
//
// A lane is reserved by a commit for its parent, so that the parent is later
// drawn on it. When a commit has several children, each of them reserves a
// lane, and all but the leftmost end at the commit.
export function assignLanes(commits: Commit[]): Row[] {
    // laneWaitsFor[i] is the id of the commit lane i is reserved for, or
    // undefined when the lane is free to be taken.
    const laneWaitsFor: (string | undefined)[] = [];
    const rows: Row[] = [];
    for (const commit of commits) {
        const childLanes: number[] = [];
        var lane = laneWaitsFor.indexOf(commit.Id);
        if (lane === -1) {
            // No child reserved a lane, so this commit starts one.
            lane = takeFreeLane(laneWaitsFor);
        } else {
            childLanes.push(lane);
        }
        // Any other lane reserved for this commit belongs to another of its
        // children, and ends here.
        for (let i = lane + 1; i < laneWaitsFor.length; i++) {
            if (laneWaitsFor[i] === commit.Id) {
                childLanes.push(i);
                laneWaitsFor[i] = undefined;
            }
        }
        if (commit.ParentId === '') {
            laneWaitsFor[lane] = undefined;
        } else {
            laneWaitsFor[lane] = commit.ParentId;
        }
        rows.push({
            commit: commit,
            lane: lane,
            childLanes: childLanes,
            crossingLanes: crossingLanes(laneWaitsFor, lane),
            hasParent: commit.ParentId !== '',
        });
    }
    return rows;
}

// Reserves the leftmost free lane, adding one when they are all taken.
function takeFreeLane(laneWaitsFor: (string | undefined)[]): number {
    const free = laneWaitsFor.indexOf(undefined);
    if (free !== -1) {
        return free;
    }
    laneWaitsFor.push(undefined);
    return laneWaitsFor.length - 1;
}

// The lanes that are still reserved once the row is placed are the ones that
// carry a line into the rows below.
function crossingLanes(
    laneWaitsFor: (string | undefined)[], lane: number): number[] {
    const crossing: number[] = [];
    for (let i = 0; i < laneWaitsFor.length; i++) {
        if (i !== lane && laneWaitsFor[i] !== undefined) {
            crossing.push(i);
        }
    }
    return crossing;
}
