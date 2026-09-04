var __create = Object.create;
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __getProtoOf = Object.getPrototypeOf;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __commonJS = (cb, mod) => function __require() {
  try {
    return mod || (0, cb[__getOwnPropNames(cb)[0]])((mod = { exports: {} }).exports, mod), mod.exports;
  } catch (e) {
    throw mod = 0, e;
  }
};
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
  }
  return to;
};
var __toESM = (mod, isNodeMode, target) => (target = mod != null ? __create(__getProtoOf(mod)) : {}, __copyProps(
  // If the importer is in node compatibility mode or this is not an ESM
  // file that has been converted to a CommonJS file using a Babel-
  // compatible transform (i.e. "__esModule" has not been set), then set
  // "default" to the CommonJS "module.exports" for node compatibility.
  isNodeMode || !mod || !mod.__esModule ? __defProp(target, "default", { value: mod, enumerable: true }) : target,
  mod
));

// package/dist/vs/base/common/hash.js
var require_hash = __commonJS({
  "package/dist/vs/base/common/hash.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.stringHash = stringHash;
    function numberHash(val, initialHashVal) {
      return (initialHashVal << 5) - initialHashVal + val | 0;
    }
    function stringHash(s, hashVal) {
      hashVal = numberHash(149417, hashVal);
      for (let i = 0, length = s.length; i < length; i++) {
        hashVal = numberHash(s.charCodeAt(i), hashVal);
      }
      return hashVal;
    }
  }
});

// package/dist/vs/base/common/diff/diffChange.js
var require_diffChange = __commonJS({
  "package/dist/vs/base/common/diff/diffChange.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.DiffChange = void 0;
    var DiffChange2 = class {
      /**
       * The position of the first element in the original sequence which
       * this change affects.
       */
      originalStart;
      /**
       * The number of elements from the original sequence which were
       * affected.
       */
      originalLength;
      /**
       * The position of the first element in the modified sequence which
       * this change affects.
       */
      modifiedStart;
      /**
       * The number of elements from the modified sequence which were
       * affected (added).
       */
      modifiedLength;
      /**
       * Constructs a new DiffChange with the given sequence information
       * and content.
       */
      constructor(originalStart, originalLength, modifiedStart, modifiedLength) {
        this.originalStart = originalStart;
        this.originalLength = originalLength;
        this.modifiedStart = modifiedStart;
        this.modifiedLength = modifiedLength;
      }
      /**
       * The end point (exclusive) of the change in the original sequence.
       */
      getOriginalEnd() {
        return this.originalStart + this.originalLength;
      }
      /**
       * The end point (exclusive) of the change in the modified sequence.
       */
      getModifiedEnd() {
        return this.modifiedStart + this.modifiedLength;
      }
    };
    exports.DiffChange = DiffChange2;
  }
});

