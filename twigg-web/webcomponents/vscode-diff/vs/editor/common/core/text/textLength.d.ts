/**
 * Represents a non-negative length of text in terms of line and column count.
*/
export declare class TextLength {
    readonly lineCount: number;
    readonly columnCount: number;
    constructor(lineCount: number, columnCount: number);
}
