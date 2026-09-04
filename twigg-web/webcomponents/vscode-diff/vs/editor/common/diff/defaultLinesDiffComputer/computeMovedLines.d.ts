import { DetailedLineRangeMapping, LineRangeMapping } from '../rangeMapping.js';
import { ITimeout } from './algorithms/diffAlgorithm.js';
export declare function computeMovedLines(changes: DetailedLineRangeMapping[], originalLines: string[], modifiedLines: string[], hashedOriginalLines: number[], hashedModifiedLines: number[], timeout: ITimeout): LineRangeMapping[];