// package/dist/vs/base/common/diff/diff.js
var require_diff = __commonJS({
  "package/dist/vs/base/common/diff/diff.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.LcsDiff = exports.StringDiffSequence = void 0;
    exports.stringDiff = stringDiff2;
    exports.computeLevenshteinDistance = computeLevenshteinDistance2;
    var hash_js_1 = require_hash();
    var diffChange_js_1 = require_diffChange();
    var StringDiffSequence2 = class {
      source;
      constructor(source) {
        this.source = source;
      }
      getElements() {
        const source = this.source;
        const characters = new Int32Array(source.length);
        for (let i = 0, len = source.length; i < len; i++) {
          characters[i] = source.charCodeAt(i);
        }
        return characters;
      }
    };
    exports.StringDiffSequence = StringDiffSequence2;
    function stringDiff2(original, modified, pretty) {
      return new LcsDiff2(new StringDiffSequence2(original), new StringDiffSequence2(modified)).ComputeDiff(pretty).changes;
    }
    var Debug = class {
      static Assert(condition, message) {
        if (!condition) {
          throw new Error(message);
        }
      }
    };
    var MyArray = class {
      /**
       * Copies a range of elements from an Array starting at the specified source index and pastes
       * them to another Array starting at the specified destination index. The length and the indexes
       * are specified as 64-bit integers.
       * sourceArray:
       *		The Array that contains the data to copy.
       * sourceIndex:
       *		A 64-bit integer that represents the index in the sourceArray at which copying begins.
       * destinationArray:
       *		The Array that receives the data.
       * destinationIndex:
       *		A 64-bit integer that represents the index in the destinationArray at which storing begins.
       * length:
       *		A 64-bit integer that represents the number of elements to copy.
       */
      static Copy(sourceArray, sourceIndex, destinationArray, destinationIndex, length) {
        for (let i = 0; i < length; i++) {
          destinationArray[destinationIndex + i] = sourceArray[sourceIndex + i];
        }
      }
      static Copy2(sourceArray, sourceIndex, destinationArray, destinationIndex, length) {
        for (let i = 0; i < length; i++) {
          destinationArray[destinationIndex + i] = sourceArray[sourceIndex + i];
        }
      }
    };
    var DiffChangeHelper = class {
      m_changes;
      m_originalStart;
      m_modifiedStart;
      m_originalCount;
      m_modifiedCount;
      /**
       * Constructs a new DiffChangeHelper for the given DiffSequences.
       */
      constructor() {
        this.m_changes = [];
        this.m_originalStart = 1073741824;
        this.m_modifiedStart = 1073741824;
        this.m_originalCount = 0;
        this.m_modifiedCount = 0;
      }
      /**
       * Marks the beginning of the next change in the set of differences.
       */
      MarkNextChange() {
        if (this.m_originalCount > 0 || this.m_modifiedCount > 0) {
          this.m_changes.push(new diffChange_js_1.DiffChange(this.m_originalStart, this.m_originalCount, this.m_modifiedStart, this.m_modifiedCount));
        }
        this.m_originalCount = 0;
        this.m_modifiedCount = 0;
        this.m_originalStart = 1073741824;
        this.m_modifiedStart = 1073741824;
      }
      /**
       * Adds the original element at the given position to the elements
       * affected by the current change. The modified index gives context
       * to the change position with respect to the original sequence.
       * @param originalIndex The index of the original element to add.
       * @param modifiedIndex The index of the modified element that provides corresponding position in the modified sequence.
       */
      AddOriginalElement(originalIndex, modifiedIndex) {
        this.m_originalStart = Math.min(this.m_originalStart, originalIndex);
        this.m_modifiedStart = Math.min(this.m_modifiedStart, modifiedIndex);
        this.m_originalCount++;
      }
      /**
       * Adds the modified element at the given position to the elements
       * affected by the current change. The original index gives context
       * to the change position with respect to the modified sequence.
       * @param originalIndex The index of the original element that provides corresponding position in the original sequence.
       * @param modifiedIndex The index of the modified element to add.
       */
      AddModifiedElement(originalIndex, modifiedIndex) {
        this.m_originalStart = Math.min(this.m_originalStart, originalIndex);
        this.m_modifiedStart = Math.min(this.m_modifiedStart, modifiedIndex);
        this.m_modifiedCount++;
      }
      /**
       * Retrieves all of the changes marked by the class.
       */
      getChanges() {
        if (this.m_originalCount > 0 || this.m_modifiedCount > 0) {
          this.MarkNextChange();
        }
        return this.m_changes;
      }
      /**
       * Retrieves all of the changes marked by the class in the reverse order
       */
      getReverseChanges() {
        if (this.m_originalCount > 0 || this.m_modifiedCount > 0) {
          this.MarkNextChange();
        }
        this.m_changes.reverse();
        return this.m_changes;
      }
    };
    var LcsDiff2 = class _LcsDiff {
      ContinueProcessingPredicate;
      _originalSequence;
      _modifiedSequence;
      _hasStrings;
      _originalStringElements;
      _originalElementsOrHash;
      _modifiedStringElements;
      _modifiedElementsOrHash;
      m_forwardHistory;
      m_reverseHistory;
      /**
       * Constructs the DiffFinder
       */
      constructor(originalSequence, modifiedSequence, continueProcessingPredicate = null) {
        this.ContinueProcessingPredicate = continueProcessingPredicate;
        this._originalSequence = originalSequence;
        this._modifiedSequence = modifiedSequence;
        const [originalStringElements, originalElementsOrHash, originalHasStrings] = _LcsDiff._getElements(originalSequence);
        const [modifiedStringElements, modifiedElementsOrHash, modifiedHasStrings] = _LcsDiff._getElements(modifiedSequence);
        this._hasStrings = originalHasStrings && modifiedHasStrings;
        this._originalStringElements = originalStringElements;
        this._originalElementsOrHash = originalElementsOrHash;
        this._modifiedStringElements = modifiedStringElements;
        this._modifiedElementsOrHash = modifiedElementsOrHash;
        this.m_forwardHistory = [];
        this.m_reverseHistory = [];
      }
      static _isStringArray(arr) {
        return arr.length > 0 && typeof arr[0] === "string";
      }
      static _getElements(sequence) {
        const elements = sequence.getElements();
        if (_LcsDiff._isStringArray(elements)) {
          const hashes = new Int32Array(elements.length);
          for (let i = 0, len = elements.length; i < len; i++) {
            hashes[i] = (0, hash_js_1.stringHash)(elements[i], 0);
          }
          return [elements, hashes, true];
        }
        if (elements instanceof Int32Array) {
          return [[], elements, false];
        }
        return [[], new Int32Array(elements), false];
      }
      ElementsAreEqual(originalIndex, newIndex) {
        if (this._originalElementsOrHash[originalIndex] !== this._modifiedElementsOrHash[newIndex]) {
          return false;
        }
        return this._hasStrings ? this._originalStringElements[originalIndex] === this._modifiedStringElements[newIndex] : true;
      }
      ElementsAreStrictEqual(originalIndex, newIndex) {
        if (!this.ElementsAreEqual(originalIndex, newIndex)) {
          return false;
        }
        const originalElement = _LcsDiff._getStrictElement(this._originalSequence, originalIndex);
        const modifiedElement = _LcsDiff._getStrictElement(this._modifiedSequence, newIndex);
        return originalElement === modifiedElement;
      }
      static _getStrictElement(sequence, index) {
        if (typeof sequence.getStrictElement === "function") {
          return sequence.getStrictElement(index);
        }
        return null;
      }
      OriginalElementsAreEqual(index1, index2) {
        if (this._originalElementsOrHash[index1] !== this._originalElementsOrHash[index2]) {
          return false;
        }
        return this._hasStrings ? this._originalStringElements[index1] === this._originalStringElements[index2] : true;
      }
      ModifiedElementsAreEqual(index1, index2) {
        if (this._modifiedElementsOrHash[index1] !== this._modifiedElementsOrHash[index2]) {
          return false;
        }
        return this._hasStrings ? this._modifiedStringElements[index1] === this._modifiedStringElements[index2] : true;
      }
      ComputeDiff(pretty) {
        return this._ComputeDiff(0, this._originalElementsOrHash.length - 1, 0, this._modifiedElementsOrHash.length - 1, pretty);
      }
      /**
       * Computes the differences between the original and modified input
       * sequences on the bounded range.
       * @returns An array of the differences between the two input sequences.
       */
      _ComputeDiff(originalStart, originalEnd, modifiedStart, modifiedEnd, pretty) {
        const quitEarlyArr = [false];
        let changes = this.ComputeDiffRecursive(originalStart, originalEnd, modifiedStart, modifiedEnd, quitEarlyArr);
        if (pretty) {
          changes = this.PrettifyChanges(changes);
        }
        return {
          quitEarly: quitEarlyArr[0],
          changes
        };
      }
      /**
       * Private helper method which computes the differences on the bounded range
       * recursively.
       * @returns An array of the differences between the two input sequences.
       */
      ComputeDiffRecursive(originalStart, originalEnd, modifiedStart, modifiedEnd, quitEarlyArr) {
        quitEarlyArr[0] = false;
        while (originalStart <= originalEnd && modifiedStart <= modifiedEnd && this.ElementsAreEqual(originalStart, modifiedStart)) {
          originalStart++;
          modifiedStart++;
        }
        while (originalEnd >= originalStart && modifiedEnd >= modifiedStart && this.ElementsAreEqual(originalEnd, modifiedEnd)) {
          originalEnd--;
          modifiedEnd--;
        }
        if (originalStart > originalEnd || modifiedStart > modifiedEnd) {
          let changes;
          if (modifiedStart <= modifiedEnd) {
            Debug.Assert(originalStart === originalEnd + 1, "originalStart should only be one more than originalEnd");
            changes = [
              new diffChange_js_1.DiffChange(originalStart, 0, modifiedStart, modifiedEnd - modifiedStart + 1)
            ];
          } else if (originalStart <= originalEnd) {
            Debug.Assert(modifiedStart === modifiedEnd + 1, "modifiedStart should only be one more than modifiedEnd");
            changes = [
              new diffChange_js_1.DiffChange(originalStart, originalEnd - originalStart + 1, modifiedStart, 0)
            ];
          } else {
            Debug.Assert(originalStart === originalEnd + 1, "originalStart should only be one more than originalEnd");
            Debug.Assert(modifiedStart === modifiedEnd + 1, "modifiedStart should only be one more than modifiedEnd");
            changes = [];
          }
          return changes;
        }
        const midOriginalArr = [0];
        const midModifiedArr = [0];
        const result = this.ComputeRecursionPoint(originalStart, originalEnd, modifiedStart, modifiedEnd, midOriginalArr, midModifiedArr, quitEarlyArr);
        const midOriginal = midOriginalArr[0];
        const midModified = midModifiedArr[0];
        if (result !== null) {
          return result;
        } else if (!quitEarlyArr[0]) {
          const leftChanges = this.ComputeDiffRecursive(originalStart, midOriginal, modifiedStart, midModified, quitEarlyArr);
          let rightChanges = [];
          if (!quitEarlyArr[0]) {
            rightChanges = this.ComputeDiffRecursive(midOriginal + 1, originalEnd, midModified + 1, modifiedEnd, quitEarlyArr);
          } else {
            rightChanges = [
              new diffChange_js_1.DiffChange(midOriginal + 1, originalEnd - (midOriginal + 1) + 1, midModified + 1, modifiedEnd - (midModified + 1) + 1)
            ];
          }
          return this.ConcatenateChanges(leftChanges, rightChanges);
        }
        return [
          new diffChange_js_1.DiffChange(originalStart, originalEnd - originalStart + 1, modifiedStart, modifiedEnd - modifiedStart + 1)
        ];
      }
      WALKTRACE(diagonalForwardBase, diagonalForwardStart, diagonalForwardEnd, diagonalForwardOffset, diagonalReverseBase, diagonalReverseStart, diagonalReverseEnd, diagonalReverseOffset, forwardPoints, reversePoints, originalIndex, originalEnd, midOriginalArr, modifiedIndex, modifiedEnd, midModifiedArr, deltaIsEven, quitEarlyArr) {
        let forwardChanges = null;
        let reverseChanges = null;
        let changeHelper = new DiffChangeHelper();
        let diagonalMin = diagonalForwardStart;
        let diagonalMax = diagonalForwardEnd;
        let diagonalRelative = midOriginalArr[0] - midModifiedArr[0] - diagonalForwardOffset;
        let lastOriginalIndex = -1073741824;
        let historyIndex = this.m_forwardHistory.length - 1;
        do {
          const diagonal = diagonalRelative + diagonalForwardBase;
          if (diagonal === diagonalMin || diagonal < diagonalMax && forwardPoints[diagonal - 1] < forwardPoints[diagonal + 1]) {
            originalIndex = forwardPoints[diagonal + 1];
            modifiedIndex = originalIndex - diagonalRelative - diagonalForwardOffset;
            if (originalIndex < lastOriginalIndex) {
              changeHelper.MarkNextChange();
            }
            lastOriginalIndex = originalIndex;
            changeHelper.AddModifiedElement(originalIndex + 1, modifiedIndex);
            diagonalRelative = diagonal + 1 - diagonalForwardBase;
          } else {
            originalIndex = forwardPoints[diagonal - 1] + 1;
            modifiedIndex = originalIndex - diagonalRelative - diagonalForwardOffset;
            if (originalIndex < lastOriginalIndex) {
              changeHelper.MarkNextChange();
            }
            lastOriginalIndex = originalIndex - 1;
            changeHelper.AddOriginalElement(originalIndex, modifiedIndex + 1);
            diagonalRelative = diagonal - 1 - diagonalForwardBase;
          }
          if (historyIndex >= 0) {
            forwardPoints = this.m_forwardHistory[historyIndex];
            diagonalForwardBase = forwardPoints[0];
            diagonalMin = 1;
            diagonalMax = forwardPoints.length - 1;
          }
        } while (--historyIndex >= -1);
        forwardChanges = changeHelper.getReverseChanges();
        if (quitEarlyArr[0]) {
          let originalStartPoint = midOriginalArr[0] + 1;
          let modifiedStartPoint = midModifiedArr[0] + 1;
          if (forwardChanges !== null && forwardChanges.length > 0) {
            const lastForwardChange = forwardChanges[forwardChanges.length - 1];
            originalStartPoint = Math.max(originalStartPoint, lastForwardChange.getOriginalEnd());
            modifiedStartPoint = Math.max(modifiedStartPoint, lastForwardChange.getModifiedEnd());
          }
          reverseChanges = [
            new diffChange_js_1.DiffChange(originalStartPoint, originalEnd - originalStartPoint + 1, modifiedStartPoint, modifiedEnd - modifiedStartPoint + 1)
          ];
        } else {
          changeHelper = new DiffChangeHelper();
          diagonalMin = diagonalReverseStart;
          diagonalMax = diagonalReverseEnd;
          diagonalRelative = midOriginalArr[0] - midModifiedArr[0] - diagonalReverseOffset;
          lastOriginalIndex = 1073741824;
          historyIndex = deltaIsEven ? this.m_reverseHistory.length - 1 : this.m_reverseHistory.length - 2;
          do {
            const diagonal = diagonalRelative + diagonalReverseBase;
            if (diagonal === diagonalMin || diagonal < diagonalMax && reversePoints[diagonal - 1] >= reversePoints[diagonal + 1]) {
              originalIndex = reversePoints[diagonal + 1] - 1;
              modifiedIndex = originalIndex - diagonalRelative - diagonalReverseOffset;
              if (originalIndex > lastOriginalIndex) {
                changeHelper.MarkNextChange();
              }
              lastOriginalIndex = originalIndex + 1;
              changeHelper.AddOriginalElement(originalIndex + 1, modifiedIndex + 1);
              diagonalRelative = diagonal + 1 - diagonalReverseBase;
            } else {
              originalIndex = reversePoints[diagonal - 1];
              modifiedIndex = originalIndex - diagonalRelative - diagonalReverseOffset;
              if (originalIndex > lastOriginalIndex) {
                changeHelper.MarkNextChange();
              }
              lastOriginalIndex = originalIndex;
              changeHelper.AddModifiedElement(originalIndex + 1, modifiedIndex + 1);
              diagonalRelative = diagonal - 1 - diagonalReverseBase;
            }
            if (historyIndex >= 0) {
              reversePoints = this.m_reverseHistory[historyIndex];
              diagonalReverseBase = reversePoints[0];
              diagonalMin = 1;
              diagonalMax = reversePoints.length - 1;
            }
          } while (--historyIndex >= -1);
          reverseChanges = changeHelper.getChanges();
        }
        return this.ConcatenateChanges(forwardChanges, reverseChanges);
      }
      /**
       * Given the range to compute the diff on, this method finds the point:
       * (midOriginal, midModified)
       * that exists in the middle of the LCS of the two sequences and
       * is the point at which the LCS problem may be broken down recursively.
       * This method will try to keep the LCS trace in memory. If the LCS recursion
       * point is calculated and the full trace is available in memory, then this method
       * will return the change list.
       * @param originalStart The start bound of the original sequence range
       * @param originalEnd The end bound of the original sequence range
       * @param modifiedStart The start bound of the modified sequence range
       * @param modifiedEnd The end bound of the modified sequence range
       * @param midOriginal The middle point of the original sequence range
       * @param midModified The middle point of the modified sequence range
       * @returns The diff changes, if available, otherwise null
       */
      ComputeRecursionPoint(originalStart, originalEnd, modifiedStart, modifiedEnd, midOriginalArr, midModifiedArr, quitEarlyArr) {
        let originalIndex = 0, modifiedIndex = 0;
        let diagonalForwardStart = 0, diagonalForwardEnd = 0;
        let diagonalReverseStart = 0, diagonalReverseEnd = 0;
        originalStart--;
        modifiedStart--;
        midOriginalArr[0] = 0;
        midModifiedArr[0] = 0;
        this.m_forwardHistory = [];
        this.m_reverseHistory = [];
        const maxDifferences = originalEnd - originalStart + (modifiedEnd - modifiedStart);
        const numDiagonals = maxDifferences + 1;
        const forwardPoints = new Int32Array(numDiagonals);
        const reversePoints = new Int32Array(numDiagonals);
        const diagonalForwardBase = modifiedEnd - modifiedStart;
        const diagonalReverseBase = originalEnd - originalStart;
        const diagonalForwardOffset = originalStart - modifiedStart;
        const diagonalReverseOffset = originalEnd - modifiedEnd;
        const delta = diagonalReverseBase - diagonalForwardBase;
        const deltaIsEven = delta % 2 === 0;
        forwardPoints[diagonalForwardBase] = originalStart;
        reversePoints[diagonalReverseBase] = originalEnd;
        quitEarlyArr[0] = false;
        for (let numDifferences = 1; numDifferences <= maxDifferences / 2 + 1; numDifferences++) {
          let furthestOriginalIndex = 0;
          let furthestModifiedIndex = 0;
          diagonalForwardStart = this.ClipDiagonalBound(diagonalForwardBase - numDifferences, numDifferences, diagonalForwardBase, numDiagonals);
          diagonalForwardEnd = this.ClipDiagonalBound(diagonalForwardBase + numDifferences, numDifferences, diagonalForwardBase, numDiagonals);
          for (let diagonal = diagonalForwardStart; diagonal <= diagonalForwardEnd; diagonal += 2) {
            if (diagonal === diagonalForwardStart || diagonal < diagonalForwardEnd && forwardPoints[diagonal - 1] < forwardPoints[diagonal + 1]) {
              originalIndex = forwardPoints[diagonal + 1];
            } else {
              originalIndex = forwardPoints[diagonal - 1] + 1;
            }
            modifiedIndex = originalIndex - (diagonal - diagonalForwardBase) - diagonalForwardOffset;
            const tempOriginalIndex = originalIndex;
            while (originalIndex < originalEnd && modifiedIndex < modifiedEnd && this.ElementsAreEqual(originalIndex + 1, modifiedIndex + 1)) {
              originalIndex++;
              modifiedIndex++;
            }
            forwardPoints[diagonal] = originalIndex;
            if (originalIndex + modifiedIndex > furthestOriginalIndex + furthestModifiedIndex) {
              furthestOriginalIndex = originalIndex;
              furthestModifiedIndex = modifiedIndex;
            }
            if (!deltaIsEven && Math.abs(diagonal - diagonalReverseBase) <= numDifferences - 1) {
              if (originalIndex >= reversePoints[diagonal]) {
                midOriginalArr[0] = originalIndex;
                midModifiedArr[0] = modifiedIndex;
                if (tempOriginalIndex <= reversePoints[diagonal] && 1447 > 0 && numDifferences <= 1447 + 1) {
                  return this.WALKTRACE(diagonalForwardBase, diagonalForwardStart, diagonalForwardEnd, diagonalForwardOffset, diagonalReverseBase, diagonalReverseStart, diagonalReverseEnd, diagonalReverseOffset, forwardPoints, reversePoints, originalIndex, originalEnd, midOriginalArr, modifiedIndex, modifiedEnd, midModifiedArr, deltaIsEven, quitEarlyArr);
                } else {
                  return null;
                }
              }
            }
          }
          const matchLengthOfLongest = (furthestOriginalIndex - originalStart + (furthestModifiedIndex - modifiedStart) - numDifferences) / 2;
          if (this.ContinueProcessingPredicate !== null && !this.ContinueProcessingPredicate(furthestOriginalIndex, matchLengthOfLongest)) {
            quitEarlyArr[0] = true;
            midOriginalArr[0] = furthestOriginalIndex;
            midModifiedArr[0] = furthestModifiedIndex;
            if (matchLengthOfLongest > 0 && 1447 > 0 && numDifferences <= 1447 + 1) {
              return this.WALKTRACE(diagonalForwardBase, diagonalForwardStart, diagonalForwardEnd, diagonalForwardOffset, diagonalReverseBase, diagonalReverseStart, diagonalReverseEnd, diagonalReverseOffset, forwardPoints, reversePoints, originalIndex, originalEnd, midOriginalArr, modifiedIndex, modifiedEnd, midModifiedArr, deltaIsEven, quitEarlyArr);
            } else {
              originalStart++;
              modifiedStart++;
              return [
                new diffChange_js_1.DiffChange(originalStart, originalEnd - originalStart + 1, modifiedStart, modifiedEnd - modifiedStart + 1)
              ];
            }
          }
          diagonalReverseStart = this.ClipDiagonalBound(diagonalReverseBase - numDifferences, numDifferences, diagonalReverseBase, numDiagonals);
          diagonalReverseEnd = this.ClipDiagonalBound(diagonalReverseBase + numDifferences, numDifferences, diagonalReverseBase, numDiagonals);
          for (let diagonal = diagonalReverseStart; diagonal <= diagonalReverseEnd; diagonal += 2) {
            if (diagonal === diagonalReverseStart || diagonal < diagonalReverseEnd && reversePoints[diagonal - 1] >= reversePoints[diagonal + 1]) {
              originalIndex = reversePoints[diagonal + 1] - 1;
            } else {
              originalIndex = reversePoints[diagonal - 1];
            }
            modifiedIndex = originalIndex - (diagonal - diagonalReverseBase) - diagonalReverseOffset;
            const tempOriginalIndex = originalIndex;
            while (originalIndex > originalStart && modifiedIndex > modifiedStart && this.ElementsAreEqual(originalIndex, modifiedIndex)) {
              originalIndex--;
              modifiedIndex--;
            }
            reversePoints[diagonal] = originalIndex;
            if (deltaIsEven && Math.abs(diagonal - diagonalForwardBase) <= numDifferences) {
              if (originalIndex <= forwardPoints[diagonal]) {
                midOriginalArr[0] = originalIndex;
                midModifiedArr[0] = modifiedIndex;
                if (tempOriginalIndex >= forwardPoints[diagonal] && 1447 > 0 && numDifferences <= 1447 + 1) {
                  return this.WALKTRACE(diagonalForwardBase, diagonalForwardStart, diagonalForwardEnd, diagonalForwardOffset, diagonalReverseBase, diagonalReverseStart, diagonalReverseEnd, diagonalReverseOffset, forwardPoints, reversePoints, originalIndex, originalEnd, midOriginalArr, modifiedIndex, modifiedEnd, midModifiedArr, deltaIsEven, quitEarlyArr);
                } else {
                  return null;
                }
              }
            }
          }
          if (numDifferences <= 1447) {
            let temp = new Int32Array(diagonalForwardEnd - diagonalForwardStart + 2);
            temp[0] = diagonalForwardBase - diagonalForwardStart + 1;
            MyArray.Copy2(forwardPoints, diagonalForwardStart, temp, 1, diagonalForwardEnd - diagonalForwardStart + 1);
            this.m_forwardHistory.push(temp);
            temp = new Int32Array(diagonalReverseEnd - diagonalReverseStart + 2);
            temp[0] = diagonalReverseBase - diagonalReverseStart + 1;
            MyArray.Copy2(reversePoints, diagonalReverseStart, temp, 1, diagonalReverseEnd - diagonalReverseStart + 1);
            this.m_reverseHistory.push(temp);
          }
        }
        return this.WALKTRACE(diagonalForwardBase, diagonalForwardStart, diagonalForwardEnd, diagonalForwardOffset, diagonalReverseBase, diagonalReverseStart, diagonalReverseEnd, diagonalReverseOffset, forwardPoints, reversePoints, originalIndex, originalEnd, midOriginalArr, modifiedIndex, modifiedEnd, midModifiedArr, deltaIsEven, quitEarlyArr);
      }
      /**
       * Shifts the given changes to provide a more intuitive diff.
       * While the first element in a diff matches the first element after the diff,
       * we shift the diff down.
       *
       * @param changes The list of changes to shift
       * @returns The shifted changes
       */
      PrettifyChanges(changes) {
        for (let i = 0; i < changes.length; i++) {
          const change = changes[i];
          const originalStop = i < changes.length - 1 ? changes[i + 1].originalStart : this._originalElementsOrHash.length;
          const modifiedStop = i < changes.length - 1 ? changes[i + 1].modifiedStart : this._modifiedElementsOrHash.length;
          const checkOriginal = change.originalLength > 0;
          const checkModified = change.modifiedLength > 0;
          while (change.originalStart + change.originalLength < originalStop && change.modifiedStart + change.modifiedLength < modifiedStop && (!checkOriginal || this.OriginalElementsAreEqual(change.originalStart, change.originalStart + change.originalLength)) && (!checkModified || this.ModifiedElementsAreEqual(change.modifiedStart, change.modifiedStart + change.modifiedLength))) {
            const startStrictEqual = this.ElementsAreStrictEqual(change.originalStart, change.modifiedStart);
            const endStrictEqual = this.ElementsAreStrictEqual(change.originalStart + change.originalLength, change.modifiedStart + change.modifiedLength);
            if (endStrictEqual && !startStrictEqual) {
              break;
            }
            change.originalStart++;
            change.modifiedStart++;
          }
          const mergedChangeArr = [null];
          if (i < changes.length - 1 && this.ChangesOverlap(changes[i], changes[i + 1], mergedChangeArr)) {
            changes[i] = mergedChangeArr[0];
            changes.splice(i + 1, 1);
            i--;
            continue;
          }
        }
        for (let i = changes.length - 1; i >= 0; i--) {
          const change = changes[i];
          let originalStop = 0;
          let modifiedStop = 0;
          if (i > 0) {
            const prevChange = changes[i - 1];
            originalStop = prevChange.originalStart + prevChange.originalLength;
            modifiedStop = prevChange.modifiedStart + prevChange.modifiedLength;
          }
          const checkOriginal = change.originalLength > 0;
          const checkModified = change.modifiedLength > 0;
          let bestDelta = 0;
          let bestScore = this._boundaryScore(change.originalStart, change.originalLength, change.modifiedStart, change.modifiedLength);
          for (let delta = 1; ; delta++) {
            const originalStart = change.originalStart - delta;
            const modifiedStart = change.modifiedStart - delta;
            if (originalStart < originalStop || modifiedStart < modifiedStop) {
              break;
            }
            if (checkOriginal && !this.OriginalElementsAreEqual(originalStart, originalStart + change.originalLength)) {
              break;
            }
            if (checkModified && !this.ModifiedElementsAreEqual(modifiedStart, modifiedStart + change.modifiedLength)) {
              break;
            }
            const touchingPreviousChange = originalStart === originalStop && modifiedStart === modifiedStop;
            const score = (touchingPreviousChange ? 5 : 0) + this._boundaryScore(originalStart, change.originalLength, modifiedStart, change.modifiedLength);
            if (score > bestScore) {
              bestScore = score;
              bestDelta = delta;
            }
          }
          change.originalStart -= bestDelta;
          change.modifiedStart -= bestDelta;
          const mergedChangeArr = [null];
          if (i > 0 && this.ChangesOverlap(changes[i - 1], changes[i], mergedChangeArr)) {
            changes[i - 1] = mergedChangeArr[0];
            changes.splice(i, 1);
            i++;
            continue;
          }
        }
        if (this._hasStrings) {
          for (let i = 1, len = changes.length; i < len; i++) {
            const aChange = changes[i - 1];
            const bChange = changes[i];
            const matchedLength = bChange.originalStart - aChange.originalStart - aChange.originalLength;
            const aOriginalStart = aChange.originalStart;
            const bOriginalEnd = bChange.originalStart + bChange.originalLength;
            const abOriginalLength = bOriginalEnd - aOriginalStart;
            const aModifiedStart = aChange.modifiedStart;
            const bModifiedEnd = bChange.modifiedStart + bChange.modifiedLength;
            const abModifiedLength = bModifiedEnd - aModifiedStart;
            if (matchedLength < 5 && abOriginalLength < 20 && abModifiedLength < 20) {
              const t = this._findBetterContiguousSequence(aOriginalStart, abOriginalLength, aModifiedStart, abModifiedLength, matchedLength);
              if (t) {
                const [originalMatchStart, modifiedMatchStart] = t;
                if (originalMatchStart !== aChange.originalStart + aChange.originalLength || modifiedMatchStart !== aChange.modifiedStart + aChange.modifiedLength) {
                  aChange.originalLength = originalMatchStart - aChange.originalStart;
                  aChange.modifiedLength = modifiedMatchStart - aChange.modifiedStart;
                  bChange.originalStart = originalMatchStart + matchedLength;
                  bChange.modifiedStart = modifiedMatchStart + matchedLength;
                  bChange.originalLength = bOriginalEnd - bChange.originalStart;
                  bChange.modifiedLength = bModifiedEnd - bChange.modifiedStart;
                }
              }
            }
          }
        }
        return changes;
      }
      _findBetterContiguousSequence(originalStart, originalLength, modifiedStart, modifiedLength, desiredLength) {
        if (originalLength < desiredLength || modifiedLength < desiredLength) {
          return null;
        }
        const originalMax = originalStart + originalLength - desiredLength + 1;
        const modifiedMax = modifiedStart + modifiedLength - desiredLength + 1;
        let bestScore = 0;
        let bestOriginalStart = 0;
        let bestModifiedStart = 0;
        for (let i = originalStart; i < originalMax; i++) {
          for (let j = modifiedStart; j < modifiedMax; j++) {
            const score = this._contiguousSequenceScore(i, j, desiredLength);
            if (score > 0 && score > bestScore) {
              bestScore = score;
              bestOriginalStart = i;
              bestModifiedStart = j;
            }
          }
        }
        if (bestScore > 0) {
          return [bestOriginalStart, bestModifiedStart];
        }
        return null;
      }
      _contiguousSequenceScore(originalStart, modifiedStart, length) {
        let score = 0;
        for (let l = 0; l < length; l++) {
          if (!this.ElementsAreEqual(originalStart + l, modifiedStart + l)) {
            return 0;
          }
          score += this._originalStringElements[originalStart + l].length;
        }
        return score;
      }
      _OriginalIsBoundary(index) {
        if (index <= 0 || index >= this._originalElementsOrHash.length - 1) {
          return true;
        }
        return this._hasStrings && /^\s*$/.test(this._originalStringElements[index]);
      }
      _OriginalRegionIsBoundary(originalStart, originalLength) {
        if (this._OriginalIsBoundary(originalStart) || this._OriginalIsBoundary(originalStart - 1)) {
          return true;
        }
        if (originalLength > 0) {
          const originalEnd = originalStart + originalLength;
          if (this._OriginalIsBoundary(originalEnd - 1) || this._OriginalIsBoundary(originalEnd)) {
            return true;
          }
        }
        return false;
      }
      _ModifiedIsBoundary(index) {
        if (index <= 0 || index >= this._modifiedElementsOrHash.length - 1) {
          return true;
        }
        return this._hasStrings && /^\s*$/.test(this._modifiedStringElements[index]);
      }
      _ModifiedRegionIsBoundary(modifiedStart, modifiedLength) {
        if (this._ModifiedIsBoundary(modifiedStart) || this._ModifiedIsBoundary(modifiedStart - 1)) {
          return true;
        }
        if (modifiedLength > 0) {
          const modifiedEnd = modifiedStart + modifiedLength;
          if (this._ModifiedIsBoundary(modifiedEnd - 1) || this._ModifiedIsBoundary(modifiedEnd)) {
            return true;
          }
        }
        return false;
      }
      _boundaryScore(originalStart, originalLength, modifiedStart, modifiedLength) {
        const originalScore = this._OriginalRegionIsBoundary(originalStart, originalLength) ? 1 : 0;
        const modifiedScore = this._ModifiedRegionIsBoundary(modifiedStart, modifiedLength) ? 1 : 0;
        return originalScore + modifiedScore;
      }
      /**
       * Concatenates the two input DiffChange lists and returns the resulting
       * list.
       * @param The left changes
       * @param The right changes
       * @returns The concatenated list
       */
      ConcatenateChanges(left, right) {
        const mergedChangeArr = [];
        if (left.length === 0 || right.length === 0) {
          return right.length > 0 ? right : left;
        } else if (this.ChangesOverlap(left[left.length - 1], right[0], mergedChangeArr)) {
          const result = new Array(left.length + right.length - 1);
          MyArray.Copy(left, 0, result, 0, left.length - 1);
          result[left.length - 1] = mergedChangeArr[0];
          MyArray.Copy(right, 1, result, left.length, right.length - 1);
          return result;
        } else {
          const result = new Array(left.length + right.length);
          MyArray.Copy(left, 0, result, 0, left.length);
          MyArray.Copy(right, 0, result, left.length, right.length);
          return result;
        }
      }
      /**
       * Returns true if the two changes overlap and can be merged into a single
       * change
       * @param left The left change
       * @param right The right change
       * @param mergedChange The merged change if the two overlap, null otherwise
       * @returns True if the two changes overlap
       */
      ChangesOverlap(left, right, mergedChangeArr) {
        Debug.Assert(left.originalStart <= right.originalStart, "Left change is not less than or equal to right change");
        Debug.Assert(left.modifiedStart <= right.modifiedStart, "Left change is not less than or equal to right change");
        if (left.originalStart + left.originalLength >= right.originalStart || left.modifiedStart + left.modifiedLength >= right.modifiedStart) {
          const originalStart = left.originalStart;
          let originalLength = left.originalLength;
          const modifiedStart = left.modifiedStart;
          let modifiedLength = left.modifiedLength;
          if (left.originalStart + left.originalLength >= right.originalStart) {
            originalLength = right.originalStart + right.originalLength - left.originalStart;
          }
          if (left.modifiedStart + left.modifiedLength >= right.modifiedStart) {
            modifiedLength = right.modifiedStart + right.modifiedLength - left.modifiedStart;
          }
          mergedChangeArr[0] = new diffChange_js_1.DiffChange(originalStart, originalLength, modifiedStart, modifiedLength);
          return true;
        } else {
          mergedChangeArr[0] = null;
          return false;
        }
      }
      /**
       * Helper method used to clip a diagonal index to the range of valid
       * diagonals. This also decides whether or not the diagonal index,
       * if it exceeds the boundary, should be clipped to the boundary or clipped
       * one inside the boundary depending on the Even/Odd status of the boundary
       * and numDifferences.
       * @param diagonal The index of the diagonal to clip.
       * @param numDifferences The current number of differences being iterated upon.
       * @param diagonalBaseIndex The base reference diagonal.
       * @param numDiagonals The total number of diagonals.
       * @returns The clipped diagonal index.
       */
      ClipDiagonalBound(diagonal, numDifferences, diagonalBaseIndex, numDiagonals) {
        if (diagonal >= 0 && diagonal < numDiagonals) {
          return diagonal;
        }
        const diagonalsBelow = diagonalBaseIndex;
        const diagonalsAbove = numDiagonals - diagonalBaseIndex - 1;
        const diffEven = numDifferences % 2 === 0;
        if (diagonal < 0) {
          const lowerBoundEven = diagonalsBelow % 2 === 0;
          return diffEven === lowerBoundEven ? 0 : 1;
        } else {
          const upperBoundEven = diagonalsAbove % 2 === 0;
          return diffEven === upperBoundEven ? numDiagonals - 1 : numDiagonals - 2;
        }
      }
    };
    exports.LcsDiff = LcsDiff2;
    var precomputedEqualityArray = new Uint32Array(65536);
    var computeLevenshteinDistanceForShortStrings = (firstString, secondString) => {
      const firstStringLength = firstString.length;
      const secondStringLength = secondString.length;
      const lastBitMask = 1 << firstStringLength - 1;
      let positiveVector = -1;
      let negativeVector = 0;
      let distance = firstStringLength;
      let index = firstStringLength;
      while (index--) {
        precomputedEqualityArray[firstString.charCodeAt(index)] |= 1 << index;
      }
      for (index = 0; index < secondStringLength; index++) {
        let equalityMask = precomputedEqualityArray[secondString.charCodeAt(index)];
        const combinedVector = equalityMask | negativeVector;
        equalityMask |= (equalityMask & positiveVector) + positiveVector ^ positiveVector;
        negativeVector |= ~(equalityMask | positiveVector);
        positiveVector &= equalityMask;
        if (negativeVector & lastBitMask) {
          distance++;
        }
        if (positiveVector & lastBitMask) {
          distance--;
        }
        negativeVector = negativeVector << 1 | 1;
        positiveVector = positiveVector << 1 | ~(combinedVector | negativeVector);
        negativeVector &= combinedVector;
      }
      index = firstStringLength;
      while (index--) {
        precomputedEqualityArray[firstString.charCodeAt(index)] = 0;
      }
      return distance;
    };
    function computeLevenshteinDistanceForLongStrings(firstString, secondString) {
      const firstStringLength = firstString.length;
      const secondStringLength = secondString.length;
      const horizontalBitArray = [];
      const verticalBitArray = [];
      const horizontalSize = Math.ceil(firstStringLength / 32);
      const verticalSize = Math.ceil(secondStringLength / 32);
      for (let i = 0; i < horizontalSize; i++) {
        horizontalBitArray[i] = -1;
        verticalBitArray[i] = 0;
      }
      let verticalIndex = 0;
      for (; verticalIndex < verticalSize - 1; verticalIndex++) {
        let negativeVector2 = 0;
        let positiveVector2 = -1;
        const start2 = verticalIndex * 32;
        const verticalLength2 = Math.min(32, secondStringLength) + start2;
        for (let k = start2; k < verticalLength2; k++) {
          precomputedEqualityArray[secondString.charCodeAt(k)] |= 1 << k;
        }
        for (let i = 0; i < firstStringLength; i++) {
          const equalityMask = precomputedEqualityArray[firstString.charCodeAt(i)];
          const previousBit = horizontalBitArray[i / 32 | 0] >>> i & 1;
          const matchBit = verticalBitArray[i / 32 | 0] >>> i & 1;
          const combinedVector = equalityMask | negativeVector2;
          const combinedHorizontalVector = ((equalityMask | matchBit) & positiveVector2) + positiveVector2 ^ positiveVector2 | equalityMask | matchBit;
          let positiveHorizontalVector = negativeVector2 | ~(combinedHorizontalVector | positiveVector2);
          let negativeHorizontalVector = positiveVector2 & combinedHorizontalVector;
          if (positiveHorizontalVector >>> 31 ^ previousBit) {
            horizontalBitArray[i / 32 | 0] ^= 1 << i;
          }
          if (negativeHorizontalVector >>> 31 ^ matchBit) {
            verticalBitArray[i / 32 | 0] ^= 1 << i;
          }
          positiveHorizontalVector = positiveHorizontalVector << 1 | previousBit;
          negativeHorizontalVector = negativeHorizontalVector << 1 | matchBit;
          positiveVector2 = negativeHorizontalVector | ~(combinedVector | positiveHorizontalVector);
          negativeVector2 = positiveHorizontalVector & combinedVector;
        }
        for (let k = start2; k < verticalLength2; k++) {
          precomputedEqualityArray[secondString.charCodeAt(k)] = 0;
        }
      }
      let negativeVector = 0;
      let positiveVector = -1;
      const start = verticalIndex * 32;
      const verticalLength = Math.min(32, secondStringLength - start) + start;
      for (let k = start; k < verticalLength; k++) {
        precomputedEqualityArray[secondString.charCodeAt(k)] |= 1 << k;
      }
      let distance = secondStringLength;
      for (let i = 0; i < firstStringLength; i++) {
        const equalityMask = precomputedEqualityArray[firstString.charCodeAt(i)];
        const previousBit = horizontalBitArray[i / 32 | 0] >>> i & 1;
        const matchBit = verticalBitArray[i / 32 | 0] >>> i & 1;
        const combinedVector = equalityMask | negativeVector;
        const combinedHorizontalVector = ((equalityMask | matchBit) & positiveVector) + positiveVector ^ positiveVector | equalityMask | matchBit;
        let positiveHorizontalVector = negativeVector | ~(combinedHorizontalVector | positiveVector);
        let negativeHorizontalVector = positiveVector & combinedHorizontalVector;
        distance += positiveHorizontalVector >>> secondStringLength - 1 & 1;
        distance -= negativeHorizontalVector >>> secondStringLength - 1 & 1;
        if (positiveHorizontalVector >>> 31 ^ previousBit) {
          horizontalBitArray[i / 32 | 0] ^= 1 << i;
        }
        if (negativeHorizontalVector >>> 31 ^ matchBit) {
          verticalBitArray[i / 32 | 0] ^= 1 << i;
        }
        positiveHorizontalVector = positiveHorizontalVector << 1 | previousBit;
        negativeHorizontalVector = negativeHorizontalVector << 1 | matchBit;
        positiveVector = negativeHorizontalVector | ~(combinedVector | positiveHorizontalVector);
        negativeVector = positiveHorizontalVector & combinedVector;
      }
      for (let k = start; k < verticalLength; k++) {
        precomputedEqualityArray[secondString.charCodeAt(k)] = 0;
      }
      return distance;
    }
    function computeLevenshteinDistance2(firstString, secondString) {
      if (firstString.length < secondString.length) {
        const temp = secondString;
        secondString = firstString;
        firstString = temp;
      }
      if (secondString.length === 0) {
        return firstString.length;
      }
      if (firstString.length <= 32) {
        return computeLevenshteinDistanceForShortStrings(firstString, secondString);
      }
      return computeLevenshteinDistanceForLongStrings(firstString, secondString);
    }
  }
});

