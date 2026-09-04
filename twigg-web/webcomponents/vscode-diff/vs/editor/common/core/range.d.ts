import { IPosition, Position } from './position.js';
/**
 * A range in the editor. This interface is suitable for serialization.
 */
interface IRange {
    /**
     * Line number on which the range starts (starts at 1).
     */
    readonly startLineNumber: number;
    /**
     * Column on which the range starts in line `startLineNumber` (starts at 1).
     */
    readonly startColumn: number;
    /**
     * Line number on which the range ends.
     */
    readonly endLineNumber: number;
    /**
     * Column on which the range ends in line `endLineNumber`.
     */
    readonly endColumn: number;
}
/**
 * A range in the editor. (startLineNumber,startColumn) is <= (endLineNumber,endColumn)
 */
export declare class Range {
    /**
     * Line number on which the range starts (starts at 1).
     */
    readonly startLineNumber: number;
    /**
     * Column on which the range starts in line `startLineNumber` (starts at 1).
     */
    readonly startColumn: number;
    /**
     * Line number on which the range ends.
     */
    readonly endLineNumber: number;
    /**
     * Column on which the range ends in line `endLineNumber`.
     */
    readonly endColumn: number;
    constructor(startLineNumber: number, startColumn: number, endLineNumber: number, endColumn: number);
    /**
     * A reunion of the two ranges.
     * The smallest position will be used as the start point, and the largest one as the end point.
     */
    plusRange(range: IRange): Range;
    /**
     * A reunion of the two ranges.
     * The smallest position will be used as the start point, and the largest one as the end point.
     */
    static plusRange(a: IRange, b: IRange): Range;
    /**
     * Return the end position (which will be after or equal to the start position)
     */
    getEndPosition(): Position;
    /**
     * Return the end position (which will be after or equal to the start position)
     */
    static getEndPosition(range: IRange): Position;
    /**
     * Return the start position (which will be before or equal to the end position)
     */
    getStartPosition(): Position;
    /**
     * Return the start position (which will be before or equal to the end position)
     */
    static getStartPosition(range: IRange): Position;
    static fromPositions(start: IPosition, end?: IPosition): Range;
}
export {};
