// LinesSet represents a set of numbered lines
export interface LinesSet {
    // Raw string of each line
    Raw: string[];
    // Nums[i] is the line number of Raw[i]
    Nums: number[];
}

// LeftAndRightLines is simply a container for a set of lines on each side
export interface LeftAndRightLines {
    Left: LinesSet;
    Right: LinesSet;
}

// NewEmptyLinesSet constructs a valid empty LinesSet
export function NewEmptyLinesSet(): LinesSet {
    return { Raw: [], Nums: [] };
}

// ParseUnifiedDiff parses a single-hunk unified diff into LeftAndRightLines.
// Throws if the diff has more than one hunk.
export function ParseUnifiedDiff(diff: string): LeftAndRightLines {
    const lines = diff.split("\n");
    // Drop the very first line ("diff ...")
    if (lines.length > 0 && lines[0].startsWith("diff")) {
        lines.shift();
    }

    const left = NewEmptyLinesSet();
    const right = NewEmptyLinesSet();
    var leftNum = 0;
    var rightNum = 0;
    var sawHunkIndicator = false;

    for (const line of lines) {
        // Drop --- and +++ headers
        if (line.startsWith("---") || line.startsWith("+++")) {
            continue;
        }

        // When a huck header is found, extract the first line nums.
        // Throw if already saw a hunk.
        if (line.startsWith("@@")) {
            if (sawHunkIndicator) {
                throw "got diff with multiple hunks";
            }
            sawHunkIndicator = true;
            const m = /@@ -(\d+),?\d* \+(\d+),?\d* @@/.exec(line);
            if (m) {
                leftNum = parseInt(m[1], 10);
                rightNum = parseInt(m[2], 10);
            }
            continue;
        }

        if (line === "\\ No newline at end of file") {
            continue;
        }
        if (line.length == 0) {
            continue;
        }

        if (line.startsWith("-")) {
            pushLine(left, line.substring(1), leftNum);
            leftNum++;
            continue;
        }

        if (line.startsWith("+")) {
            pushLine(right, line.substring(1), rightNum);
            rightNum++;
            continue;
        }

        if (line.startsWith(" ")) {
            const text = line.substring(1);
            pushLine(left, text, leftNum);
            leftNum++;
            pushLine(right, text, rightNum);
            rightNum++;
            continue;
        }

        console.log("unexpected line in unified diff: `", line, "`");
    }

    return { Left: left, Right: right };
}

function pushLine(side: LinesSet, raw: string, num: number) {
    side.Raw.push(raw);
    side.Nums.push(num);
}
