import { Range } from '../range.js';
import { OffsetRange } from './offsetRange.js';
/**
 * A range of lines (1-based).
 */
export declare class LineRange {
    /**
     * The start line number.
     */
    readonly startLineNumber: number;
    /**
     * The end line number (exclusive).
     */
    readonly endLineNumberExclusive: number;
    constructor(startLineNumber: number, endLineNumberExclusive: number);
    /**
     * Indicates if this line range is empty.
     */
    get isEmpty(): boolean;
    /**
     * Moves this line range by the given offset of line numbers.
     */
    delta(offset: number): LineRange;
    /**
     * The number of lines this line range spans.
     */
    get length(): number;
    /**
     * Creates a line range that combines this and the given line range.
     */
    join(other: LineRange): LineRange;
    toString(): string;
    /**
     * The resulting range is empty if the ranges do not intersect, but touch.
     * If the ranges don't even touch, the result is undefined.
     */
    intersect(other: LineRange): LineRange | undefined;
    intersectsOrTouches(other: LineRange): boolean;
    toInclusiveRange(): Range | null;
    /**
     * Converts this 1-based line range to a 0-based offset range (subtracts 1!).
     * @internal
     */
    toOffsetRange(): OffsetRange;
}
export declare class LineRangeSet {
    /**
     * Sorted by start line number.
     * No two line ranges are touching or intersecting.
     */
    private readonly _normalizedRanges;
    constructor(
    /**
     * Sorted by start line number.
     * No two line ranges are touching or intersecting.
     */
    _normalizedRanges?: LineRange[]);
    get ranges(): readonly LineRange[];
    addRange(range: LineRange): void;
    contains(lineNumber: number): boolean;
    /**
     * Subtracts all ranges in this set from `range` and returns the result.
     */
    subtractFrom(range: LineRange): LineRangeSet;
    getIntersection(other: LineRangeSet): LineRangeSet;
    getWithDelta(value: number): LineRangeSet;
}
