/**
 * A position in the editor. This interface is suitable for serialization.
 */
export interface IPosition {
    /**
     * line number (starts at 1)
     */
    readonly lineNumber: number;
    /**
     * column (the first character in a line is between column 1 and column 2)
     */
    readonly column: number;
}
/**
 * A position in the editor.
 */
export declare class Position {
    /**
     * line number (starts at 1)
     */
    readonly lineNumber: number;
    /**
     * column (the first character in a line is between column 1 and column 2)
     */
    readonly column: number;
    constructor(lineNumber: number, column: number);
    /**
     * Test if this position is before other position.
     * If the two positions are equal, the result will be false.
     */
    isBefore(other: IPosition): boolean;
    /**
     * Test if position `a` is before position `b`.
     * If the two positions are equal, the result will be false.
     */
    static isBefore(a: IPosition, b: IPosition): boolean;
    /**
     * Test if this position is before other position.
     * If the two positions are equal, the result will be true.
     */
    isBeforeOrEqual(other: IPosition): boolean;
    /**
     * Test if position `a` is before position `b`.
     * If the two positions are equal, the result will be true.
     */
    static isBeforeOrEqual(a: IPosition, b: IPosition): boolean;
    /**
     * Convert to a human-readable representation.
     */
    toString(): string;
}