// package/dist/vs/base/common/arrays.js
var require_arrays = __commonJS({
  "package/dist/vs/base/common/arrays.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.numberComparator = exports.CompareResult = void 0;
    exports.equals = equals;
    exports.groupAdjacentBy = groupAdjacentBy;
    exports.forEachAdjacent = forEachAdjacent;
    exports.forEachWithNeighbors = forEachWithNeighbors;
    exports.pushMany = pushMany;
    exports.compareBy = compareBy;
    exports.reverseOrder = reverseOrder;
    function equals(one, other, itemEquals = (a, b) => a === b) {
      if (one === other) {
        return true;
      }
      if (!one || !other) {
        return false;
      }
      if (one.length !== other.length) {
        return false;
      }
      for (let i = 0, len = one.length; i < len; i++) {
        if (!itemEquals(one[i], other[i])) {
          return false;
        }
      }
      return true;
    }
    function* groupAdjacentBy(items, shouldBeGrouped) {
      let currentGroup;
      let last;
      for (const item of items) {
        if (last !== void 0 && shouldBeGrouped(last, item)) {
          currentGroup.push(item);
        } else {
          if (currentGroup) {
            yield currentGroup;
          }
          currentGroup = [item];
        }
        last = item;
      }
      if (currentGroup) {
        yield currentGroup;
      }
    }
    function forEachAdjacent(arr, f) {
      for (let i = 0; i <= arr.length; i++) {
        f(i === 0 ? void 0 : arr[i - 1], i === arr.length ? void 0 : arr[i]);
      }
    }
    function forEachWithNeighbors(arr, f) {
      for (let i = 0; i < arr.length; i++) {
        f(i === 0 ? void 0 : arr[i - 1], arr[i], i + 1 === arr.length ? void 0 : arr[i + 1]);
      }
    }
    function pushMany(arr, items) {
      for (const item of items) {
        arr.push(item);
      }
    }
    var CompareResult;
    (function(CompareResult2) {
      function isLessThan(result) {
        return result < 0;
      }
      CompareResult2.isLessThan = isLessThan;
      function isLessThanOrEqual(result) {
        return result <= 0;
      }
      CompareResult2.isLessThanOrEqual = isLessThanOrEqual;
      function isGreaterThan(result) {
        return result > 0;
      }
      CompareResult2.isGreaterThan = isGreaterThan;
      function isNeitherLessOrGreaterThan(result) {
        return result === 0;
      }
      CompareResult2.isNeitherLessOrGreaterThan = isNeitherLessOrGreaterThan;
      CompareResult2.greaterThan = 1;
      CompareResult2.lessThan = -1;
      CompareResult2.neitherLessOrGreaterThan = 0;
    })(CompareResult || (exports.CompareResult = CompareResult = {}));
    function compareBy(selector, comparator) {
      return (a, b) => comparator(selector(a), selector(b));
    }
    var numberComparator = (a, b) => a - b;
    exports.numberComparator = numberComparator;
    function reverseOrder(comparator) {
      return (a, b) => -comparator(a, b);
    }
  }
});

// package/dist/vs/base/common/errors.js
var require_errors = __commonJS({
  "package/dist/vs/base/common/errors.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.BugIndicatingError = exports.ErrorNoTelemetry = void 0;
    exports.onUnexpectedError = onUnexpectedError;
    var ErrorHandler = class {
      unexpectedErrorHandler;
      listeners;
      constructor() {
        this.listeners = [];
        this.unexpectedErrorHandler = function(e) {
          setTimeout(() => {
            if (e.stack) {
              if (ErrorNoTelemetry.isErrorNoTelemetry(e)) {
                throw new ErrorNoTelemetry(e.message + "\n\n" + e.stack);
              }
              throw new Error(e.message + "\n\n" + e.stack);
            }
            throw e;
          }, 0);
        };
      }
      addListener(listener) {
        this.listeners.push(listener);
        return () => {
          this._removeListener(listener);
        };
      }
      emit(e) {
        this.listeners.forEach((listener) => {
          listener(e);
        });
      }
      _removeListener(listener) {
        this.listeners.splice(this.listeners.indexOf(listener), 1);
      }
      setUnexpectedErrorHandler(newUnexpectedErrorHandler) {
        this.unexpectedErrorHandler = newUnexpectedErrorHandler;
      }
      getUnexpectedErrorHandler() {
        return this.unexpectedErrorHandler;
      }
      onUnexpectedError(e) {
        this.unexpectedErrorHandler(e);
        this.emit(e);
      }
      // For external errors, we don't want the listeners to be called
      onUnexpectedExternalError(e) {
        this.unexpectedErrorHandler(e);
      }
    };
    var errorHandler = new ErrorHandler();
    function onUnexpectedError(e) {
      if (!isCancellationError(e)) {
        errorHandler.onUnexpectedError(e);
      }
      return void 0;
    }
    var canceledName = "Canceled";
    function isCancellationError(error) {
      if (error instanceof CancellationError) {
        return true;
      }
      return error instanceof Error && error.name === canceledName && error.message === canceledName;
    }
    var CancellationError = class extends Error {
      constructor() {
        super(canceledName);
        this.name = this.message;
      }
    };
    var ErrorNoTelemetry = class _ErrorNoTelemetry extends Error {
      name;
      constructor(msg) {
        super(msg);
        this.name = "CodeExpectedError";
      }
      static fromError(err) {
        if (err instanceof _ErrorNoTelemetry) {
          return err;
        }
        const result = new _ErrorNoTelemetry();
        result.message = err.message;
        result.stack = err.stack;
        return result;
      }
      static isErrorNoTelemetry(err) {
        return err.name === "CodeExpectedError";
      }
    };
    exports.ErrorNoTelemetry = ErrorNoTelemetry;
    var BugIndicatingError = class _BugIndicatingError extends Error {
      constructor(message) {
        super(message || "An unexpected bug occurred.");
        Object.setPrototypeOf(this, _BugIndicatingError.prototype);
      }
    };
    exports.BugIndicatingError = BugIndicatingError;
  }
});

// package/dist/vs/base/common/assert.js
var require_assert = __commonJS({
  "package/dist/vs/base/common/assert.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.assert = assert;
    exports.assertFn = assertFn;
    exports.checkAdjacentItems = checkAdjacentItems;
    var errors_js_1 = require_errors();
    function assert(condition, messageOrError = "unexpected state") {
      if (!condition) {
        const errorToThrow = typeof messageOrError === "string" ? new errors_js_1.BugIndicatingError(`Assertion Failed: ${messageOrError}`) : messageOrError;
        throw errorToThrow;
      }
    }
    function assertFn(condition) {
      if (!condition()) {
        debugger;
        condition();
        (0, errors_js_1.onUnexpectedError)(new errors_js_1.BugIndicatingError("Assertion Failed"));
      }
    }
    function checkAdjacentItems(items, predicate) {
      let i = 0;
      while (i < items.length - 1) {
        const a = items[i];
        const b = items[i + 1];
        if (!predicate(a, b)) {
          return false;
        }
        i++;
      }
      return true;
    }
  }
});

// package/dist/vs/editor/common/core/position.js
var require_position = __commonJS({
  "package/dist/vs/editor/common/core/position.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.Position = void 0;
    var Position = class _Position {
      /**
       * line number (starts at 1)
       */
      lineNumber;
      /**
       * column (the first character in a line is between column 1 and column 2)
       */
      column;
      constructor(lineNumber, column) {
        this.lineNumber = lineNumber;
        this.column = column;
      }
      /**
       * Test if this position is before other position.
       * If the two positions are equal, the result will be false.
       */
      isBefore(other) {
        return _Position.isBefore(this, other);
      }
      /**
       * Test if position `a` is before position `b`.
       * If the two positions are equal, the result will be false.
       */
      static isBefore(a, b) {
        if (a.lineNumber < b.lineNumber) {
          return true;
        }
        if (b.lineNumber < a.lineNumber) {
          return false;
        }
        return a.column < b.column;
      }
      /**
       * Test if this position is before other position.
       * If the two positions are equal, the result will be true.
       */
      isBeforeOrEqual(other) {
        return _Position.isBeforeOrEqual(this, other);
      }
      /**
       * Test if position `a` is before position `b`.
       * If the two positions are equal, the result will be true.
       */
      static isBeforeOrEqual(a, b) {
        if (a.lineNumber < b.lineNumber) {
          return true;
        }
        if (b.lineNumber < a.lineNumber) {
          return false;
        }
        return a.column <= b.column;
      }
      /**
       * Convert to a human-readable representation.
       */
      toString() {
        return "(" + this.lineNumber + "," + this.column + ")";
      }
    };
    exports.Position = Position;
  }
});

// package/dist/vs/editor/common/core/range.js
var require_range = __commonJS({
  "package/dist/vs/editor/common/core/range.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.Range = void 0;
    var position_js_1 = require_position();
    var Range = class _Range {
      /**
       * Line number on which the range starts (starts at 1).
       */
      startLineNumber;
      /**
       * Column on which the range starts in line `startLineNumber` (starts at 1).
       */
      startColumn;
      /**
       * Line number on which the range ends.
       */
      endLineNumber;
      /**
       * Column on which the range ends in line `endLineNumber`.
       */
      endColumn;
      constructor(startLineNumber, startColumn, endLineNumber, endColumn) {
        if (startLineNumber > endLineNumber || startLineNumber === endLineNumber && startColumn > endColumn) {
          this.startLineNumber = endLineNumber;
          this.startColumn = endColumn;
          this.endLineNumber = startLineNumber;
          this.endColumn = startColumn;
        } else {
          this.startLineNumber = startLineNumber;
          this.startColumn = startColumn;
          this.endLineNumber = endLineNumber;
          this.endColumn = endColumn;
        }
      }
      /**
       * A reunion of the two ranges.
       * The smallest position will be used as the start point, and the largest one as the end point.
       */
      plusRange(range) {
        return _Range.plusRange(this, range);
      }
      /**
       * A reunion of the two ranges.
       * The smallest position will be used as the start point, and the largest one as the end point.
       */
      static plusRange(a, b) {
        let startLineNumber;
        let startColumn;
        let endLineNumber;
        let endColumn;
        if (b.startLineNumber < a.startLineNumber) {
          startLineNumber = b.startLineNumber;
          startColumn = b.startColumn;
        } else if (b.startLineNumber === a.startLineNumber) {
          startLineNumber = b.startLineNumber;
          startColumn = Math.min(b.startColumn, a.startColumn);
        } else {
          startLineNumber = a.startLineNumber;
          startColumn = a.startColumn;
        }
        if (b.endLineNumber > a.endLineNumber) {
          endLineNumber = b.endLineNumber;
          endColumn = b.endColumn;
        } else if (b.endLineNumber === a.endLineNumber) {
          endLineNumber = b.endLineNumber;
          endColumn = Math.max(b.endColumn, a.endColumn);
        } else {
          endLineNumber = a.endLineNumber;
          endColumn = a.endColumn;
        }
        return new _Range(startLineNumber, startColumn, endLineNumber, endColumn);
      }
      /**
       * Return the end position (which will be after or equal to the start position)
       */
      getEndPosition() {
        return _Range.getEndPosition(this);
      }
      /**
       * Return the end position (which will be after or equal to the start position)
       */
      static getEndPosition(range) {
        return new position_js_1.Position(range.endLineNumber, range.endColumn);
      }
      /**
       * Return the start position (which will be before or equal to the end position)
       */
      getStartPosition() {
        return _Range.getStartPosition(this);
      }
      /**
       * Return the start position (which will be before or equal to the end position)
       */
      static getStartPosition(range) {
        return new position_js_1.Position(range.startLineNumber, range.startColumn);
      }
      // ---
      static fromPositions(start, end = start) {
        return new _Range(start.lineNumber, start.column, end.lineNumber, end.column);
      }
    };
    exports.Range = Range;
  }
});

// package/dist/vs/base/common/arraysFind.js
var require_arraysFind = __commonJS({
  "package/dist/vs/base/common/arraysFind.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.MonotonousArray = void 0;
    exports.findLastMonotonous = findLastMonotonous;
    exports.findLastIdxMonotonous = findLastIdxMonotonous;
    exports.findFirstMonotonous = findFirstMonotonous;
    exports.findFirstIdxMonotonousOrArrLen = findFirstIdxMonotonousOrArrLen;
    function findLastMonotonous(array, predicate) {
      const idx = findLastIdxMonotonous(array, predicate);
      return idx === -1 ? void 0 : array[idx];
    }
    function findLastIdxMonotonous(array, predicate, startIdx = 0, endIdxEx = array.length) {
      let i = startIdx;
      let j = endIdxEx;
      while (i < j) {
        const k = Math.floor((i + j) / 2);
        if (predicate(array[k])) {
          i = k + 1;
        } else {
          j = k;
        }
      }
      return i - 1;
    }
    function findFirstMonotonous(array, predicate) {
      const idx = findFirstIdxMonotonousOrArrLen(array, predicate);
      return idx === array.length ? void 0 : array[idx];
    }
    function findFirstIdxMonotonousOrArrLen(array, predicate, startIdx = 0, endIdxEx = array.length) {
      let i = startIdx;
      let j = endIdxEx;
      while (i < j) {
        const k = Math.floor((i + j) / 2);
        if (predicate(array[k])) {
          j = k;
        } else {
          i = k + 1;
        }
      }
      return i;
    }
    var MonotonousArray = class _MonotonousArray {
      _array;
      static assertInvariants = false;
      _findLastMonotonousLastIdx = 0;
      _prevFindLastPredicate;
      constructor(_array) {
        this._array = _array;
      }
      /**
       * The predicate must be monotonous, i.e. `arr.map(predicate)` must be like `[true, ..., true, false, ..., false]`!
       * For subsequent calls, current predicate must be weaker than (or equal to) the previous predicate, i.e. more entries must be `true`.
       */
      findLastMonotonous(predicate) {
        if (_MonotonousArray.assertInvariants) {
          if (this._prevFindLastPredicate) {
            for (const item of this._array) {
              if (this._prevFindLastPredicate(item) && !predicate(item)) {
                throw new Error("MonotonousArray: current predicate must be weaker than (or equal to) the previous predicate.");
              }
            }
          }
          this._prevFindLastPredicate = predicate;
        }
        const idx = findLastIdxMonotonous(this._array, predicate, this._findLastMonotonousLastIdx);
        this._findLastMonotonousLastIdx = idx + 1;
        return idx === -1 ? void 0 : this._array[idx];
      }
    };
    exports.MonotonousArray = MonotonousArray;
  }
});

// package/dist/vs/editor/common/core/ranges/offsetRange.js
var require_offsetRange = __commonJS({
  "package/dist/vs/editor/common/core/ranges/offsetRange.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.OffsetRangeSet = exports.OffsetRange = void 0;
    var errors_js_1 = require_errors();
    var OffsetRange = class _OffsetRange {
      start;
      endExclusive;
      static ofLength(length) {
        return new _OffsetRange(0, length);
      }
      constructor(start, endExclusive) {
        this.start = start;
        this.endExclusive = endExclusive;
        if (start > endExclusive) {
          throw new errors_js_1.BugIndicatingError(`Invalid range: ${this.toString()}`);
        }
      }
      get isEmpty() {
        return this.start === this.endExclusive;
      }
      delta(offset) {
        return new _OffsetRange(this.start + offset, this.endExclusive + offset);
      }
      deltaStart(offset) {
        return new _OffsetRange(this.start + offset, this.endExclusive);
      }
      deltaEnd(offset) {
        return new _OffsetRange(this.start, this.endExclusive + offset);
      }
      get length() {
        return this.endExclusive - this.start;
      }
      toString() {
        return `[${this.start}, ${this.endExclusive})`;
      }
      /**
       * for all numbers n: range1.contains(n) or range2.contains(n) => range1.join(range2).contains(n)
       * The joined range is the smallest range that contains both ranges.
       */
      join(other) {
        return new _OffsetRange(Math.min(this.start, other.start), Math.max(this.endExclusive, other.endExclusive));
      }
      /**
       * for all numbers n: range1.contains(n) and range2.contains(n) <=> range1.intersect(range2).contains(n)
       *
       * The resulting range is empty if the ranges do not intersect, but touch.
       * If the ranges don't even touch, the result is undefined.
       */
      intersect(other) {
        const start = Math.max(this.start, other.start);
        const end = Math.min(this.endExclusive, other.endExclusive);
        if (start <= end) {
          return new _OffsetRange(start, end);
        }
        return void 0;
      }
      intersects(other) {
        const start = Math.max(this.start, other.start);
        const end = Math.min(this.endExclusive, other.endExclusive);
        return start < end;
      }
      intersectsOrTouches(other) {
        const start = Math.max(this.start, other.start);
        const end = Math.min(this.endExclusive, other.endExclusive);
        return start <= end;
      }
      slice(arr) {
        return arr.slice(this.start, this.endExclusive);
      }
      map(f) {
        const result = [];
        for (let i = this.start; i < this.endExclusive; i++) {
          result.push(f(i));
        }
        return result;
      }
      forEach(f) {
        for (let i = this.start; i < this.endExclusive; i++) {
          f(i);
        }
      }
    };
    exports.OffsetRange = OffsetRange;
    var OffsetRangeSet = class _OffsetRangeSet {
      _sortedRanges = [];
      get ranges() {
        return [...this._sortedRanges];
      }
      addRange(range) {
        let i = 0;
        while (i < this._sortedRanges.length && this._sortedRanges[i].endExclusive < range.start) {
          i++;
        }
        let j = i;
        while (j < this._sortedRanges.length && this._sortedRanges[j].start <= range.endExclusive) {
          j++;
        }
        if (i === j) {
          this._sortedRanges.splice(i, 0, range);
        } else {
          const start = Math.min(range.start, this._sortedRanges[i].start);
          const end = Math.max(range.endExclusive, this._sortedRanges[j - 1].endExclusive);
          this._sortedRanges.splice(i, j - i, new OffsetRange(start, end));
        }
      }
      toString() {
        return this._sortedRanges.map((r) => r.toString()).join(", ");
      }
      /**
       * Returns of there is a value that is contained in this instance and the given range.
       */
      intersectsStrict(other) {
        let i = 0;
        while (i < this._sortedRanges.length && this._sortedRanges[i].endExclusive <= other.start) {
          i++;
        }
        return i < this._sortedRanges.length && this._sortedRanges[i].start < other.endExclusive;
      }
      intersectWithRange(other) {
        const result = new _OffsetRangeSet();
        for (const range of this._sortedRanges) {
          const intersection = range.intersect(other);
          if (intersection) {
            result.addRange(intersection);
          }
        }
        return result;
      }
      intersectWithRangeLength(other) {
        return this.intersectWithRange(other).length;
      }
      get length() {
        return this._sortedRanges.reduce((prev, cur) => prev + cur.length, 0);
      }
    };
    exports.OffsetRangeSet = OffsetRangeSet;
  }
});

// package/dist/vs/editor/common/core/ranges/lineRange.js
var require_lineRange = __commonJS({
  "package/dist/vs/editor/common/core/ranges/lineRange.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.LineRangeSet = exports.LineRange = void 0;
    var arraysFind_js_1 = require_arraysFind();
    var errors_js_1 = require_errors();
    var range_js_1 = require_range();
    var offsetRange_js_1 = require_offsetRange();
    var LineRange = class _LineRange {
      /**
       * The start line number.
       */
      startLineNumber;
      /**
       * The end line number (exclusive).
       */
      endLineNumberExclusive;
      constructor(startLineNumber, endLineNumberExclusive) {
        if (startLineNumber > endLineNumberExclusive) {
          throw new errors_js_1.BugIndicatingError(`startLineNumber ${startLineNumber} cannot be after endLineNumberExclusive ${endLineNumberExclusive}`);
        }
        this.startLineNumber = startLineNumber;
        this.endLineNumberExclusive = endLineNumberExclusive;
      }
      /**
       * Indicates if this line range is empty.
       */
      get isEmpty() {
        return this.startLineNumber === this.endLineNumberExclusive;
      }
      /**
       * Moves this line range by the given offset of line numbers.
       */
      delta(offset) {
        return new _LineRange(this.startLineNumber + offset, this.endLineNumberExclusive + offset);
      }
      /**
       * The number of lines this line range spans.
       */
      get length() {
        return this.endLineNumberExclusive - this.startLineNumber;
      }
      /**
       * Creates a line range that combines this and the given line range.
       */
      join(other) {
        return new _LineRange(Math.min(this.startLineNumber, other.startLineNumber), Math.max(this.endLineNumberExclusive, other.endLineNumberExclusive));
      }
      toString() {
        return `[${this.startLineNumber},${this.endLineNumberExclusive})`;
      }
      /**
       * The resulting range is empty if the ranges do not intersect, but touch.
       * If the ranges don't even touch, the result is undefined.
       */
      intersect(other) {
        const startLineNumber = Math.max(this.startLineNumber, other.startLineNumber);
        const endLineNumberExclusive = Math.min(this.endLineNumberExclusive, other.endLineNumberExclusive);
        if (startLineNumber <= endLineNumberExclusive) {
          return new _LineRange(startLineNumber, endLineNumberExclusive);
        }
        return void 0;
      }
      intersectsOrTouches(other) {
        return this.startLineNumber <= other.endLineNumberExclusive && other.startLineNumber <= this.endLineNumberExclusive;
      }
      toInclusiveRange() {
        if (this.isEmpty) {
          return null;
        }
        return new range_js_1.Range(this.startLineNumber, 1, this.endLineNumberExclusive - 1, Number.MAX_SAFE_INTEGER);
      }
      /**
       * Converts this 1-based line range to a 0-based offset range (subtracts 1!).
       * @internal
       */
      toOffsetRange() {
        return new offsetRange_js_1.OffsetRange(this.startLineNumber - 1, this.endLineNumberExclusive - 1);
      }
    };
    exports.LineRange = LineRange;
    var LineRangeSet = class _LineRangeSet {
      _normalizedRanges;
      constructor(_normalizedRanges = []) {
        this._normalizedRanges = _normalizedRanges;
      }
      get ranges() {
        return this._normalizedRanges;
      }
      addRange(range) {
        if (range.length === 0) {
          return;
        }
        const joinRangeStartIdx = (0, arraysFind_js_1.findFirstIdxMonotonousOrArrLen)(this._normalizedRanges, (r) => r.endLineNumberExclusive >= range.startLineNumber);
        const joinRangeEndIdxExclusive = (0, arraysFind_js_1.findLastIdxMonotonous)(this._normalizedRanges, (r) => r.startLineNumber <= range.endLineNumberExclusive) + 1;
        if (joinRangeStartIdx === joinRangeEndIdxExclusive) {
          this._normalizedRanges.splice(joinRangeStartIdx, 0, range);
        } else if (joinRangeStartIdx === joinRangeEndIdxExclusive - 1) {
          const joinRange = this._normalizedRanges[joinRangeStartIdx];
          this._normalizedRanges[joinRangeStartIdx] = joinRange.join(range);
        } else {
          const joinRange = this._normalizedRanges[joinRangeStartIdx].join(this._normalizedRanges[joinRangeEndIdxExclusive - 1]).join(range);
          this._normalizedRanges.splice(joinRangeStartIdx, joinRangeEndIdxExclusive - joinRangeStartIdx, joinRange);
        }
      }
      contains(lineNumber) {
        const rangeThatStartsBeforeEnd = (0, arraysFind_js_1.findLastMonotonous)(this._normalizedRanges, (r) => r.startLineNumber <= lineNumber);
        return !!rangeThatStartsBeforeEnd && rangeThatStartsBeforeEnd.endLineNumberExclusive > lineNumber;
      }
      /**
       * Subtracts all ranges in this set from `range` and returns the result.
       */
      subtractFrom(range) {
        const joinRangeStartIdx = (0, arraysFind_js_1.findFirstIdxMonotonousOrArrLen)(this._normalizedRanges, (r) => r.endLineNumberExclusive >= range.startLineNumber);
        const joinRangeEndIdxExclusive = (0, arraysFind_js_1.findLastIdxMonotonous)(this._normalizedRanges, (r) => r.startLineNumber <= range.endLineNumberExclusive) + 1;
        if (joinRangeStartIdx === joinRangeEndIdxExclusive) {
          return new _LineRangeSet([range]);
        }
        const result = [];
        let startLineNumber = range.startLineNumber;
        for (let i = joinRangeStartIdx; i < joinRangeEndIdxExclusive; i++) {
          const r = this._normalizedRanges[i];
          if (r.startLineNumber > startLineNumber) {
            result.push(new LineRange(startLineNumber, r.startLineNumber));
          }
          startLineNumber = r.endLineNumberExclusive;
        }
        if (startLineNumber < range.endLineNumberExclusive) {
          result.push(new LineRange(startLineNumber, range.endLineNumberExclusive));
        }
        return new _LineRangeSet(result);
      }
      getIntersection(other) {
        const result = [];
        let i1 = 0;
        let i2 = 0;
        while (i1 < this._normalizedRanges.length && i2 < other._normalizedRanges.length) {
          const r1 = this._normalizedRanges[i1];
          const r2 = other._normalizedRanges[i2];
          const i = r1.intersect(r2);
          if (i && !i.isEmpty) {
            result.push(i);
          }
          if (r1.endLineNumberExclusive < r2.endLineNumberExclusive) {
            i1++;
          } else {
            i2++;
          }
        }
        return new _LineRangeSet(result);
      }
      getWithDelta(value) {
        return new _LineRangeSet(this._normalizedRanges.map((r) => r.delta(value)));
      }
    };
    exports.LineRangeSet = LineRangeSet;
  }
});

