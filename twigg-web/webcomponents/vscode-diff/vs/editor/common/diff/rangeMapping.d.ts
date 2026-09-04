import { Range } from '../core/range.js';
import { LineRange } from '../core/ranges/lineRange.js';
import { AbstractText } from '../core/text/abstractText.js';
/**
 * Maps a line range in the original text model to a line range in the modified text model.
 */
export declare class LineRangeMapping {
    /**
     * The line range in the original text model.
     */
    readonly original: LineRange;
    /**
     * The line range in the modified text model.
     */
    readonly modified: LineRange;
    constructor(originalRange: LineRange, modifiedRange: LineRange);
    toString(): string;
    flip(): LineRangeMapping;
    join(other: LineRangeMapping): LineRangeMapping;
    /**
     * This method assumes that the LineRangeMapping describes a valid diff!
     * I.e. if one range is empty, the other range cannot be the entire document.
     * It avoids various problems when the line range points to non-existing line-numbers.
    */
    toRangeMapping(): RangeMapping;
    /**
     * This method assumes that the LineRangeMapping describes a valid diff!
     * I.e. if one range is empty, the other range cannot be the entire document.
     * It avoids various problems when the line range points to non-existing line-numbers.
    */
    toRangeMapping2(original: string[], modified: string[]): RangeMapping;
}
/**
 * Maps a line range in the original text model to a line range in the modified text model.
 * Also contains inner range mappings.
 */
export declare class DetailedLineRangeMapping extends LineRangeMapping {
    /**
     * If inner changes have not been computed, this is set to undefined.
     * Otherwise, it represents the character-level diff in this line range.
     * The original range of each range mapping should be contained in the original line range (same for modified), exceptions are new-lines.
     * Must not be an empty array.
     */
    readonly innerChanges: RangeMapping[] | undefined;
    constructor(originalRange: LineRange, modifiedRange: LineRange, innerChanges: RangeMapping[] | undefined);
    flip(): DetailedLineRangeMapping;
}
/**
 * Maps a range in the original text model to a range in the modified text model.
 */
export declare class RangeMapping {
    static join(rangeMappings: RangeMapping[]): RangeMapping;
    static assertSorted(rangeMappings: RangeMapping[]): void;
    /**
     * The original range.
     */
    readonly originalRange: Range;
    /**
     * The modified range.
     */
    readonly modifiedRange: Range;
    constructor(originalRange: Range, modifiedRange: Range);
    flip(): RangeMapping;
    join(other: RangeMapping): RangeMapping;
}
export declare function lineRangeMappingFromRangeMappings(alignments: readonly RangeMapping[], originalLines: AbstractText, modifiedLines: AbstractText, dontAssertStartLine?: boolean): DetailedLineRangeMapping[];
export declare function getLineRangeMapping(rangeMapping: RangeMapping, originalLines: AbstractText, modifiedLines: AbstractText): DetailedLineRangeMapping;
