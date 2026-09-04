import { Range } from '../range.js';
import { TextLength } from '../text/textLength.js';
export declare abstract class AbstractText {
    abstract getValueOfRange(range: Range): string;
    abstract readonly length: TextLength;
    getLineLength(lineNumber: number): number;
}
declare class LineBasedText extends AbstractText {
    private readonly _getLineContent;
    private readonly _lineCount;
    constructor(_getLineContent: (lineNumber: number) => string, _lineCount: number);
    getValueOfRange(range: Range): string;
    getLineLength(lineNumber: number): number;
    get length(): TextLength;
}
export declare class ArrayText extends LineBasedText {
    constructor(lines: string[]);
}
export {};