// package/dist/vs/editor/common/core/text/textLength.js
var require_textLength = __commonJS({
  "package/dist/vs/editor/common/core/text/textLength.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.TextLength = void 0;
    var TextLength = class {
      lineCount;
      columnCount;
      constructor(lineCount, columnCount) {
        this.lineCount = lineCount;
        this.columnCount = columnCount;
      }
    };
    exports.TextLength = TextLength;
  }
});

// package/dist/vs/editor/common/core/text/abstractText.js
var require_abstractText = __commonJS({
  "package/dist/vs/editor/common/core/text/abstractText.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.ArrayText = exports.AbstractText = void 0;
    var assert_js_1 = require_assert();
    var range_js_1 = require_range();
    var textLength_js_1 = require_textLength();
    var AbstractText = class {
      getLineLength(lineNumber) {
        return this.getValueOfRange(new range_js_1.Range(lineNumber, 1, lineNumber, Number.MAX_SAFE_INTEGER)).length;
      }
    };
    exports.AbstractText = AbstractText;
    var LineBasedText = class extends AbstractText {
      _getLineContent;
      _lineCount;
      constructor(_getLineContent, _lineCount) {
        (0, assert_js_1.assert)(_lineCount >= 1);
        super();
        this._getLineContent = _getLineContent;
        this._lineCount = _lineCount;
      }
      getValueOfRange(range) {
        if (range.startLineNumber === range.endLineNumber) {
          return this._getLineContent(range.startLineNumber).substring(range.startColumn - 1, range.endColumn - 1);
        }
        let result = this._getLineContent(range.startLineNumber).substring(range.startColumn - 1);
        for (let i = range.startLineNumber + 1; i < range.endLineNumber; i++) {
          result += "\n" + this._getLineContent(i);
        }
        result += "\n" + this._getLineContent(range.endLineNumber).substring(0, range.endColumn - 1);
        return result;
      }
      getLineLength(lineNumber) {
        return this._getLineContent(lineNumber).length;
      }
      get length() {
        const lastLine = this._getLineContent(this._lineCount);
        return new textLength_js_1.TextLength(this._lineCount - 1, lastLine.length);
      }
    };
    var ArrayText = class extends LineBasedText {
      constructor(lines) {
        super((lineNumber) => lines[lineNumber - 1], lines.length);
      }
    };
    exports.ArrayText = ArrayText;
  }
});

// package/dist/vs/editor/common/diff/linesDiffComputer.js
var require_linesDiffComputer = __commonJS({
  "package/dist/vs/editor/common/diff/linesDiffComputer.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.MovedText = exports.LinesDiff = void 0;
    var LinesDiff2 = class {
      changes;
      moves;
      hitTimeout;
      constructor(changes, moves, hitTimeout) {
        this.changes = changes;
        this.moves = moves;
        this.hitTimeout = hitTimeout;
      }
    };
    exports.LinesDiff = LinesDiff2;
    var MovedText2 = class _MovedText {
      lineRangeMapping;
      /**
       * The diff from the original text to the moved text.
       * Must be contained in the original/modified line range.
       * Can be empty if the text didn't change (only moved).
       */
      changes;
      constructor(lineRangeMapping, changes) {
        this.lineRangeMapping = lineRangeMapping;
        this.changes = changes;
      }
      flip() {
        return new _MovedText(this.lineRangeMapping.flip(), this.changes.map((c) => c.flip()));
      }
    };
    exports.MovedText = MovedText2;
  }
});

// package/dist/vs/editor/common/diff/rangeMapping.js
var require_rangeMapping = __commonJS({
  "package/dist/vs/editor/common/diff/rangeMapping.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.RangeMapping = exports.DetailedLineRangeMapping = exports.LineRangeMapping = void 0;
    exports.lineRangeMappingFromRangeMappings = lineRangeMappingFromRangeMappings2;
    exports.getLineRangeMapping = getLineRangeMapping2;
    var arrays_js_1 = require_arrays();
    var assert_js_1 = require_assert();
    var errors_js_1 = require_errors();
    var position_js_1 = require_position();
    var range_js_1 = require_range();
    var lineRange_js_1 = require_lineRange();
    var LineRangeMapping2 = class _LineRangeMapping {
      /**
       * The line range in the original text model.
       */
      original;
      /**
       * The line range in the modified text model.
       */
      modified;
      constructor(originalRange, modifiedRange) {
        this.original = originalRange;
        this.modified = modifiedRange;
      }
      toString() {
        return `{${this.original.toString()}->${this.modified.toString()}}`;
      }
      flip() {
        return new _LineRangeMapping(this.modified, this.original);
      }
      join(other) {
        return new _LineRangeMapping(this.original.join(other.original), this.modified.join(other.modified));
      }
      /**
       * This method assumes that the LineRangeMapping describes a valid diff!
       * I.e. if one range is empty, the other range cannot be the entire document.
       * It avoids various problems when the line range points to non-existing line-numbers.
      */
      toRangeMapping() {
        const origInclusiveRange = this.original.toInclusiveRange();
        const modInclusiveRange = this.modified.toInclusiveRange();
        if (origInclusiveRange && modInclusiveRange) {
          return new RangeMapping2(origInclusiveRange, modInclusiveRange);
        } else if (this.original.startLineNumber === 1 || this.modified.startLineNumber === 1) {
          if (!(this.modified.startLineNumber === 1 && this.original.startLineNumber === 1)) {
            throw new errors_js_1.BugIndicatingError("not a valid diff");
          }
          return new RangeMapping2(new range_js_1.Range(this.original.startLineNumber, 1, this.original.endLineNumberExclusive, 1), new range_js_1.Range(this.modified.startLineNumber, 1, this.modified.endLineNumberExclusive, 1));
        } else {
          return new RangeMapping2(new range_js_1.Range(this.original.startLineNumber - 1, Number.MAX_SAFE_INTEGER, this.original.endLineNumberExclusive - 1, Number.MAX_SAFE_INTEGER), new range_js_1.Range(this.modified.startLineNumber - 1, Number.MAX_SAFE_INTEGER, this.modified.endLineNumberExclusive - 1, Number.MAX_SAFE_INTEGER));
        }
      }
      /**
       * This method assumes that the LineRangeMapping describes a valid diff!
       * I.e. if one range is empty, the other range cannot be the entire document.
       * It avoids various problems when the line range points to non-existing line-numbers.
      */
      toRangeMapping2(original, modified) {
        if (isValidLineNumber(this.original.endLineNumberExclusive, original) && isValidLineNumber(this.modified.endLineNumberExclusive, modified)) {
          return new RangeMapping2(new range_js_1.Range(this.original.startLineNumber, 1, this.original.endLineNumberExclusive, 1), new range_js_1.Range(this.modified.startLineNumber, 1, this.modified.endLineNumberExclusive, 1));
        }
        if (!this.original.isEmpty && !this.modified.isEmpty) {
          return new RangeMapping2(range_js_1.Range.fromPositions(new position_js_1.Position(this.original.startLineNumber, 1), normalizePosition(new position_js_1.Position(this.original.endLineNumberExclusive - 1, Number.MAX_SAFE_INTEGER), original)), range_js_1.Range.fromPositions(new position_js_1.Position(this.modified.startLineNumber, 1), normalizePosition(new position_js_1.Position(this.modified.endLineNumberExclusive - 1, Number.MAX_SAFE_INTEGER), modified)));
        }
        if (this.original.startLineNumber > 1 && this.modified.startLineNumber > 1) {
          return new RangeMapping2(range_js_1.Range.fromPositions(normalizePosition(new position_js_1.Position(this.original.startLineNumber - 1, Number.MAX_SAFE_INTEGER), original), normalizePosition(new position_js_1.Position(this.original.endLineNumberExclusive - 1, Number.MAX_SAFE_INTEGER), original)), range_js_1.Range.fromPositions(normalizePosition(new position_js_1.Position(this.modified.startLineNumber - 1, Number.MAX_SAFE_INTEGER), modified), normalizePosition(new position_js_1.Position(this.modified.endLineNumberExclusive - 1, Number.MAX_SAFE_INTEGER), modified)));
        }
        throw new errors_js_1.BugIndicatingError();
      }
    };
    exports.LineRangeMapping = LineRangeMapping2;
    function normalizePosition(position, content) {
      if (position.lineNumber < 1) {
        return new position_js_1.Position(1, 1);
      }
      if (position.lineNumber > content.length) {
        return new position_js_1.Position(content.length, content[content.length - 1].length + 1);
      }
      const line = content[position.lineNumber - 1];
      if (position.column > line.length + 1) {
        return new position_js_1.Position(position.lineNumber, line.length + 1);
      }
      return position;
    }
    function isValidLineNumber(lineNumber, lines) {
      return lineNumber >= 1 && lineNumber <= lines.length;
    }
    var DetailedLineRangeMapping2 = class _DetailedLineRangeMapping extends LineRangeMapping2 {
      /**
       * If inner changes have not been computed, this is set to undefined.
       * Otherwise, it represents the character-level diff in this line range.
       * The original range of each range mapping should be contained in the original line range (same for modified), exceptions are new-lines.
       * Must not be an empty array.
       */
      innerChanges;
      constructor(originalRange, modifiedRange, innerChanges) {
        super(originalRange, modifiedRange);
        this.innerChanges = innerChanges;
      }
      flip() {
        return new _DetailedLineRangeMapping(this.modified, this.original, this.innerChanges?.map((c) => c.flip()));
      }
    };
    exports.DetailedLineRangeMapping = DetailedLineRangeMapping2;
    var RangeMapping2 = class _RangeMapping {
      static join(rangeMappings) {
        if (rangeMappings.length === 0) {
          throw new errors_js_1.BugIndicatingError("Cannot join an empty list of range mappings");
        }
        let result = rangeMappings[0];
        for (let i = 1; i < rangeMappings.length; i++) {
          result = result.join(rangeMappings[i]);
        }
        return result;
      }
      static assertSorted(rangeMappings) {
        for (let i = 1; i < rangeMappings.length; i++) {
          const previous = rangeMappings[i - 1];
          const current = rangeMappings[i];
          if (!(previous.originalRange.getEndPosition().isBeforeOrEqual(current.originalRange.getStartPosition()) && previous.modifiedRange.getEndPosition().isBeforeOrEqual(current.modifiedRange.getStartPosition()))) {
            throw new errors_js_1.BugIndicatingError("Range mappings must be sorted");
          }
        }
      }
      /**
       * The original range.
       */
      originalRange;
      /**
       * The modified range.
       */
      modifiedRange;
      constructor(originalRange, modifiedRange) {
        this.originalRange = originalRange;
        this.modifiedRange = modifiedRange;
      }
      flip() {
        return new _RangeMapping(this.modifiedRange, this.originalRange);
      }
      join(other) {
        return new _RangeMapping(this.originalRange.plusRange(other.originalRange), this.modifiedRange.plusRange(other.modifiedRange));
      }
    };
    exports.RangeMapping = RangeMapping2;
    function lineRangeMappingFromRangeMappings2(alignments, originalLines, modifiedLines, dontAssertStartLine = false) {
      const changes = [];
      for (const g of (0, arrays_js_1.groupAdjacentBy)(alignments.map((a) => getLineRangeMapping2(a, originalLines, modifiedLines)), (a1, a2) => a1.original.intersectsOrTouches(a2.original) || a1.modified.intersectsOrTouches(a2.modified))) {
        const first = g[0];
        const last = g[g.length - 1];
        changes.push(new DetailedLineRangeMapping2(first.original.join(last.original), first.modified.join(last.modified), g.map((a) => a.innerChanges[0])));
      }
      (0, assert_js_1.assertFn)(() => {
        if (!dontAssertStartLine && changes.length > 0) {
          if (changes[0].modified.startLineNumber !== changes[0].original.startLineNumber) {
            return false;
          }
          if (modifiedLines.length.lineCount - changes[changes.length - 1].modified.endLineNumberExclusive !== originalLines.length.lineCount - changes[changes.length - 1].original.endLineNumberExclusive) {
            return false;
          }
        }
        return (0, assert_js_1.checkAdjacentItems)(changes, (m1, m2) => m2.original.startLineNumber - m1.original.endLineNumberExclusive === m2.modified.startLineNumber - m1.modified.endLineNumberExclusive && // There has to be an unchanged line in between (otherwise both diffs should have been joined)
        m1.original.endLineNumberExclusive < m2.original.startLineNumber && m1.modified.endLineNumberExclusive < m2.modified.startLineNumber);
      });
      return changes;
    }
    function getLineRangeMapping2(rangeMapping, originalLines, modifiedLines) {
      let lineStartDelta = 0;
      let lineEndDelta = 0;
      if (rangeMapping.modifiedRange.endColumn === 1 && rangeMapping.originalRange.endColumn === 1 && rangeMapping.originalRange.startLineNumber + lineStartDelta <= rangeMapping.originalRange.endLineNumber && rangeMapping.modifiedRange.startLineNumber + lineStartDelta <= rangeMapping.modifiedRange.endLineNumber) {
        lineEndDelta = -1;
      }
      if (rangeMapping.modifiedRange.startColumn - 1 >= modifiedLines.getLineLength(rangeMapping.modifiedRange.startLineNumber) && rangeMapping.originalRange.startColumn - 1 >= originalLines.getLineLength(rangeMapping.originalRange.startLineNumber) && rangeMapping.originalRange.startLineNumber <= rangeMapping.originalRange.endLineNumber + lineEndDelta && rangeMapping.modifiedRange.startLineNumber <= rangeMapping.modifiedRange.endLineNumber + lineEndDelta) {
        lineStartDelta = 1;
      }
      const originalLineRange = new lineRange_js_1.LineRange(rangeMapping.originalRange.startLineNumber + lineStartDelta, rangeMapping.originalRange.endLineNumber + 1 + lineEndDelta);
      const modifiedLineRange = new lineRange_js_1.LineRange(rangeMapping.modifiedRange.startLineNumber + lineStartDelta, rangeMapping.modifiedRange.endLineNumber + 1 + lineEndDelta);
      return new DetailedLineRangeMapping2(originalLineRange, modifiedLineRange, [rangeMapping]);
    }
  }
});

// package/dist/vs/editor/common/diff/defaultLinesDiffComputer/algorithms/diffAlgorithm.js
var require_diffAlgorithm = __commonJS({
  "package/dist/vs/editor/common/diff/defaultLinesDiffComputer/algorithms/diffAlgorithm.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.DateTimeout = exports.InfiniteTimeout = exports.OffsetPair = exports.SequenceDiff = exports.DiffAlgorithmResult = void 0;
    var arrays_js_1 = require_arrays();
    var errors_js_1 = require_errors();
    var offsetRange_js_1 = require_offsetRange();
    var DiffAlgorithmResult = class _DiffAlgorithmResult {
      diffs;
      hitTimeout;
      static trivial(seq1, seq2) {
        return new _DiffAlgorithmResult([new SequenceDiff(offsetRange_js_1.OffsetRange.ofLength(seq1.length), offsetRange_js_1.OffsetRange.ofLength(seq2.length))], false);
      }
      static trivialTimedOut(seq1, seq2) {
        return new _DiffAlgorithmResult([new SequenceDiff(offsetRange_js_1.OffsetRange.ofLength(seq1.length), offsetRange_js_1.OffsetRange.ofLength(seq2.length))], true);
      }
      constructor(diffs, hitTimeout) {
        this.diffs = diffs;
        this.hitTimeout = hitTimeout;
      }
    };
    exports.DiffAlgorithmResult = DiffAlgorithmResult;
    var SequenceDiff = class _SequenceDiff {
      seq1Range;
      seq2Range;
      static invert(sequenceDiffs, doc1Length) {
        const result = [];
        (0, arrays_js_1.forEachAdjacent)(sequenceDiffs, (a, b) => {
          result.push(_SequenceDiff.fromOffsetPairs(a ? a.getEndExclusives() : OffsetPair.zero, b ? b.getStarts() : new OffsetPair(doc1Length, (a ? a.seq2Range.endExclusive - a.seq1Range.endExclusive : 0) + doc1Length)));
        });
        return result;
      }
      static fromOffsetPairs(start, endExclusive) {
        return new _SequenceDiff(new offsetRange_js_1.OffsetRange(start.offset1, endExclusive.offset1), new offsetRange_js_1.OffsetRange(start.offset2, endExclusive.offset2));
      }
      static assertSorted(sequenceDiffs) {
        let last = void 0;
        for (const cur of sequenceDiffs) {
          if (last) {
            if (!(last.seq1Range.endExclusive <= cur.seq1Range.start && last.seq2Range.endExclusive <= cur.seq2Range.start)) {
              throw new errors_js_1.BugIndicatingError("Sequence diffs must be sorted");
            }
          }
          last = cur;
        }
      }
      constructor(seq1Range, seq2Range) {
        this.seq1Range = seq1Range;
        this.seq2Range = seq2Range;
      }
      swap() {
        return new _SequenceDiff(this.seq2Range, this.seq1Range);
      }
      toString() {
        return `${this.seq1Range} <-> ${this.seq2Range}`;
      }
      join(other) {
        return new _SequenceDiff(this.seq1Range.join(other.seq1Range), this.seq2Range.join(other.seq2Range));
      }
      delta(offset) {
        if (offset === 0) {
          return this;
        }
        return new _SequenceDiff(this.seq1Range.delta(offset), this.seq2Range.delta(offset));
      }
      deltaStart(offset) {
        if (offset === 0) {
          return this;
        }
        return new _SequenceDiff(this.seq1Range.deltaStart(offset), this.seq2Range.deltaStart(offset));
      }
      deltaEnd(offset) {
        if (offset === 0) {
          return this;
        }
        return new _SequenceDiff(this.seq1Range.deltaEnd(offset), this.seq2Range.deltaEnd(offset));
      }
      intersectsOrTouches(other) {
        return this.seq1Range.intersectsOrTouches(other.seq1Range) || this.seq2Range.intersectsOrTouches(other.seq2Range);
      }
      intersect(other) {
        const i1 = this.seq1Range.intersect(other.seq1Range);
        const i2 = this.seq2Range.intersect(other.seq2Range);
        if (!i1 || !i2) {
          return void 0;
        }
        return new _SequenceDiff(i1, i2);
      }
      getStarts() {
        return new OffsetPair(this.seq1Range.start, this.seq2Range.start);
      }
      getEndExclusives() {
        return new OffsetPair(this.seq1Range.endExclusive, this.seq2Range.endExclusive);
      }
    };
    exports.SequenceDiff = SequenceDiff;
    var OffsetPair = class _OffsetPair {
      offset1;
      offset2;
      static zero = new _OffsetPair(0, 0);
      static max = new _OffsetPair(Number.MAX_SAFE_INTEGER, Number.MAX_SAFE_INTEGER);
      constructor(offset1, offset2) {
        this.offset1 = offset1;
        this.offset2 = offset2;
      }
      toString() {
        return `${this.offset1} <-> ${this.offset2}`;
      }
      delta(offset) {
        if (offset === 0) {
          return this;
        }
        return new _OffsetPair(this.offset1 + offset, this.offset2 + offset);
      }
      equals(other) {
        return this.offset1 === other.offset1 && this.offset2 === other.offset2;
      }
    };
    exports.OffsetPair = OffsetPair;
    var InfiniteTimeout = class _InfiniteTimeout {
      static instance = new _InfiniteTimeout();
      isValid() {
        return true;
      }
    };
    exports.InfiniteTimeout = InfiniteTimeout;
    var DateTimeout = class {
      timeout;
      startTime = Date.now();
      valid = true;
      constructor(timeout) {
        this.timeout = timeout;
        if (timeout <= 0) {
          throw new errors_js_1.BugIndicatingError("timeout must be positive");
        }
      }
      // Recommendation: Set a log-point `{this.disable()}` in the body
      isValid() {
        const valid = Date.now() - this.startTime < this.timeout;
        if (!valid && this.valid) {
          this.valid = false;
        }
        return this.valid;
      }
      disable() {
        this.timeout = Number.MAX_SAFE_INTEGER;
        this.isValid = () => true;
        this.valid = true;
      }
    };
    exports.DateTimeout = DateTimeout;
  }
});

