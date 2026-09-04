import { OffsetRange } from '../../core/ranges/offsetRange.js';
import { ISequence, SequenceDiff } from './algorithms/diffAlgorithm.js';
import { LineSequence } from './lineSequence.js';
import { LinesSliceCharSequence } from './linesSliceCharSequence.js';
export declare function optimizeSequenceDiffs(sequence1: ISequence, sequence2: ISequence, sequenceDiffs: SequenceDiff[]): SequenceDiff[];
export declare function removeShortMatches(sequence1: ISequence, sequence2: ISequence, sequenceDiffs: SequenceDiff[]): SequenceDiff[];
export declare function extendDiffsToEntireWordIfAppropriate(sequence1: LinesSliceCharSequence, sequence2: LinesSliceCharSequence, sequenceDiffs: SequenceDiff[], findParent: (seq: LinesSliceCharSequence, idx: number) => OffsetRange | undefined, force?: boolean): SequenceDiff[];
export declare function removeVeryShortMatchingLinesBetweenDiffs(sequence1: LineSequence, _sequence2: LineSequence, sequenceDiffs: SequenceDiff[]): SequenceDiff[];
export declare function removeVeryShortMatchingTextBetweenLongDiffs(sequence1: LinesSliceCharSequence, sequence2: LinesSliceCharSequence, sequenceDiffs: SequenceDiff[]): SequenceDiff[];