// package/dist/vs/editor/common/diff/defaultLinesDiffComputer/utils.js
var require_utils = __commonJS({
  "package/dist/vs/editor/common/diff/defaultLinesDiffComputer/utils.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.LineRangeFragment = exports.Array2D = void 0;
    exports.isSpace = isSpace;
    var Array2D = class {
      width;
      height;
      array = [];
      constructor(width, height) {
        this.width = width;
        this.height = height;
        this.array = new Array(width * height);
      }
      get(x, y) {
        return this.array[x + y * this.width];
      }
      set(x, y, value) {
        this.array[x + y * this.width] = value;
      }
    };
    exports.Array2D = Array2D;
    function isSpace(charCode) {
      return charCode === 32 || charCode === 9;
    }
    var LineRangeFragment = class _LineRangeFragment {
      range;
      lines;
      source;
      static chrKeys = /* @__PURE__ */ new Map();
      static getKey(chr) {
        let key = this.chrKeys.get(chr);
        if (key === void 0) {
          key = this.chrKeys.size;
          this.chrKeys.set(chr, key);
        }
        return key;
      }
      totalCount;
      histogram = [];
      constructor(range, lines, source) {
        this.range = range;
        this.lines = lines;
        this.source = source;
        let counter = 0;
        for (let i = range.startLineNumber - 1; i < range.endLineNumberExclusive - 1; i++) {
          const line = lines[i];
          for (let j = 0; j < line.length; j++) {
            counter++;
            const chr = line[j];
            const key2 = _LineRangeFragment.getKey(chr);
            this.histogram[key2] = (this.histogram[key2] || 0) + 1;
          }
          counter++;
          const key = _LineRangeFragment.getKey("\n");
          this.histogram[key] = (this.histogram[key] || 0) + 1;
        }
        this.totalCount = counter;
      }
      computeSimilarity(other) {
        let sumDifferences = 0;
        const maxLength = Math.max(this.histogram.length, other.histogram.length);
        for (let i = 0; i < maxLength; i++) {
          sumDifferences += Math.abs((this.histogram[i] ?? 0) - (other.histogram[i] ?? 0));
        }
        return 1 - sumDifferences / (this.totalCount + other.totalCount);
      }
    };
    exports.LineRangeFragment = LineRangeFragment;
  }
});

// package/dist/vs/editor/common/diff/defaultLinesDiffComputer/algorithms/dynamicProgrammingDiffing.js
var require_dynamicProgrammingDiffing = __commonJS({
  "package/dist/vs/editor/common/diff/defaultLinesDiffComputer/algorithms/dynamicProgrammingDiffing.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.DynamicProgrammingDiffing = void 0;
    var offsetRange_js_1 = require_offsetRange();
    var utils_js_1 = require_utils();
    var diffAlgorithm_js_1 = require_diffAlgorithm();
    var DynamicProgrammingDiffing = class {
      compute(sequence1, sequence2, timeout = diffAlgorithm_js_1.InfiniteTimeout.instance, equalityScore) {
        if (sequence1.length === 0 || sequence2.length === 0) {
          return diffAlgorithm_js_1.DiffAlgorithmResult.trivial(sequence1, sequence2);
        }
        const lcsLengths = new utils_js_1.Array2D(sequence1.length, sequence2.length);
        const directions = new utils_js_1.Array2D(sequence1.length, sequence2.length);
        const lengths = new utils_js_1.Array2D(sequence1.length, sequence2.length);
        for (let s12 = 0; s12 < sequence1.length; s12++) {
          for (let s22 = 0; s22 < sequence2.length; s22++) {
            if (!timeout.isValid()) {
              return diffAlgorithm_js_1.DiffAlgorithmResult.trivialTimedOut(sequence1, sequence2);
            }
            const horizontalLen = s12 === 0 ? 0 : lcsLengths.get(s12 - 1, s22);
            const verticalLen = s22 === 0 ? 0 : lcsLengths.get(s12, s22 - 1);
            let extendedSeqScore;
            if (sequence1.getElement(s12) === sequence2.getElement(s22)) {
              if (s12 === 0 || s22 === 0) {
                extendedSeqScore = 0;
              } else {
                extendedSeqScore = lcsLengths.get(s12 - 1, s22 - 1);
              }
              if (s12 > 0 && s22 > 0 && directions.get(s12 - 1, s22 - 1) === 3) {
                extendedSeqScore += lengths.get(s12 - 1, s22 - 1);
              }
              extendedSeqScore += equalityScore ? equalityScore(s12, s22) : 1;
            } else {
              extendedSeqScore = -1;
            }
            const newValue = Math.max(horizontalLen, verticalLen, extendedSeqScore);
            if (newValue === extendedSeqScore) {
              const prevLen = s12 > 0 && s22 > 0 ? lengths.get(s12 - 1, s22 - 1) : 0;
              lengths.set(s12, s22, prevLen + 1);
              directions.set(s12, s22, 3);
            } else if (newValue === horizontalLen) {
              lengths.set(s12, s22, 0);
              directions.set(s12, s22, 1);
            } else if (newValue === verticalLen) {
              lengths.set(s12, s22, 0);
              directions.set(s12, s22, 2);
            }
            lcsLengths.set(s12, s22, newValue);
          }
        }
        const result = [];
        let lastAligningPosS1 = sequence1.length;
        let lastAligningPosS2 = sequence2.length;
        function reportDecreasingAligningPositions(s12, s22) {
          if (s12 + 1 !== lastAligningPosS1 || s22 + 1 !== lastAligningPosS2) {
            result.push(new diffAlgorithm_js_1.SequenceDiff(new offsetRange_js_1.OffsetRange(s12 + 1, lastAligningPosS1), new offsetRange_js_1.OffsetRange(s22 + 1, lastAligningPosS2)));
          }
          lastAligningPosS1 = s12;
          lastAligningPosS2 = s22;
        }
        let s1 = sequence1.length - 1;
        let s2 = sequence2.length - 1;
        while (s1 >= 0 && s2 >= 0) {
          if (directions.get(s1, s2) === 3) {
            reportDecreasingAligningPositions(s1, s2);
            s1--;
            s2--;
          } else {
            if (directions.get(s1, s2) === 1) {
              s1--;
            } else {
              s2--;
            }
          }
        }
        reportDecreasingAligningPositions(-1, -1);
        result.reverse();
        return new diffAlgorithm_js_1.DiffAlgorithmResult(result, false);
      }
    };
    exports.DynamicProgrammingDiffing = DynamicProgrammingDiffing;
  }
});

// package/dist/vs/editor/common/diff/defaultLinesDiffComputer/algorithms/myersDiffAlgorithm.js
var require_myersDiffAlgorithm = __commonJS({
  "package/dist/vs/editor/common/diff/defaultLinesDiffComputer/algorithms/myersDiffAlgorithm.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.MyersDiffAlgorithm = void 0;
    var offsetRange_js_1 = require_offsetRange();
    var diffAlgorithm_js_1 = require_diffAlgorithm();
    var MyersDiffAlgorithm = class {
      compute(seq1, seq2, timeout = diffAlgorithm_js_1.InfiniteTimeout.instance) {
        if (seq1.length === 0 || seq2.length === 0) {
          return diffAlgorithm_js_1.DiffAlgorithmResult.trivial(seq1, seq2);
        }
        const seqX = seq1;
        const seqY = seq2;
        function getXAfterSnake(x, y) {
          while (x < seqX.length && y < seqY.length && seqX.getElement(x) === seqY.getElement(y)) {
            x++;
            y++;
          }
          return x;
        }
        let d = 0;
        const V = new FastInt32Array();
        V.set(0, getXAfterSnake(0, 0));
        const paths = new FastArrayNegativeIndices();
        paths.set(0, V.get(0) === 0 ? null : new SnakePath(null, 0, 0, V.get(0)));
        let k = 0;
        loop: while (true) {
          d++;
          if (!timeout.isValid()) {
            return diffAlgorithm_js_1.DiffAlgorithmResult.trivialTimedOut(seqX, seqY);
          }
          const lowerBound = -Math.min(d, seqY.length + d % 2);
          const upperBound = Math.min(d, seqX.length + d % 2);
          for (k = lowerBound; k <= upperBound; k += 2) {
            let step = 0;
            const maxXofDLineTop = k === upperBound ? -1 : V.get(k + 1);
            const maxXofDLineLeft = k === lowerBound ? -1 : V.get(k - 1) + 1;
            step++;
            const x = Math.min(Math.max(maxXofDLineTop, maxXofDLineLeft), seqX.length);
            const y = x - k;
            step++;
            if (x > seqX.length || y > seqY.length) {
              continue;
            }
            const newMaxX = getXAfterSnake(x, y);
            V.set(k, newMaxX);
            const lastPath = x === maxXofDLineTop ? paths.get(k + 1) : paths.get(k - 1);
            paths.set(k, newMaxX !== x ? new SnakePath(lastPath, x, y, newMaxX - x) : lastPath);
            if (V.get(k) === seqX.length && V.get(k) - k === seqY.length) {
              break loop;
            }
          }
        }
        let path = paths.get(k);
        const result = [];
        let lastAligningPosS1 = seqX.length;
        let lastAligningPosS2 = seqY.length;
        while (true) {
          const endX = path ? path.x + path.length : 0;
          const endY = path ? path.y + path.length : 0;
          if (endX !== lastAligningPosS1 || endY !== lastAligningPosS2) {
            result.push(new diffAlgorithm_js_1.SequenceDiff(new offsetRange_js_1.OffsetRange(endX, lastAligningPosS1), new offsetRange_js_1.OffsetRange(endY, lastAligningPosS2)));
          }
          if (!path) {
            break;
          }
          lastAligningPosS1 = path.x;
          lastAligningPosS2 = path.y;
          path = path.prev;
        }
        result.reverse();
        return new diffAlgorithm_js_1.DiffAlgorithmResult(result, false);
      }
    };
    exports.MyersDiffAlgorithm = MyersDiffAlgorithm;
    var SnakePath = class {
      prev;
      x;
      y;
      length;
      constructor(prev, x, y, length) {
        this.prev = prev;
        this.x = x;
        this.y = y;
        this.length = length;
      }
    };
    var FastInt32Array = class {
      positiveArr = new Int32Array(10);
      negativeArr = new Int32Array(10);
      get(idx) {
        if (idx < 0) {
          idx = -idx - 1;
          return this.negativeArr[idx];
        } else {
          return this.positiveArr[idx];
        }
      }
      set(idx, value) {
        if (idx < 0) {
          idx = -idx - 1;
          if (idx >= this.negativeArr.length) {
            const arr = this.negativeArr;
            this.negativeArr = new Int32Array(arr.length * 2);
            this.negativeArr.set(arr);
          }
          this.negativeArr[idx] = value;
        } else {
          if (idx >= this.positiveArr.length) {
            const arr = this.positiveArr;
            this.positiveArr = new Int32Array(arr.length * 2);
            this.positiveArr.set(arr);
          }
          this.positiveArr[idx] = value;
        }
      }
    };
    var FastArrayNegativeIndices = class {
      positiveArr = [];
      negativeArr = [];
      get(idx) {
        if (idx < 0) {
          idx = -idx - 1;
          return this.negativeArr[idx];
        } else {
          return this.positiveArr[idx];
        }
      }
      set(idx, value) {
        if (idx < 0) {
          idx = -idx - 1;
          this.negativeArr[idx] = value;
        } else {
          this.positiveArr[idx] = value;
        }
      }
    };
  }
});

// package/dist/vs/base/common/map.js
var require_map = __commonJS({
  "package/dist/vs/base/common/map.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.SetMap = void 0;
    var SetMap = class {
      map = /* @__PURE__ */ new Map();
      add(key, value) {
        let values = this.map.get(key);
        if (!values) {
          values = /* @__PURE__ */ new Set();
          this.map.set(key, values);
        }
        values.add(value);
      }
      forEach(key, fn) {
        const values = this.map.get(key);
        if (!values) {
          return;
        }
        values.forEach(fn);
      }
    };
    exports.SetMap = SetMap;
  }
});

// package/dist/vs/editor/common/diff/defaultLinesDiffComputer/linesSliceCharSequence.js
var require_linesSliceCharSequence = __commonJS({
  "package/dist/vs/editor/common/diff/defaultLinesDiffComputer/linesSliceCharSequence.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.LinesSliceCharSequence = void 0;
    var arraysFind_js_1 = require_arraysFind();
    var offsetRange_js_1 = require_offsetRange();
    var position_js_1 = require_position();
    var range_js_1 = require_range();
    var utils_js_1 = require_utils();
    var LinesSliceCharSequence = class {
      lines;
      range;
      considerWhitespaceChanges;
      elements = [];
      firstElementOffsetByLineIdx = [];
      lineStartOffsets = [];
      trimmedWsLengthsByLineIdx = [];
      constructor(lines, range, considerWhitespaceChanges) {
        this.lines = lines;
        this.range = range;
        this.considerWhitespaceChanges = considerWhitespaceChanges;
        this.firstElementOffsetByLineIdx.push(0);
        for (let lineNumber = this.range.startLineNumber; lineNumber <= this.range.endLineNumber; lineNumber++) {
          let line = lines[lineNumber - 1];
          let lineStartOffset = 0;
          if (lineNumber === this.range.startLineNumber && this.range.startColumn > 1) {
            lineStartOffset = this.range.startColumn - 1;
            line = line.substring(lineStartOffset);
          }
          this.lineStartOffsets.push(lineStartOffset);
          let trimmedWsLength = 0;
          if (!considerWhitespaceChanges) {
            const trimmedStartLine = line.trimStart();
            trimmedWsLength = line.length - trimmedStartLine.length;
            line = trimmedStartLine.trimEnd();
          }
          this.trimmedWsLengthsByLineIdx.push(trimmedWsLength);
          const lineLength = lineNumber === this.range.endLineNumber ? Math.min(this.range.endColumn - 1 - lineStartOffset - trimmedWsLength, line.length) : line.length;
          for (let i = 0; i < lineLength; i++) {
            this.elements.push(line.charCodeAt(i));
          }
          if (lineNumber < this.range.endLineNumber) {
            this.elements.push("\n".charCodeAt(0));
            this.firstElementOffsetByLineIdx.push(this.elements.length);
          }
        }
      }
      toString() {
        return `Slice: "${this.text}"`;
      }
      get text() {
        return this.getText(new offsetRange_js_1.OffsetRange(0, this.length));
      }
      getText(range) {
        return this.elements.slice(range.start, range.endExclusive).map((e) => String.fromCharCode(e)).join("");
      }
      getElement(offset) {
        return this.elements[offset];
      }
      get length() {
        return this.elements.length;
      }
      getBoundaryScore(length) {
        const prevCategory = getCategory(length > 0 ? this.elements[length - 1] : -1);
        const nextCategory = getCategory(length < this.elements.length ? this.elements[length] : -1);
        if (prevCategory === 7 && nextCategory === 8) {
          return 0;
        }
        if (prevCategory === 8) {
          return 150;
        }
        let score2 = 0;
        if (prevCategory !== nextCategory) {
          score2 += 10;
          if (prevCategory === 0 && nextCategory === 1) {
            score2 += 1;
          }
        }
        score2 += getCategoryBoundaryScore(prevCategory);
        score2 += getCategoryBoundaryScore(nextCategory);
        return score2;
      }
      translateOffset(offset, preference = "right") {
        const i = (0, arraysFind_js_1.findLastIdxMonotonous)(this.firstElementOffsetByLineIdx, (value) => value <= offset);
        const lineOffset = offset - this.firstElementOffsetByLineIdx[i];
        return new position_js_1.Position(this.range.startLineNumber + i, 1 + this.lineStartOffsets[i] + lineOffset + (lineOffset === 0 && preference === "left" ? 0 : this.trimmedWsLengthsByLineIdx[i]));
      }
      translateRange(range) {
        const pos1 = this.translateOffset(range.start, "right");
        const pos2 = this.translateOffset(range.endExclusive, "left");
        if (pos2.isBefore(pos1)) {
          return range_js_1.Range.fromPositions(pos2, pos2);
        }
        return range_js_1.Range.fromPositions(pos1, pos2);
      }
      /**
       * Finds the word that contains the character at the given offset
       */
      findWordContaining(offset) {
        if (offset < 0 || offset >= this.elements.length) {
          return void 0;
        }
        if (!isWordChar(this.elements[offset])) {
          return void 0;
        }
        let start = offset;
        while (start > 0 && isWordChar(this.elements[start - 1])) {
          start--;
        }
        let end = offset;
        while (end < this.elements.length && isWordChar(this.elements[end])) {
          end++;
        }
        return new offsetRange_js_1.OffsetRange(start, end);
      }
      /** fooBar has the two sub-words foo and bar */
      findSubWordContaining(offset) {
        if (offset < 0 || offset >= this.elements.length) {
          return void 0;
        }
        if (!isWordChar(this.elements[offset])) {
          return void 0;
        }
        let start = offset;
        while (start > 0 && isWordChar(this.elements[start - 1]) && !isUpperCase(this.elements[start])) {
          start--;
        }
        let end = offset;
        while (end < this.elements.length && isWordChar(this.elements[end]) && !isUpperCase(this.elements[end])) {
          end++;
        }
        return new offsetRange_js_1.OffsetRange(start, end);
      }
      countLinesIn(range) {
        return this.translateOffset(range.endExclusive).lineNumber - this.translateOffset(range.start).lineNumber;
      }
      isStronglyEqual(offset1, offset2) {
        return this.elements[offset1] === this.elements[offset2];
      }
      extendToFullLines(range) {
        const start = (0, arraysFind_js_1.findLastMonotonous)(this.firstElementOffsetByLineIdx, (x) => x <= range.start) ?? 0;
        const end = (0, arraysFind_js_1.findFirstMonotonous)(this.firstElementOffsetByLineIdx, (x) => range.endExclusive <= x) ?? this.elements.length;
        return new offsetRange_js_1.OffsetRange(start, end);
      }
    };
    exports.LinesSliceCharSequence = LinesSliceCharSequence;
    function isWordChar(charCode) {
      return charCode >= 97 && charCode <= 122 || charCode >= 65 && charCode <= 90 || charCode >= 48 && charCode <= 57;
    }
    function isUpperCase(charCode) {
      return charCode >= 65 && charCode <= 90;
    }
    var score = {
      [
        0
        /* CharBoundaryCategory.WordLower */
      ]: 0,
      [
        1
        /* CharBoundaryCategory.WordUpper */
      ]: 0,
      [
        2
        /* CharBoundaryCategory.WordNumber */
      ]: 0,
      [
        3
        /* CharBoundaryCategory.End */
      ]: 10,
      [
        4
        /* CharBoundaryCategory.Other */
      ]: 2,
      [
        5
        /* CharBoundaryCategory.Separator */
      ]: 30,
      [
        6
        /* CharBoundaryCategory.Space */
      ]: 3,
      [
        7
        /* CharBoundaryCategory.LineBreakCR */
      ]: 10,
      [
        8
        /* CharBoundaryCategory.LineBreakLF */
      ]: 10
    };
    function getCategoryBoundaryScore(category) {
      return score[category];
    }
    function getCategory(charCode) {
      if (charCode === 10) {
        return 8;
      } else if (charCode === 13) {
        return 7;
      } else if ((0, utils_js_1.isSpace)(charCode)) {
        return 6;
      } else if (charCode >= 97 && charCode <= 122) {
        return 0;
      } else if (charCode >= 65 && charCode <= 90) {
        return 1;
      } else if (charCode >= 48 && charCode <= 57) {
        return 2;
      } else if (charCode === -1) {
        return 3;
      } else if (charCode === 44 || charCode === 59) {
        return 5;
      } else {
        return 4;
      }
    }
  }
});

// package/dist/vs/editor/common/diff/defaultLinesDiffComputer/computeMovedLines.js
var require_computeMovedLines = __commonJS({
  "package/dist/vs/editor/common/diff/defaultLinesDiffComputer/computeMovedLines.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.computeMovedLines = computeMovedLines;
    var arrays_js_1 = require_arrays();
    var arraysFind_js_1 = require_arraysFind();
    var map_js_1 = require_map();
    var range_js_1 = require_range();
    var lineRange_js_1 = require_lineRange();
    var rangeMapping_js_1 = require_rangeMapping();
    var diffAlgorithm_js_1 = require_diffAlgorithm();
    var myersDiffAlgorithm_js_1 = require_myersDiffAlgorithm();
    var linesSliceCharSequence_js_1 = require_linesSliceCharSequence();
    var utils_js_1 = require_utils();
    function computeMovedLines(changes, originalLines, modifiedLines, hashedOriginalLines, hashedModifiedLines, timeout) {
      let { moves, excludedChanges } = computeMovesFromSimpleDeletionsToSimpleInsertions(changes, originalLines, modifiedLines, timeout);
      if (!timeout.isValid()) {
        return [];
      }
      const filteredChanges = changes.filter((c) => !excludedChanges.has(c));
      const unchangedMoves = computeUnchangedMoves(filteredChanges, hashedOriginalLines, hashedModifiedLines, originalLines, modifiedLines, timeout);
      (0, arrays_js_1.pushMany)(moves, unchangedMoves);
      moves = joinCloseConsecutiveMoves(moves);
      moves = moves.filter((current) => {
        const lines = current.original.toOffsetRange().slice(originalLines).map((l) => l.trim());
        const originalText = lines.join("\n");
        return originalText.length >= 15 && countWhere(lines, (l) => l.length >= 2) >= 2;
      });
      moves = removeMovesInSameDiff(changes, moves);
      return moves;
    }
    function countWhere(arr, predicate) {
      let count = 0;
      for (const t of arr) {
        if (predicate(t)) {
          count++;
        }
      }
      return count;
    }
    function computeMovesFromSimpleDeletionsToSimpleInsertions(changes, originalLines, modifiedLines, timeout) {
      const moves = [];
      const deletions = changes.filter((c) => c.modified.isEmpty && c.original.length >= 3).map((d) => new utils_js_1.LineRangeFragment(d.original, originalLines, d));
      const insertions = new Set(changes.filter((c) => c.original.isEmpty && c.modified.length >= 3).map((d) => new utils_js_1.LineRangeFragment(d.modified, modifiedLines, d)));
      const excludedChanges = /* @__PURE__ */ new Set();
      for (const deletion of deletions) {
        let highestSimilarity = -1;
        let best;
        for (const insertion of insertions) {
          const similarity = deletion.computeSimilarity(insertion);
          if (similarity > highestSimilarity) {
            highestSimilarity = similarity;
            best = insertion;
          }
        }
        if (highestSimilarity > 0.9 && best) {
          insertions.delete(best);
          moves.push(new rangeMapping_js_1.LineRangeMapping(deletion.range, best.range));
          excludedChanges.add(deletion.source);
          excludedChanges.add(best.source);
        }
        if (!timeout.isValid()) {
          return { moves, excludedChanges };
        }
      }
      return { moves, excludedChanges };
    }
    function computeUnchangedMoves(changes, hashedOriginalLines, hashedModifiedLines, originalLines, modifiedLines, timeout) {
      const moves = [];
      const original3LineHashes = new map_js_1.SetMap();
      for (const change of changes) {
        for (let i = change.original.startLineNumber; i < change.original.endLineNumberExclusive - 2; i++) {
          const key = `${hashedOriginalLines[i - 1]}:${hashedOriginalLines[i + 1 - 1]}:${hashedOriginalLines[i + 2 - 1]}`;
          original3LineHashes.add(key, { range: new lineRange_js_1.LineRange(i, i + 3) });
        }
      }
      const possibleMappings = [];
      changes.sort((0, arrays_js_1.compareBy)((c) => c.modified.startLineNumber, arrays_js_1.numberComparator));
      for (const change of changes) {
        let lastMappings = [];
        for (let i = change.modified.startLineNumber; i < change.modified.endLineNumberExclusive - 2; i++) {
          const key = `${hashedModifiedLines[i - 1]}:${hashedModifiedLines[i + 1 - 1]}:${hashedModifiedLines[i + 2 - 1]}`;
          const currentModifiedRange = new lineRange_js_1.LineRange(i, i + 3);
          const nextMappings = [];
          original3LineHashes.forEach(key, ({ range }) => {
            for (const lastMapping of lastMappings) {
              if (lastMapping.originalLineRange.endLineNumberExclusive + 1 === range.endLineNumberExclusive && lastMapping.modifiedLineRange.endLineNumberExclusive + 1 === currentModifiedRange.endLineNumberExclusive) {
                lastMapping.originalLineRange = new lineRange_js_1.LineRange(lastMapping.originalLineRange.startLineNumber, range.endLineNumberExclusive);
                lastMapping.modifiedLineRange = new lineRange_js_1.LineRange(lastMapping.modifiedLineRange.startLineNumber, currentModifiedRange.endLineNumberExclusive);
                nextMappings.push(lastMapping);
                return;
              }
            }
            const mapping = {
              modifiedLineRange: currentModifiedRange,
              originalLineRange: range
            };
            possibleMappings.push(mapping);
            nextMappings.push(mapping);
          });
          lastMappings = nextMappings;
        }
        if (!timeout.isValid()) {
          return [];
        }
      }
      possibleMappings.sort((0, arrays_js_1.reverseOrder)((0, arrays_js_1.compareBy)((m) => m.modifiedLineRange.length, arrays_js_1.numberComparator)));
      const modifiedSet = new lineRange_js_1.LineRangeSet();
      const originalSet = new lineRange_js_1.LineRangeSet();
      for (const mapping of possibleMappings) {
        const diffOrigToMod = mapping.modifiedLineRange.startLineNumber - mapping.originalLineRange.startLineNumber;
        const modifiedSections = modifiedSet.subtractFrom(mapping.modifiedLineRange);
        const originalTranslatedSections = originalSet.subtractFrom(mapping.originalLineRange).getWithDelta(diffOrigToMod);
        const modifiedIntersectedSections = modifiedSections.getIntersection(originalTranslatedSections);
        for (const s of modifiedIntersectedSections.ranges) {
          if (s.length < 3) {
            continue;
          }
          const modifiedLineRange = s;
          const originalLineRange = s.delta(-diffOrigToMod);
          moves.push(new rangeMapping_js_1.LineRangeMapping(originalLineRange, modifiedLineRange));
          modifiedSet.addRange(modifiedLineRange);
          originalSet.addRange(originalLineRange);
        }
      }
      moves.sort((0, arrays_js_1.compareBy)((m) => m.original.startLineNumber, arrays_js_1.numberComparator));
      const monotonousChanges = new arraysFind_js_1.MonotonousArray(changes);
      for (let i = 0; i < moves.length; i++) {
        const move = moves[i];
        const firstTouchingChangeOrig = monotonousChanges.findLastMonotonous((c) => c.original.startLineNumber <= move.original.startLineNumber);
        const firstTouchingChangeMod = (0, arraysFind_js_1.findLastMonotonous)(changes, (c) => c.modified.startLineNumber <= move.modified.startLineNumber);
        const linesAbove = Math.max(move.original.startLineNumber - firstTouchingChangeOrig.original.startLineNumber, move.modified.startLineNumber - firstTouchingChangeMod.modified.startLineNumber);
        const lastTouchingChangeOrig = monotonousChanges.findLastMonotonous((c) => c.original.startLineNumber < move.original.endLineNumberExclusive);
        const lastTouchingChangeMod = (0, arraysFind_js_1.findLastMonotonous)(changes, (c) => c.modified.startLineNumber < move.modified.endLineNumberExclusive);
        const linesBelow = Math.max(lastTouchingChangeOrig.original.endLineNumberExclusive - move.original.endLineNumberExclusive, lastTouchingChangeMod.modified.endLineNumberExclusive - move.modified.endLineNumberExclusive);
        let extendToTop;
        for (extendToTop = 0; extendToTop < linesAbove; extendToTop++) {
          const origLine = move.original.startLineNumber - extendToTop - 1;
          const modLine = move.modified.startLineNumber - extendToTop - 1;
          if (origLine > originalLines.length || modLine > modifiedLines.length) {
            break;
          }
          if (modifiedSet.contains(modLine) || originalSet.contains(origLine)) {
            break;
          }
          if (!areLinesSimilar(originalLines[origLine - 1], modifiedLines[modLine - 1], timeout)) {
            break;
          }
        }
        if (extendToTop > 0) {
          originalSet.addRange(new lineRange_js_1.LineRange(move.original.startLineNumber - extendToTop, move.original.startLineNumber));
          modifiedSet.addRange(new lineRange_js_1.LineRange(move.modified.startLineNumber - extendToTop, move.modified.startLineNumber));
        }
        let extendToBottom;
        for (extendToBottom = 0; extendToBottom < linesBelow; extendToBottom++) {
          const origLine = move.original.endLineNumberExclusive + extendToBottom;
          const modLine = move.modified.endLineNumberExclusive + extendToBottom;
          if (origLine > originalLines.length || modLine > modifiedLines.length) {
            break;
          }
          if (modifiedSet.contains(modLine) || originalSet.contains(origLine)) {
            break;
          }
          if (!areLinesSimilar(originalLines[origLine - 1], modifiedLines[modLine - 1], timeout)) {
            break;
          }
        }
        if (extendToBottom > 0) {
          originalSet.addRange(new lineRange_js_1.LineRange(move.original.endLineNumberExclusive, move.original.endLineNumberExclusive + extendToBottom));
          modifiedSet.addRange(new lineRange_js_1.LineRange(move.modified.endLineNumberExclusive, move.modified.endLineNumberExclusive + extendToBottom));
        }
        if (extendToTop > 0 || extendToBottom > 0) {
          moves[i] = new rangeMapping_js_1.LineRangeMapping(new lineRange_js_1.LineRange(move.original.startLineNumber - extendToTop, move.original.endLineNumberExclusive + extendToBottom), new lineRange_js_1.LineRange(move.modified.startLineNumber - extendToTop, move.modified.endLineNumberExclusive + extendToBottom));
        }
      }
      return moves;
    }
    function areLinesSimilar(line1, line2, timeout) {
      if (line1.trim() === line2.trim()) {
        return true;
      }
      if (line1.length > 300 && line2.length > 300) {
        return false;
      }
      const myersDiffingAlgorithm = new myersDiffAlgorithm_js_1.MyersDiffAlgorithm();
      const result = myersDiffingAlgorithm.compute(new linesSliceCharSequence_js_1.LinesSliceCharSequence([line1], new range_js_1.Range(1, 1, 1, line1.length), false), new linesSliceCharSequence_js_1.LinesSliceCharSequence([line2], new range_js_1.Range(1, 1, 1, line2.length), false), timeout);
      let commonNonSpaceCharCount = 0;
      const inverted = diffAlgorithm_js_1.SequenceDiff.invert(result.diffs, line1.length);
      for (const seq of inverted) {
        seq.seq1Range.forEach((idx) => {
          if (!(0, utils_js_1.isSpace)(line1.charCodeAt(idx))) {
            commonNonSpaceCharCount++;
          }
        });
      }
      function countNonWsChars(str) {
        let count = 0;
        for (let i = 0; i < line1.length; i++) {
          if (!(0, utils_js_1.isSpace)(str.charCodeAt(i))) {
            count++;
          }
        }
        return count;
      }
      const longerLineLength = countNonWsChars(line1.length > line2.length ? line1 : line2);
      const r = commonNonSpaceCharCount / longerLineLength > 0.6 && longerLineLength > 10;
      return r;
    }
    function joinCloseConsecutiveMoves(moves) {
      if (moves.length === 0) {
        return moves;
      }
      moves.sort((0, arrays_js_1.compareBy)((m) => m.original.startLineNumber, arrays_js_1.numberComparator));
      const result = [moves[0]];
      for (let i = 1; i < moves.length; i++) {
        const last = result[result.length - 1];
        const current = moves[i];
        const originalDist = current.original.startLineNumber - last.original.endLineNumberExclusive;
        const modifiedDist = current.modified.startLineNumber - last.modified.endLineNumberExclusive;
        const currentMoveAfterLast = originalDist >= 0 && modifiedDist >= 0;
        if (currentMoveAfterLast && originalDist + modifiedDist <= 2) {
          result[result.length - 1] = last.join(current);
          continue;
        }
        result.push(current);
      }
      return result;
    }
    function removeMovesInSameDiff(changes, moves) {
      const changesMonotonous = new arraysFind_js_1.MonotonousArray(changes);
      moves = moves.filter((m) => {
        const diffBeforeEndOfMoveOriginal = changesMonotonous.findLastMonotonous((c) => c.original.startLineNumber < m.original.endLineNumberExclusive) || new rangeMapping_js_1.LineRangeMapping(new lineRange_js_1.LineRange(1, 1), new lineRange_js_1.LineRange(1, 1));
        const diffBeforeEndOfMoveModified = (0, arraysFind_js_1.findLastMonotonous)(changes, (c) => c.modified.startLineNumber < m.modified.endLineNumberExclusive);
        const differentDiffs = diffBeforeEndOfMoveOriginal !== diffBeforeEndOfMoveModified;
        return differentDiffs;
      });
      return moves;
    }
  }
});

// package/dist/vs/editor/common/diff/defaultLinesDiffComputer/heuristicSequenceOptimizations.js
var require_heuristicSequenceOptimizations = __commonJS({
  "package/dist/vs/editor/common/diff/defaultLinesDiffComputer/heuristicSequenceOptimizations.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.optimizeSequenceDiffs = optimizeSequenceDiffs;
    exports.removeShortMatches = removeShortMatches;
    exports.extendDiffsToEntireWordIfAppropriate = extendDiffsToEntireWordIfAppropriate;
    exports.removeVeryShortMatchingLinesBetweenDiffs = removeVeryShortMatchingLinesBetweenDiffs;
    exports.removeVeryShortMatchingTextBetweenLongDiffs = removeVeryShortMatchingTextBetweenLongDiffs;
    var arrays_js_1 = require_arrays();
    var offsetRange_js_1 = require_offsetRange();
    var diffAlgorithm_js_1 = require_diffAlgorithm();
    function optimizeSequenceDiffs(sequence1, sequence2, sequenceDiffs) {
      let result = sequenceDiffs;
      result = joinSequenceDiffsByShifting(sequence1, sequence2, result);
      result = joinSequenceDiffsByShifting(sequence1, sequence2, result);
      result = shiftSequenceDiffs(sequence1, sequence2, result);
      return result;
    }
    function joinSequenceDiffsByShifting(sequence1, sequence2, sequenceDiffs) {
      if (sequenceDiffs.length === 0) {
        return sequenceDiffs;
      }
      const result = [];
      result.push(sequenceDiffs[0]);
      for (let i = 1; i < sequenceDiffs.length; i++) {
        const prevResult = result[result.length - 1];
        let cur = sequenceDiffs[i];
        if (cur.seq1Range.isEmpty || cur.seq2Range.isEmpty) {
          const length = cur.seq1Range.start - prevResult.seq1Range.endExclusive;
          let d;
          for (d = 1; d <= length; d++) {
            if (sequence1.getElement(cur.seq1Range.start - d) !== sequence1.getElement(cur.seq1Range.endExclusive - d) || sequence2.getElement(cur.seq2Range.start - d) !== sequence2.getElement(cur.seq2Range.endExclusive - d)) {
              break;
            }
          }
          d--;
          if (d === length) {
            result[result.length - 1] = new diffAlgorithm_js_1.SequenceDiff(new offsetRange_js_1.OffsetRange(prevResult.seq1Range.start, cur.seq1Range.endExclusive - length), new offsetRange_js_1.OffsetRange(prevResult.seq2Range.start, cur.seq2Range.endExclusive - length));
            continue;
          }
          cur = cur.delta(-d);
        }
        result.push(cur);
      }
      const result2 = [];
      for (let i = 0; i < result.length - 1; i++) {
        const nextResult = result[i + 1];
        let cur = result[i];
        if (cur.seq1Range.isEmpty || cur.seq2Range.isEmpty) {
          const length = nextResult.seq1Range.start - cur.seq1Range.endExclusive;
          let d;
          for (d = 0; d < length; d++) {
            if (!sequence1.isStronglyEqual(cur.seq1Range.start + d, cur.seq1Range.endExclusive + d) || !sequence2.isStronglyEqual(cur.seq2Range.start + d, cur.seq2Range.endExclusive + d)) {
              break;
            }
          }
          if (d === length) {
            result[i + 1] = new diffAlgorithm_js_1.SequenceDiff(new offsetRange_js_1.OffsetRange(cur.seq1Range.start + length, nextResult.seq1Range.endExclusive), new offsetRange_js_1.OffsetRange(cur.seq2Range.start + length, nextResult.seq2Range.endExclusive));
            continue;
          }
          if (d > 0) {
            cur = cur.delta(d);
          }
        }
        result2.push(cur);
      }
      if (result.length > 0) {
        result2.push(result[result.length - 1]);
      }
      return result2;
    }
    function shiftSequenceDiffs(sequence1, sequence2, sequenceDiffs) {
      if (!sequence1.getBoundaryScore || !sequence2.getBoundaryScore) {
        return sequenceDiffs;
      }
      for (let i = 0; i < sequenceDiffs.length; i++) {
        const prevDiff = i > 0 ? sequenceDiffs[i - 1] : void 0;
        const diff = sequenceDiffs[i];
        const nextDiff = i + 1 < sequenceDiffs.length ? sequenceDiffs[i + 1] : void 0;
        const seq1ValidRange = new offsetRange_js_1.OffsetRange(prevDiff ? prevDiff.seq1Range.endExclusive + 1 : 0, nextDiff ? nextDiff.seq1Range.start - 1 : sequence1.length);
        const seq2ValidRange = new offsetRange_js_1.OffsetRange(prevDiff ? prevDiff.seq2Range.endExclusive + 1 : 0, nextDiff ? nextDiff.seq2Range.start - 1 : sequence2.length);
        if (diff.seq1Range.isEmpty) {
          sequenceDiffs[i] = shiftDiffToBetterPosition(diff, sequence1, sequence2, seq1ValidRange, seq2ValidRange);
        } else if (diff.seq2Range.isEmpty) {
          sequenceDiffs[i] = shiftDiffToBetterPosition(diff.swap(), sequence2, sequence1, seq2ValidRange, seq1ValidRange).swap();
        }
      }
      return sequenceDiffs;
    }
    function shiftDiffToBetterPosition(diff, sequence1, sequence2, seq1ValidRange, seq2ValidRange) {
      const maxShiftLimit = 100;
      let deltaBefore = 1;
      while (diff.seq1Range.start - deltaBefore >= seq1ValidRange.start && diff.seq2Range.start - deltaBefore >= seq2ValidRange.start && sequence2.isStronglyEqual(diff.seq2Range.start - deltaBefore, diff.seq2Range.endExclusive - deltaBefore) && deltaBefore < maxShiftLimit) {
        deltaBefore++;
      }
      deltaBefore--;
      let deltaAfter = 0;
      while (diff.seq1Range.start + deltaAfter < seq1ValidRange.endExclusive && diff.seq2Range.endExclusive + deltaAfter < seq2ValidRange.endExclusive && sequence2.isStronglyEqual(diff.seq2Range.start + deltaAfter, diff.seq2Range.endExclusive + deltaAfter) && deltaAfter < maxShiftLimit) {
        deltaAfter++;
      }
      if (deltaBefore === 0 && deltaAfter === 0) {
        return diff;
      }
      let bestDelta = 0;
      let bestScore = -1;
      for (let delta = -deltaBefore; delta <= deltaAfter; delta++) {
        const seq2OffsetStart = diff.seq2Range.start + delta;
        const seq2OffsetEndExclusive = diff.seq2Range.endExclusive + delta;
        const seq1Offset = diff.seq1Range.start + delta;
        const score = sequence1.getBoundaryScore(seq1Offset) + sequence2.getBoundaryScore(seq2OffsetStart) + sequence2.getBoundaryScore(seq2OffsetEndExclusive);
        if (score > bestScore) {
          bestScore = score;
          bestDelta = delta;
        }
      }
      return diff.delta(bestDelta);
    }
    function removeShortMatches(sequence1, sequence2, sequenceDiffs) {
      const result = [];
      for (const s of sequenceDiffs) {
        const last = result[result.length - 1];
        if (!last) {
          result.push(s);
          continue;
        }
        if (s.seq1Range.start - last.seq1Range.endExclusive <= 2 || s.seq2Range.start - last.seq2Range.endExclusive <= 2) {
          result[result.length - 1] = new diffAlgorithm_js_1.SequenceDiff(last.seq1Range.join(s.seq1Range), last.seq2Range.join(s.seq2Range));
        } else {
          result.push(s);
        }
      }
      return result;
    }
    function extendDiffsToEntireWordIfAppropriate(sequence1, sequence2, sequenceDiffs, findParent, force = false) {
      const equalMappings = diffAlgorithm_js_1.SequenceDiff.invert(sequenceDiffs, sequence1.length);
      const additional = [];
      let lastPoint = new diffAlgorithm_js_1.OffsetPair(0, 0);
      function scanWord(pair, equalMapping) {
        if (pair.offset1 < lastPoint.offset1 || pair.offset2 < lastPoint.offset2) {
          return;
        }
        const w1 = findParent(sequence1, pair.offset1);
        const w2 = findParent(sequence2, pair.offset2);
        if (!w1 || !w2) {
          return;
        }
        let w = new diffAlgorithm_js_1.SequenceDiff(w1, w2);
        const equalPart = w.intersect(equalMapping);
        let equalChars1 = equalPart.seq1Range.length;
        let equalChars2 = equalPart.seq2Range.length;
        while (equalMappings.length > 0) {
          const next = equalMappings[0];
          const intersects = next.seq1Range.intersects(w.seq1Range) || next.seq2Range.intersects(w.seq2Range);
          if (!intersects) {
            break;
          }
          const v1 = findParent(sequence1, next.seq1Range.start);
          const v2 = findParent(sequence2, next.seq2Range.start);
          const v = new diffAlgorithm_js_1.SequenceDiff(v1, v2);
          const equalPart2 = v.intersect(next);
          equalChars1 += equalPart2.seq1Range.length;
          equalChars2 += equalPart2.seq2Range.length;
          w = w.join(v);
          if (w.seq1Range.endExclusive >= next.seq1Range.endExclusive) {
            equalMappings.shift();
          } else {
            break;
          }
        }
        if (force && equalChars1 + equalChars2 < w.seq1Range.length + w.seq2Range.length || equalChars1 + equalChars2 < (w.seq1Range.length + w.seq2Range.length) * 2 / 3) {
          additional.push(w);
        }
        lastPoint = w.getEndExclusives();
      }
      while (equalMappings.length > 0) {
        const next = equalMappings.shift();
        if (next.seq1Range.isEmpty) {
          continue;
        }
        scanWord(next.getStarts(), next);
        scanWord(next.getEndExclusives().delta(-1), next);
      }
      const merged = mergeSequenceDiffs(sequenceDiffs, additional);
      return merged;
    }
    function mergeSequenceDiffs(sequenceDiffs1, sequenceDiffs2) {
      const result = [];
      while (sequenceDiffs1.length > 0 || sequenceDiffs2.length > 0) {
        const sd1 = sequenceDiffs1[0];
        const sd2 = sequenceDiffs2[0];
        let next;
        if (sd1 && (!sd2 || sd1.seq1Range.start < sd2.seq1Range.start)) {
          next = sequenceDiffs1.shift();
        } else {
          next = sequenceDiffs2.shift();
        }
        if (result.length > 0 && result[result.length - 1].seq1Range.endExclusive >= next.seq1Range.start) {
          result[result.length - 1] = result[result.length - 1].join(next);
        } else {
          result.push(next);
        }
      }
      return result;
    }
    function removeVeryShortMatchingLinesBetweenDiffs(sequence1, _sequence2, sequenceDiffs) {
      let diffs = sequenceDiffs;
      if (diffs.length === 0) {
        return diffs;
      }
      let counter = 0;
      let shouldRepeat;
      do {
        shouldRepeat = false;
        const result = [
          diffs[0]
        ];
        for (let i = 1; i < diffs.length; i++) {
          let shouldJoinDiffs = function(before, after) {
            const unchangedRange = new offsetRange_js_1.OffsetRange(lastResult.seq1Range.endExclusive, cur.seq1Range.start);
            const unchangedText = sequence1.getText(unchangedRange);
            const unchangedTextWithoutWs = unchangedText.replace(/\s/g, "");
            if (unchangedTextWithoutWs.length <= 4 && (before.seq1Range.length + before.seq2Range.length > 5 || after.seq1Range.length + after.seq2Range.length > 5)) {
              return true;
            }
            return false;
          };
          const cur = diffs[i];
          const lastResult = result[result.length - 1];
          const shouldJoin = shouldJoinDiffs(lastResult, cur);
          if (shouldJoin) {
            shouldRepeat = true;
            result[result.length - 1] = result[result.length - 1].join(cur);
          } else {
            result.push(cur);
          }
        }
        diffs = result;
      } while (counter++ < 10 && shouldRepeat);
      return diffs;
    }
    function removeVeryShortMatchingTextBetweenLongDiffs(sequence1, sequence2, sequenceDiffs) {
      let diffs = sequenceDiffs;
      if (diffs.length === 0) {
        return diffs;
      }
      let counter = 0;
      let shouldRepeat;
      do {
        shouldRepeat = false;
        const result = [
          diffs[0]
        ];
        for (let i = 1; i < diffs.length; i++) {
          let shouldJoinDiffs = function(before, after) {
            const unchangedRange = new offsetRange_js_1.OffsetRange(lastResult.seq1Range.endExclusive, cur.seq1Range.start);
            const unchangedLineCount = sequence1.countLinesIn(unchangedRange);
            if (unchangedLineCount > 5 || unchangedRange.length > 500) {
              return false;
            }
            const unchangedText = sequence1.getText(unchangedRange).trim();
            if (unchangedText.length > 20 || unchangedText.split(/\r\n|\r|\n/).length > 1) {
              return false;
            }
            const beforeLineCount1 = sequence1.countLinesIn(before.seq1Range);
            const beforeSeq1Length = before.seq1Range.length;
            const beforeLineCount2 = sequence2.countLinesIn(before.seq2Range);
            const beforeSeq2Length = before.seq2Range.length;
            const afterLineCount1 = sequence1.countLinesIn(after.seq1Range);
            const afterSeq1Length = after.seq1Range.length;
            const afterLineCount2 = sequence2.countLinesIn(after.seq2Range);
            const afterSeq2Length = after.seq2Range.length;
            const max = 2 * 40 + 50;
            function cap(v) {
              return Math.min(v, max);
            }
            if (Math.pow(Math.pow(cap(beforeLineCount1 * 40 + beforeSeq1Length), 1.5) + Math.pow(cap(beforeLineCount2 * 40 + beforeSeq2Length), 1.5), 1.5) + Math.pow(Math.pow(cap(afterLineCount1 * 40 + afterSeq1Length), 1.5) + Math.pow(cap(afterLineCount2 * 40 + afterSeq2Length), 1.5), 1.5) > (max ** 1.5) ** 1.5 * 1.3) {
              return true;
            }
            return false;
          };
          const cur = diffs[i];
          const lastResult = result[result.length - 1];
          const shouldJoin = shouldJoinDiffs(lastResult, cur);
          if (shouldJoin) {
            shouldRepeat = true;
            result[result.length - 1] = result[result.length - 1].join(cur);
          } else {
            result.push(cur);
          }
        }
        diffs = result;
      } while (counter++ < 10 && shouldRepeat);
      const newDiffs = [];
      (0, arrays_js_1.forEachWithNeighbors)(diffs, (prev, cur, next) => {
        let newDiff = cur;
        function shouldMarkAsChanged(text) {
          return text.length > 0 && text.trim().length <= 3 && cur.seq1Range.length + cur.seq2Range.length > 100;
        }
        const fullRange1 = sequence1.extendToFullLines(cur.seq1Range);
        const prefix = sequence1.getText(new offsetRange_js_1.OffsetRange(fullRange1.start, cur.seq1Range.start));
        if (shouldMarkAsChanged(prefix)) {
          newDiff = newDiff.deltaStart(-prefix.length);
        }
        const suffix = sequence1.getText(new offsetRange_js_1.OffsetRange(cur.seq1Range.endExclusive, fullRange1.endExclusive));
        if (shouldMarkAsChanged(suffix)) {
          newDiff = newDiff.deltaEnd(suffix.length);
        }
        const availableSpace = diffAlgorithm_js_1.SequenceDiff.fromOffsetPairs(prev ? prev.getEndExclusives() : diffAlgorithm_js_1.OffsetPair.zero, next ? next.getStarts() : diffAlgorithm_js_1.OffsetPair.max);
        const result = newDiff.intersect(availableSpace);
        if (newDiffs.length > 0 && result.getStarts().equals(newDiffs[newDiffs.length - 1].getEndExclusives())) {
          newDiffs[newDiffs.length - 1] = newDiffs[newDiffs.length - 1].join(result);
        } else {
          newDiffs.push(result);
        }
      });
      return newDiffs;
    }
  }
});

// package/dist/vs/editor/common/diff/defaultLinesDiffComputer/lineSequence.js
var require_lineSequence = __commonJS({
  "package/dist/vs/editor/common/diff/defaultLinesDiffComputer/lineSequence.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.LineSequence = void 0;
    var LineSequence = class {
      trimmedHash;
      lines;
      constructor(trimmedHash, lines) {
        this.trimmedHash = trimmedHash;
        this.lines = lines;
      }
      getElement(offset) {
        return this.trimmedHash[offset];
      }
      get length() {
        return this.trimmedHash.length;
      }
      getBoundaryScore(length) {
        const indentationBefore = length === 0 ? 0 : getIndentation(this.lines[length - 1]);
        const indentationAfter = length === this.lines.length ? 0 : getIndentation(this.lines[length]);
        return 1e3 - (indentationBefore + indentationAfter);
      }
      getText(range) {
        return this.lines.slice(range.start, range.endExclusive).join("\n");
      }
      isStronglyEqual(offset1, offset2) {
        return this.lines[offset1] === this.lines[offset2];
      }
    };
    exports.LineSequence = LineSequence;
    function getIndentation(str) {
      let i = 0;
      while (i < str.length && (str.charCodeAt(i) === 32 || str.charCodeAt(i) === 9)) {
        i++;
      }
      return i;
    }
  }
});

// package/dist/vs/editor/common/diff/defaultLinesDiffComputer/defaultLinesDiffComputer.js
var require_defaultLinesDiffComputer = __commonJS({
  "package/dist/vs/editor/common/diff/defaultLinesDiffComputer/defaultLinesDiffComputer.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.DefaultLinesDiffComputer = void 0;
    var arrays_js_1 = require_arrays();
    var assert_js_1 = require_assert();
    var range_js_1 = require_range();
    var lineRange_js_1 = require_lineRange();
    var offsetRange_js_1 = require_offsetRange();
    var abstractText_js_1 = require_abstractText();
    var linesDiffComputer_js_1 = require_linesDiffComputer();
    var rangeMapping_js_1 = require_rangeMapping();
    var diffAlgorithm_js_1 = require_diffAlgorithm();
    var dynamicProgrammingDiffing_js_1 = require_dynamicProgrammingDiffing();
    var myersDiffAlgorithm_js_1 = require_myersDiffAlgorithm();
    var computeMovedLines_js_1 = require_computeMovedLines();
    var heuristicSequenceOptimizations_js_1 = require_heuristicSequenceOptimizations();
    var lineSequence_js_1 = require_lineSequence();
    var linesSliceCharSequence_js_1 = require_linesSliceCharSequence();
    var DefaultLinesDiffComputer2 = class {
      dynamicProgrammingDiffing = new dynamicProgrammingDiffing_js_1.DynamicProgrammingDiffing();
      myersDiffingAlgorithm = new myersDiffAlgorithm_js_1.MyersDiffAlgorithm();
      computeDiff(originalLines, modifiedLines, options) {
        if (originalLines.length <= 1 && (0, arrays_js_1.equals)(originalLines, modifiedLines, (a, b) => a === b)) {
          return new linesDiffComputer_js_1.LinesDiff([], [], false);
        }
        if (originalLines.length === 1 && originalLines[0].length === 0 || modifiedLines.length === 1 && modifiedLines[0].length === 0) {
          return new linesDiffComputer_js_1.LinesDiff([
            new rangeMapping_js_1.DetailedLineRangeMapping(new lineRange_js_1.LineRange(1, originalLines.length + 1), new lineRange_js_1.LineRange(1, modifiedLines.length + 1), [
              new rangeMapping_js_1.RangeMapping(new range_js_1.Range(1, 1, originalLines.length, originalLines[originalLines.length - 1].length + 1), new range_js_1.Range(1, 1, modifiedLines.length, modifiedLines[modifiedLines.length - 1].length + 1))
            ])
          ], [], false);
        }
        const timeout = options.maxComputationTimeMs === 0 ? diffAlgorithm_js_1.InfiniteTimeout.instance : new diffAlgorithm_js_1.DateTimeout(options.maxComputationTimeMs);
        const considerWhitespaceChanges = !options.ignoreTrimWhitespace;
        const perfectHashes = /* @__PURE__ */ new Map();
        function getOrCreateHash(text) {
          let hash = perfectHashes.get(text);
          if (hash === void 0) {
            hash = perfectHashes.size;
            perfectHashes.set(text, hash);
          }
          return hash;
        }
        const originalLinesHashes = originalLines.map((l) => getOrCreateHash(l.trim()));
        const modifiedLinesHashes = modifiedLines.map((l) => getOrCreateHash(l.trim()));
        const sequence1 = new lineSequence_js_1.LineSequence(originalLinesHashes, originalLines);
        const sequence2 = new lineSequence_js_1.LineSequence(modifiedLinesHashes, modifiedLines);
        const lineAlignmentResult = (() => {
          if (sequence1.length + sequence2.length < 1700) {
            return this.dynamicProgrammingDiffing.compute(sequence1, sequence2, timeout, (offset1, offset2) => originalLines[offset1] === modifiedLines[offset2] ? modifiedLines[offset2].length === 0 ? 0.1 : 1 + Math.log(1 + modifiedLines[offset2].length) : 0.99);
          }
          return this.myersDiffingAlgorithm.compute(sequence1, sequence2, timeout);
        })();
        let lineAlignments = lineAlignmentResult.diffs;
        let hitTimeout = lineAlignmentResult.hitTimeout;
        lineAlignments = (0, heuristicSequenceOptimizations_js_1.optimizeSequenceDiffs)(sequence1, sequence2, lineAlignments);
        lineAlignments = (0, heuristicSequenceOptimizations_js_1.removeVeryShortMatchingLinesBetweenDiffs)(sequence1, sequence2, lineAlignments);
        const alignments = [];
        const scanForWhitespaceChanges = (equalLinesCount) => {
          if (!considerWhitespaceChanges) {
            return;
          }
          for (let i = 0; i < equalLinesCount; i++) {
            const seq1Offset = seq1LastStart + i;
            const seq2Offset = seq2LastStart + i;
            if (originalLines[seq1Offset] !== modifiedLines[seq2Offset]) {
              const characterDiffs = this.refineDiff(originalLines, modifiedLines, new diffAlgorithm_js_1.SequenceDiff(new offsetRange_js_1.OffsetRange(seq1Offset, seq1Offset + 1), new offsetRange_js_1.OffsetRange(seq2Offset, seq2Offset + 1)), timeout, considerWhitespaceChanges, options);
              for (const a of characterDiffs.mappings) {
                alignments.push(a);
              }
              if (characterDiffs.hitTimeout) {
                hitTimeout = true;
              }
            }
          }
        };
        let seq1LastStart = 0;
        let seq2LastStart = 0;
        for (const diff of lineAlignments) {
          (0, assert_js_1.assertFn)(() => diff.seq1Range.start - seq1LastStart === diff.seq2Range.start - seq2LastStart);
          const equalLinesCount = diff.seq1Range.start - seq1LastStart;
          scanForWhitespaceChanges(equalLinesCount);
          seq1LastStart = diff.seq1Range.endExclusive;
          seq2LastStart = diff.seq2Range.endExclusive;
          const characterDiffs = this.refineDiff(originalLines, modifiedLines, diff, timeout, considerWhitespaceChanges, options);
          if (characterDiffs.hitTimeout) {
            hitTimeout = true;
          }
          for (const a of characterDiffs.mappings) {
            alignments.push(a);
          }
        }
        scanForWhitespaceChanges(originalLines.length - seq1LastStart);
        const original = new abstractText_js_1.ArrayText(originalLines);
        const modified = new abstractText_js_1.ArrayText(modifiedLines);
        const changes = (0, rangeMapping_js_1.lineRangeMappingFromRangeMappings)(alignments, original, modified);
        let moves = [];
        if (options.computeMoves) {
          moves = this.computeMoves(changes, originalLines, modifiedLines, originalLinesHashes, modifiedLinesHashes, timeout, considerWhitespaceChanges, options);
        }
        (0, assert_js_1.assertFn)(() => {
          function validatePosition(pos, lines) {
            if (pos.lineNumber < 1 || pos.lineNumber > lines.length) {
              return false;
            }
            const line = lines[pos.lineNumber - 1];
            if (pos.column < 1 || pos.column > line.length + 1) {
              return false;
            }
            return true;
          }
          function validateRange(range, lines) {
            if (range.startLineNumber < 1 || range.startLineNumber > lines.length + 1) {
              return false;
            }
            if (range.endLineNumberExclusive < 1 || range.endLineNumberExclusive > lines.length + 1) {
              return false;
            }
            return true;
          }
          for (const c of changes) {
            if (!c.innerChanges) {
              return false;
            }
            for (const ic of c.innerChanges) {
              const valid = validatePosition(ic.modifiedRange.getStartPosition(), modifiedLines) && validatePosition(ic.modifiedRange.getEndPosition(), modifiedLines) && validatePosition(ic.originalRange.getStartPosition(), originalLines) && validatePosition(ic.originalRange.getEndPosition(), originalLines);
              if (!valid) {
                return false;
              }
            }
            if (!validateRange(c.modified, modifiedLines) || !validateRange(c.original, originalLines)) {
              return false;
            }
          }
          return true;
        });
        return new linesDiffComputer_js_1.LinesDiff(changes, moves, hitTimeout);
      }
      computeMoves(changes, originalLines, modifiedLines, hashedOriginalLines, hashedModifiedLines, timeout, considerWhitespaceChanges, options) {
        const moves = (0, computeMovedLines_js_1.computeMovedLines)(changes, originalLines, modifiedLines, hashedOriginalLines, hashedModifiedLines, timeout);
        const movesWithDiffs = moves.map((m) => {
          const moveChanges = this.refineDiff(originalLines, modifiedLines, new diffAlgorithm_js_1.SequenceDiff(m.original.toOffsetRange(), m.modified.toOffsetRange()), timeout, considerWhitespaceChanges, options);
          const mappings = (0, rangeMapping_js_1.lineRangeMappingFromRangeMappings)(moveChanges.mappings, new abstractText_js_1.ArrayText(originalLines), new abstractText_js_1.ArrayText(modifiedLines), true);
          return new linesDiffComputer_js_1.MovedText(m, mappings);
        });
        return movesWithDiffs;
      }
      refineDiff(originalLines, modifiedLines, diff, timeout, considerWhitespaceChanges, options) {
        const lineRangeMapping = toLineRangeMapping(diff);
        const rangeMapping = lineRangeMapping.toRangeMapping2(originalLines, modifiedLines);
        const slice1 = new linesSliceCharSequence_js_1.LinesSliceCharSequence(originalLines, rangeMapping.originalRange, considerWhitespaceChanges);
        const slice2 = new linesSliceCharSequence_js_1.LinesSliceCharSequence(modifiedLines, rangeMapping.modifiedRange, considerWhitespaceChanges);
        const diffResult = slice1.length + slice2.length < 500 ? this.dynamicProgrammingDiffing.compute(slice1, slice2, timeout) : this.myersDiffingAlgorithm.compute(slice1, slice2, timeout);
        const check = false;
        let diffs = diffResult.diffs;
        if (check) {
          diffAlgorithm_js_1.SequenceDiff.assertSorted(diffs);
        }
        diffs = (0, heuristicSequenceOptimizations_js_1.optimizeSequenceDiffs)(slice1, slice2, diffs);
        if (check) {
          diffAlgorithm_js_1.SequenceDiff.assertSorted(diffs);
        }
        diffs = (0, heuristicSequenceOptimizations_js_1.extendDiffsToEntireWordIfAppropriate)(slice1, slice2, diffs, (seq, idx) => seq.findWordContaining(idx));
        if (check) {
          diffAlgorithm_js_1.SequenceDiff.assertSorted(diffs);
        }
        if (options.extendToSubwords) {
          diffs = (0, heuristicSequenceOptimizations_js_1.extendDiffsToEntireWordIfAppropriate)(slice1, slice2, diffs, (seq, idx) => seq.findSubWordContaining(idx), true);
          if (check) {
            diffAlgorithm_js_1.SequenceDiff.assertSorted(diffs);
          }
        }
        diffs = (0, heuristicSequenceOptimizations_js_1.removeShortMatches)(slice1, slice2, diffs);
        if (check) {
          diffAlgorithm_js_1.SequenceDiff.assertSorted(diffs);
        }
        diffs = (0, heuristicSequenceOptimizations_js_1.removeVeryShortMatchingTextBetweenLongDiffs)(slice1, slice2, diffs);
        if (check) {
          diffAlgorithm_js_1.SequenceDiff.assertSorted(diffs);
        }
        const result = diffs.map((d) => new rangeMapping_js_1.RangeMapping(slice1.translateRange(d.seq1Range), slice2.translateRange(d.seq2Range)));
        if (check) {
          rangeMapping_js_1.RangeMapping.assertSorted(result);
        }
        return {
          mappings: result,
          hitTimeout: diffResult.hitTimeout
        };
      }
    };
    exports.DefaultLinesDiffComputer = DefaultLinesDiffComputer2;
    function toLineRangeMapping(sequenceDiff) {
      return new rangeMapping_js_1.LineRangeMapping(new lineRange_js_1.LineRange(sequenceDiff.seq1Range.start + 1, sequenceDiff.seq1Range.endExclusive + 1), new lineRange_js_1.LineRange(sequenceDiff.seq2Range.start + 1, sequenceDiff.seq2Range.endExclusive + 1));
    }
  }
});

// package/dist/vs/base/common/strings.js
var require_strings = __commonJS({
  "package/dist/vs/base/common/strings.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.firstNonWhitespaceIndex = firstNonWhitespaceIndex;
    exports.lastNonWhitespaceIndex = lastNonWhitespaceIndex;
    function firstNonWhitespaceIndex(str) {
      for (let i = 0, len = str.length; i < len; i++) {
        const chCode = str.charCodeAt(i);
        if (chCode !== 32 && chCode !== 9) {
          return i;
        }
      }
      return -1;
    }
    function lastNonWhitespaceIndex(str, startIndex = str.length - 1) {
      for (let i = startIndex; i >= 0; i--) {
        const chCode = str.charCodeAt(i);
        if (chCode !== 32 && chCode !== 9) {
          return i;
        }
      }
      return -1;
    }
  }
});

// package/dist/vs/editor/common/diff/legacyLinesDiffComputer.js
var require_legacyLinesDiffComputer = __commonJS({
  "package/dist/vs/editor/common/diff/legacyLinesDiffComputer.js"(exports) {
    "use strict";
    var __createBinding = exports && exports.__createBinding || (Object.create ? (function(o, m, k, k2) {
      if (k2 === void 0) k2 = k;
      var desc = Object.getOwnPropertyDescriptor(m, k);
      if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
        desc = { enumerable: true, get: function() {
          return m[k];
        } };
      }
      Object.defineProperty(o, k2, desc);
    }) : (function(o, m, k, k2) {
      if (k2 === void 0) k2 = k;
      o[k2] = m[k];
    }));
    var __setModuleDefault = exports && exports.__setModuleDefault || (Object.create ? (function(o, v) {
      Object.defineProperty(o, "default", { enumerable: true, value: v });
    }) : function(o, v) {
      o["default"] = v;
    });
    var __importStar = exports && exports.__importStar || /* @__PURE__ */ (function() {
      var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function(o2) {
          var ar = [];
          for (var k in o2) if (Object.prototype.hasOwnProperty.call(o2, k)) ar[ar.length] = k;
          return ar;
        };
        return ownKeys(o);
      };
      return function(mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) {
          for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        }
        __setModuleDefault(result, mod);
        return result;
      };
    })();
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.DiffComputer = exports.LegacyLinesDiffComputer = void 0;
    var assert_js_1 = require_assert();
    var diff_js_1 = require_diff();
    var strings = __importStar(require_strings());
    var range_js_1 = require_range();
    var lineRange_js_1 = require_lineRange();
    var linesDiffComputer_js_1 = require_linesDiffComputer();
    var rangeMapping_js_1 = require_rangeMapping();
    var MINIMUM_MATCHING_CHARACTER_LENGTH = 3;
    var LegacyLinesDiffComputer2 = class {
      computeDiff(originalLines, modifiedLines, options) {
        const diffComputer = new DiffComputer2(originalLines, modifiedLines, {
          maxComputationTime: options.maxComputationTimeMs,
          shouldIgnoreTrimWhitespace: options.ignoreTrimWhitespace,
          shouldComputeCharChanges: true,
          shouldMakePrettyDiff: true,
          shouldPostProcessCharChanges: true
        });
        const result = diffComputer.computeDiff();
        const changes = [];
        let lastChange = null;
        for (const c of result.changes) {
          let originalRange;
          if (c.originalEndLineNumber === 0) {
            originalRange = new lineRange_js_1.LineRange(c.originalStartLineNumber + 1, c.originalStartLineNumber + 1);
          } else {
            originalRange = new lineRange_js_1.LineRange(c.originalStartLineNumber, c.originalEndLineNumber + 1);
          }
          let modifiedRange;
          if (c.modifiedEndLineNumber === 0) {
            modifiedRange = new lineRange_js_1.LineRange(c.modifiedStartLineNumber + 1, c.modifiedStartLineNumber + 1);
          } else {
            modifiedRange = new lineRange_js_1.LineRange(c.modifiedStartLineNumber, c.modifiedEndLineNumber + 1);
          }
          let change = new rangeMapping_js_1.DetailedLineRangeMapping(originalRange, modifiedRange, c.charChanges?.map((c2) => new rangeMapping_js_1.RangeMapping(new range_js_1.Range(c2.originalStartLineNumber, c2.originalStartColumn, c2.originalEndLineNumber, c2.originalEndColumn), new range_js_1.Range(c2.modifiedStartLineNumber, c2.modifiedStartColumn, c2.modifiedEndLineNumber, c2.modifiedEndColumn))));
          if (lastChange) {
            if (lastChange.modified.endLineNumberExclusive === change.modified.startLineNumber || lastChange.original.endLineNumberExclusive === change.original.startLineNumber) {
              change = new rangeMapping_js_1.DetailedLineRangeMapping(lastChange.original.join(change.original), lastChange.modified.join(change.modified), lastChange.innerChanges && change.innerChanges ? lastChange.innerChanges.concat(change.innerChanges) : void 0);
              changes.pop();
            }
          }
          changes.push(change);
          lastChange = change;
        }
        (0, assert_js_1.assertFn)(() => {
          return (0, assert_js_1.checkAdjacentItems)(changes, (m1, m2) => m2.original.startLineNumber - m1.original.endLineNumberExclusive === m2.modified.startLineNumber - m1.modified.endLineNumberExclusive && // There has to be an unchanged line in between (otherwise both diffs should have been joined)
          m1.original.endLineNumberExclusive < m2.original.startLineNumber && m1.modified.endLineNumberExclusive < m2.modified.startLineNumber);
        });
        return new linesDiffComputer_js_1.LinesDiff(changes, [], result.quitEarly);
      }
    };
    exports.LegacyLinesDiffComputer = LegacyLinesDiffComputer2;
    function computeDiff(originalSequence, modifiedSequence, continueProcessingPredicate, pretty) {
      const diffAlgo = new diff_js_1.LcsDiff(originalSequence, modifiedSequence, continueProcessingPredicate);
      return diffAlgo.ComputeDiff(pretty);
    }
    var LineSequence = class {
      lines;
      _startColumns;
      _endColumns;
      constructor(lines) {
        const startColumns = [];
        const endColumns = [];
        for (let i = 0, length = lines.length; i < length; i++) {
          startColumns[i] = getFirstNonBlankColumn(lines[i], 1);
          endColumns[i] = getLastNonBlankColumn(lines[i], 1);
        }
        this.lines = lines;
        this._startColumns = startColumns;
        this._endColumns = endColumns;
      }
      getElements() {
        const elements = [];
        for (let i = 0, len = this.lines.length; i < len; i++) {
          elements[i] = this.lines[i].substring(this._startColumns[i] - 1, this._endColumns[i] - 1);
        }
        return elements;
      }
      getStrictElement(index) {
        return this.lines[index];
      }
      getStartLineNumber(i) {
        return i + 1;
      }
      getEndLineNumber(i) {
        return i + 1;
      }
      createCharSequence(shouldIgnoreTrimWhitespace, startIndex, endIndex) {
        const charCodes = [];
        const lineNumbers = [];
        const columns = [];
        let len = 0;
        for (let index = startIndex; index <= endIndex; index++) {
          const lineContent = this.lines[index];
          const startColumn = shouldIgnoreTrimWhitespace ? this._startColumns[index] : 1;
          const endColumn = shouldIgnoreTrimWhitespace ? this._endColumns[index] : lineContent.length + 1;
          for (let col = startColumn; col < endColumn; col++) {
            charCodes[len] = lineContent.charCodeAt(col - 1);
            lineNumbers[len] = index + 1;
            columns[len] = col;
            len++;
          }
          if (!shouldIgnoreTrimWhitespace && index < endIndex) {
            charCodes[len] = 10;
            lineNumbers[len] = index + 1;
            columns[len] = lineContent.length + 1;
            len++;
          }
        }
        return new CharSequence(charCodes, lineNumbers, columns);
      }
    };
    var CharSequence = class {
      _charCodes;
      _lineNumbers;
      _columns;
      constructor(charCodes, lineNumbers, columns) {
        this._charCodes = charCodes;
        this._lineNumbers = lineNumbers;
        this._columns = columns;
      }
      toString() {
        return "[" + this._charCodes.map((s, idx) => (s === 10 ? "\\n" : String.fromCharCode(s)) + `-(${this._lineNumbers[idx]},${this._columns[idx]})`).join(", ") + "]";
      }
      _assertIndex(index, arr) {
        if (index < 0 || index >= arr.length) {
          throw new Error(`Illegal index`);
        }
      }
      getElements() {
        return this._charCodes;
      }
      getStartLineNumber(i) {
        if (i > 0 && i === this._lineNumbers.length) {
          return this.getEndLineNumber(i - 1);
        }
        this._assertIndex(i, this._lineNumbers);
        return this._lineNumbers[i];
      }
      getEndLineNumber(i) {
        if (i === -1) {
          return this.getStartLineNumber(i + 1);
        }
        this._assertIndex(i, this._lineNumbers);
        if (this._charCodes[i] === 10) {
          return this._lineNumbers[i] + 1;
        }
        return this._lineNumbers[i];
      }
      getStartColumn(i) {
        if (i > 0 && i === this._columns.length) {
          return this.getEndColumn(i - 1);
        }
        this._assertIndex(i, this._columns);
        return this._columns[i];
      }
      getEndColumn(i) {
        if (i === -1) {
          return this.getStartColumn(i + 1);
        }
        this._assertIndex(i, this._columns);
        if (this._charCodes[i] === 10) {
          return 1;
        }
        return this._columns[i] + 1;
      }
    };
    var CharChange = class _CharChange {
      originalStartLineNumber;
      originalStartColumn;
      originalEndLineNumber;
      originalEndColumn;
      modifiedStartLineNumber;
      modifiedStartColumn;
      modifiedEndLineNumber;
      modifiedEndColumn;
      constructor(originalStartLineNumber, originalStartColumn, originalEndLineNumber, originalEndColumn, modifiedStartLineNumber, modifiedStartColumn, modifiedEndLineNumber, modifiedEndColumn) {
        this.originalStartLineNumber = originalStartLineNumber;
        this.originalStartColumn = originalStartColumn;
        this.originalEndLineNumber = originalEndLineNumber;
        this.originalEndColumn = originalEndColumn;
        this.modifiedStartLineNumber = modifiedStartLineNumber;
        this.modifiedStartColumn = modifiedStartColumn;
        this.modifiedEndLineNumber = modifiedEndLineNumber;
        this.modifiedEndColumn = modifiedEndColumn;
      }
      static createFromDiffChange(diffChange, originalCharSequence, modifiedCharSequence) {
        const originalStartLineNumber = originalCharSequence.getStartLineNumber(diffChange.originalStart);
        const originalStartColumn = originalCharSequence.getStartColumn(diffChange.originalStart);
        const originalEndLineNumber = originalCharSequence.getEndLineNumber(diffChange.originalStart + diffChange.originalLength - 1);
        const originalEndColumn = originalCharSequence.getEndColumn(diffChange.originalStart + diffChange.originalLength - 1);
        const modifiedStartLineNumber = modifiedCharSequence.getStartLineNumber(diffChange.modifiedStart);
        const modifiedStartColumn = modifiedCharSequence.getStartColumn(diffChange.modifiedStart);
        const modifiedEndLineNumber = modifiedCharSequence.getEndLineNumber(diffChange.modifiedStart + diffChange.modifiedLength - 1);
        const modifiedEndColumn = modifiedCharSequence.getEndColumn(diffChange.modifiedStart + diffChange.modifiedLength - 1);
        return new _CharChange(originalStartLineNumber, originalStartColumn, originalEndLineNumber, originalEndColumn, modifiedStartLineNumber, modifiedStartColumn, modifiedEndLineNumber, modifiedEndColumn);
      }
    };
    function postProcessCharChanges(rawChanges) {
      if (rawChanges.length <= 1) {
        return rawChanges;
      }
      const result = [rawChanges[0]];
      let prevChange = result[0];
      for (let i = 1, len = rawChanges.length; i < len; i++) {
        const currChange = rawChanges[i];
        const originalMatchingLength = currChange.originalStart - (prevChange.originalStart + prevChange.originalLength);
        const modifiedMatchingLength = currChange.modifiedStart - (prevChange.modifiedStart + prevChange.modifiedLength);
        const matchingLength = Math.min(originalMatchingLength, modifiedMatchingLength);
        if (matchingLength < MINIMUM_MATCHING_CHARACTER_LENGTH) {
          prevChange.originalLength = currChange.originalStart + currChange.originalLength - prevChange.originalStart;
          prevChange.modifiedLength = currChange.modifiedStart + currChange.modifiedLength - prevChange.modifiedStart;
        } else {
          result.push(currChange);
          prevChange = currChange;
        }
      }
      return result;
    }
    var LineChange = class _LineChange {
      originalStartLineNumber;
      originalEndLineNumber;
      modifiedStartLineNumber;
      modifiedEndLineNumber;
      charChanges;
      constructor(originalStartLineNumber, originalEndLineNumber, modifiedStartLineNumber, modifiedEndLineNumber, charChanges) {
        this.originalStartLineNumber = originalStartLineNumber;
        this.originalEndLineNumber = originalEndLineNumber;
        this.modifiedStartLineNumber = modifiedStartLineNumber;
        this.modifiedEndLineNumber = modifiedEndLineNumber;
        this.charChanges = charChanges;
      }
      static createFromDiffResult(shouldIgnoreTrimWhitespace, diffChange, originalLineSequence, modifiedLineSequence, continueCharDiff, shouldComputeCharChanges, shouldPostProcessCharChanges) {
        let originalStartLineNumber;
        let originalEndLineNumber;
        let modifiedStartLineNumber;
        let modifiedEndLineNumber;
        let charChanges = void 0;
        if (diffChange.originalLength === 0) {
          originalStartLineNumber = originalLineSequence.getStartLineNumber(diffChange.originalStart) - 1;
          originalEndLineNumber = 0;
        } else {
          originalStartLineNumber = originalLineSequence.getStartLineNumber(diffChange.originalStart);
          originalEndLineNumber = originalLineSequence.getEndLineNumber(diffChange.originalStart + diffChange.originalLength - 1);
        }
        if (diffChange.modifiedLength === 0) {
          modifiedStartLineNumber = modifiedLineSequence.getStartLineNumber(diffChange.modifiedStart) - 1;
          modifiedEndLineNumber = 0;
        } else {
          modifiedStartLineNumber = modifiedLineSequence.getStartLineNumber(diffChange.modifiedStart);
          modifiedEndLineNumber = modifiedLineSequence.getEndLineNumber(diffChange.modifiedStart + diffChange.modifiedLength - 1);
        }
        if (shouldComputeCharChanges && diffChange.originalLength > 0 && diffChange.originalLength < 20 && diffChange.modifiedLength > 0 && diffChange.modifiedLength < 20 && continueCharDiff()) {
          const originalCharSequence = originalLineSequence.createCharSequence(shouldIgnoreTrimWhitespace, diffChange.originalStart, diffChange.originalStart + diffChange.originalLength - 1);
          const modifiedCharSequence = modifiedLineSequence.createCharSequence(shouldIgnoreTrimWhitespace, diffChange.modifiedStart, diffChange.modifiedStart + diffChange.modifiedLength - 1);
          if (originalCharSequence.getElements().length > 0 && modifiedCharSequence.getElements().length > 0) {
            let rawChanges = computeDiff(originalCharSequence, modifiedCharSequence, continueCharDiff, true).changes;
            if (shouldPostProcessCharChanges) {
              rawChanges = postProcessCharChanges(rawChanges);
            }
            charChanges = [];
            for (let i = 0, length = rawChanges.length; i < length; i++) {
              charChanges.push(CharChange.createFromDiffChange(rawChanges[i], originalCharSequence, modifiedCharSequence));
            }
          }
        }
        return new _LineChange(originalStartLineNumber, originalEndLineNumber, modifiedStartLineNumber, modifiedEndLineNumber, charChanges);
      }
    };
    var DiffComputer2 = class {
      shouldComputeCharChanges;
      shouldPostProcessCharChanges;
      shouldIgnoreTrimWhitespace;
      shouldMakePrettyDiff;
      originalLines;
      modifiedLines;
      original;
      modified;
      continueLineDiff;
      continueCharDiff;
      constructor(originalLines, modifiedLines, opts) {
        this.shouldComputeCharChanges = opts.shouldComputeCharChanges;
        this.shouldPostProcessCharChanges = opts.shouldPostProcessCharChanges;
        this.shouldIgnoreTrimWhitespace = opts.shouldIgnoreTrimWhitespace;
        this.shouldMakePrettyDiff = opts.shouldMakePrettyDiff;
        this.originalLines = originalLines;
        this.modifiedLines = modifiedLines;
        this.original = new LineSequence(originalLines);
        this.modified = new LineSequence(modifiedLines);
        this.continueLineDiff = createContinueProcessingPredicate(opts.maxComputationTime);
        this.continueCharDiff = createContinueProcessingPredicate(opts.maxComputationTime === 0 ? 0 : Math.min(opts.maxComputationTime, 5e3));
      }
      computeDiff() {
        if (this.original.lines.length === 1 && this.original.lines[0].length === 0) {
          if (this.modified.lines.length === 1 && this.modified.lines[0].length === 0) {
            return {
              quitEarly: false,
              changes: []
            };
          }
          return {
            quitEarly: false,
            changes: [{
              originalStartLineNumber: 1,
              originalEndLineNumber: 1,
              modifiedStartLineNumber: 1,
              modifiedEndLineNumber: this.modified.lines.length,
              charChanges: void 0
            }]
          };
        }
        if (this.modified.lines.length === 1 && this.modified.lines[0].length === 0) {
          return {
            quitEarly: false,
            changes: [{
              originalStartLineNumber: 1,
              originalEndLineNumber: this.original.lines.length,
              modifiedStartLineNumber: 1,
              modifiedEndLineNumber: 1,
              charChanges: void 0
            }]
          };
        }
        const diffResult = computeDiff(this.original, this.modified, this.continueLineDiff, this.shouldMakePrettyDiff);
        const rawChanges = diffResult.changes;
        const quitEarly = diffResult.quitEarly;
        if (this.shouldIgnoreTrimWhitespace) {
          const lineChanges = [];
          for (let i = 0, length = rawChanges.length; i < length; i++) {
            lineChanges.push(LineChange.createFromDiffResult(this.shouldIgnoreTrimWhitespace, rawChanges[i], this.original, this.modified, this.continueCharDiff, this.shouldComputeCharChanges, this.shouldPostProcessCharChanges));
          }
          return {
            quitEarly,
            changes: lineChanges
          };
        }
        const result = [];
        let originalLineIndex = 0;
        let modifiedLineIndex = 0;
        for (let i = -1, len = rawChanges.length; i < len; i++) {
          const nextChange = i + 1 < len ? rawChanges[i + 1] : null;
          const originalStop = nextChange ? nextChange.originalStart : this.originalLines.length;
          const modifiedStop = nextChange ? nextChange.modifiedStart : this.modifiedLines.length;
          while (originalLineIndex < originalStop && modifiedLineIndex < modifiedStop) {
            const originalLine = this.originalLines[originalLineIndex];
            const modifiedLine = this.modifiedLines[modifiedLineIndex];
            if (originalLine !== modifiedLine) {
              {
                let originalStartColumn = getFirstNonBlankColumn(originalLine, 1);
                let modifiedStartColumn = getFirstNonBlankColumn(modifiedLine, 1);
                while (originalStartColumn > 1 && modifiedStartColumn > 1) {
                  const originalChar = originalLine.charCodeAt(originalStartColumn - 2);
                  const modifiedChar = modifiedLine.charCodeAt(modifiedStartColumn - 2);
                  if (originalChar !== modifiedChar) {
                    break;
                  }
                  originalStartColumn--;
                  modifiedStartColumn--;
                }
                if (originalStartColumn > 1 || modifiedStartColumn > 1) {
                  this._pushTrimWhitespaceCharChange(result, originalLineIndex + 1, 1, originalStartColumn, modifiedLineIndex + 1, 1, modifiedStartColumn);
                }
              }
              {
                let originalEndColumn = getLastNonBlankColumn(originalLine, 1);
                let modifiedEndColumn = getLastNonBlankColumn(modifiedLine, 1);
                const originalMaxColumn = originalLine.length + 1;
                const modifiedMaxColumn = modifiedLine.length + 1;
                while (originalEndColumn < originalMaxColumn && modifiedEndColumn < modifiedMaxColumn) {
                  const originalChar = originalLine.charCodeAt(originalEndColumn - 1);
                  const modifiedChar = originalLine.charCodeAt(modifiedEndColumn - 1);
                  if (originalChar !== modifiedChar) {
                    break;
                  }
                  originalEndColumn++;
                  modifiedEndColumn++;
                }
                if (originalEndColumn < originalMaxColumn || modifiedEndColumn < modifiedMaxColumn) {
                  this._pushTrimWhitespaceCharChange(result, originalLineIndex + 1, originalEndColumn, originalMaxColumn, modifiedLineIndex + 1, modifiedEndColumn, modifiedMaxColumn);
                }
              }
            }
            originalLineIndex++;
            modifiedLineIndex++;
          }
          if (nextChange) {
            result.push(LineChange.createFromDiffResult(this.shouldIgnoreTrimWhitespace, nextChange, this.original, this.modified, this.continueCharDiff, this.shouldComputeCharChanges, this.shouldPostProcessCharChanges));
            originalLineIndex += nextChange.originalLength;
            modifiedLineIndex += nextChange.modifiedLength;
          }
        }
        return {
          quitEarly,
          changes: result
        };
      }
      _pushTrimWhitespaceCharChange(result, originalLineNumber, originalStartColumn, originalEndColumn, modifiedLineNumber, modifiedStartColumn, modifiedEndColumn) {
        if (this._mergeTrimWhitespaceCharChange(result, originalLineNumber, originalStartColumn, originalEndColumn, modifiedLineNumber, modifiedStartColumn, modifiedEndColumn)) {
          return;
        }
        let charChanges = void 0;
        if (this.shouldComputeCharChanges) {
          charChanges = [new CharChange(originalLineNumber, originalStartColumn, originalLineNumber, originalEndColumn, modifiedLineNumber, modifiedStartColumn, modifiedLineNumber, modifiedEndColumn)];
        }
        result.push(new LineChange(originalLineNumber, originalLineNumber, modifiedLineNumber, modifiedLineNumber, charChanges));
      }
      _mergeTrimWhitespaceCharChange(result, originalLineNumber, originalStartColumn, originalEndColumn, modifiedLineNumber, modifiedStartColumn, modifiedEndColumn) {
        const len = result.length;
        if (len === 0) {
          return false;
        }
        const prevChange = result[len - 1];
        if (prevChange.originalEndLineNumber === 0 || prevChange.modifiedEndLineNumber === 0) {
          return false;
        }
        if (prevChange.originalEndLineNumber === originalLineNumber && prevChange.modifiedEndLineNumber === modifiedLineNumber) {
          if (this.shouldComputeCharChanges && prevChange.charChanges) {
            prevChange.charChanges.push(new CharChange(originalLineNumber, originalStartColumn, originalLineNumber, originalEndColumn, modifiedLineNumber, modifiedStartColumn, modifiedLineNumber, modifiedEndColumn));
          }
          return true;
        }
        if (prevChange.originalEndLineNumber + 1 === originalLineNumber && prevChange.modifiedEndLineNumber + 1 === modifiedLineNumber) {
          prevChange.originalEndLineNumber = originalLineNumber;
          prevChange.modifiedEndLineNumber = modifiedLineNumber;
          if (this.shouldComputeCharChanges && prevChange.charChanges) {
            prevChange.charChanges.push(new CharChange(originalLineNumber, originalStartColumn, originalLineNumber, originalEndColumn, modifiedLineNumber, modifiedStartColumn, modifiedLineNumber, modifiedEndColumn));
          }
          return true;
        }
        return false;
      }
    };
    exports.DiffComputer = DiffComputer2;
    function getFirstNonBlankColumn(txt, defaultValue) {
      const r = strings.firstNonWhitespaceIndex(txt);
      if (r === -1) {
        return defaultValue;
      }
      return r + 1;
    }
    function getLastNonBlankColumn(txt, defaultValue) {
      const r = strings.lastNonWhitespaceIndex(txt);
      if (r === -1) {
        return defaultValue;
      }
      return r + 2;
    }
    function createContinueProcessingPredicate(maximumRuntime) {
      if (maximumRuntime === 0) {
        return () => true;
      }
      const startTime = Date.now();
      return () => {
        return Date.now() - startTime < maximumRuntime;
      };
    }
  }
});

// package/dist/vs/editor/common/diff/linesDiffComputers.js
var require_linesDiffComputers = __commonJS({
  "package/dist/vs/editor/common/diff/linesDiffComputers.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.linesDiffComputers = void 0;
    var legacyLinesDiffComputer_js_1 = require_legacyLinesDiffComputer();
    var defaultLinesDiffComputer_js_1 = require_defaultLinesDiffComputer();
    exports.linesDiffComputers = {
      getLegacy: () => new legacyLinesDiffComputer_js_1.LegacyLinesDiffComputer(),
      getDefault: () => new defaultLinesDiffComputer_js_1.DefaultLinesDiffComputer()
    };
  }
});

// package/dist/index.js
var require_dist = __commonJS({
  "package/dist/index.js"(exports) {
    "use strict";
    var __createBinding = exports && exports.__createBinding || (Object.create ? (function(o, m, k, k2) {
      if (k2 === void 0) k2 = k;
      var desc = Object.getOwnPropertyDescriptor(m, k);
      if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
        desc = { enumerable: true, get: function() {
          return m[k];
        } };
      }
      Object.defineProperty(o, k2, desc);
    }) : (function(o, m, k, k2) {
      if (k2 === void 0) k2 = k;
      o[k2] = m[k];
    }));
    var __exportStar = exports && exports.__exportStar || function(m, exports2) {
      for (var p in m) if (p !== "default" && !Object.prototype.hasOwnProperty.call(exports2, p)) __createBinding(exports2, m, p);
    };
    Object.defineProperty(exports, "__esModule", { value: true });
    __exportStar(require_diff(), exports);
    __exportStar(require_diffChange(), exports);
    __exportStar(require_defaultLinesDiffComputer(), exports);
    __exportStar(require_legacyLinesDiffComputer(), exports);
    __exportStar(require_linesDiffComputer(), exports);
    __exportStar(require_linesDiffComputers(), exports);
    __exportStar(require_rangeMapping(), exports);
  }
});

// esm-entry.js
var import_dist = __toESM(require_dist());
var export_DefaultLinesDiffComputer = import_dist.DefaultLinesDiffComputer;
var export_DetailedLineRangeMapping = import_dist.DetailedLineRangeMapping;
var export_DiffChange = import_dist.DiffChange;
var export_DiffComputer = import_dist.DiffComputer;
var export_LcsDiff = import_dist.LcsDiff;
var export_LegacyLinesDiffComputer = import_dist.LegacyLinesDiffComputer;
var export_LineRangeMapping = import_dist.LineRangeMapping;
var export_LinesDiff = import_dist.LinesDiff;
var export_MovedText = import_dist.MovedText;
var export_RangeMapping = import_dist.RangeMapping;
var export_StringDiffSequence = import_dist.StringDiffSequence;
var export_computeLevenshteinDistance = import_dist.computeLevenshteinDistance;
var export_getLineRangeMapping = import_dist.getLineRangeMapping;
var export_lineRangeMappingFromRangeMappings = import_dist.lineRangeMappingFromRangeMappings;
var export_linesDiffComputers = import_dist.linesDiffComputers;
var export_stringDiff = import_dist.stringDiff;
export {
  export_DefaultLinesDiffComputer as DefaultLinesDiffComputer,
  export_DetailedLineRangeMapping as DetailedLineRangeMapping,
  export_DiffChange as DiffChange,
  export_DiffComputer as DiffComputer,
  export_LcsDiff as LcsDiff,
  export_LegacyLinesDiffComputer as LegacyLinesDiffComputer,
  export_LineRangeMapping as LineRangeMapping,
  export_LinesDiff as LinesDiff,
  export_MovedText as MovedText,
  export_RangeMapping as RangeMapping,
  export_StringDiffSequence as StringDiffSequence,
  export_computeLevenshteinDistance as computeLevenshteinDistance,
  export_getLineRangeMapping as getLineRangeMapping,
  export_lineRangeMappingFromRangeMappings as lineRangeMappingFromRangeMappings,
  export_linesDiffComputers as linesDiffComputers,
  export_stringDiff as stringDiff
};
