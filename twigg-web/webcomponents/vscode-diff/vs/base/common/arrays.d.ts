export declare function equals<T>(one: ReadonlyArray<T> | undefined, other: ReadonlyArray<T> | undefined, itemEquals?: (a: T, b: T) => boolean): boolean;
/**
 * Splits the given items into a list of (non-empty) groups.
 * `shouldBeGrouped` is used to decide if two consecutive items should be in the same group.
 * The order of the items is preserved.
 */
export declare function groupAdjacentBy<T>(items: Iterable<T>, shouldBeGrouped: (item1: T, item2: T) => boolean): Iterable<T[]>;
export declare function forEachAdjacent<T>(arr: T[], f: (item1: T | undefined, item2: T | undefined) => void): void;
export declare function forEachWithNeighbors<T>(arr: T[], f: (before: T | undefined, element: T, after: T | undefined) => void): void;
export declare function pushMany<T>(arr: T[], items: ReadonlyArray<T>): void;
/**
 * When comparing two values,
 * a negative number indicates that the first value is less than the second,
 * a positive number indicates that the first value is greater than the second,
 * and zero indicates that neither is the case.
*/
type CompareResult = number;
export declare namespace CompareResult {
    function isLessThan(result: CompareResult): boolean;
    function isLessThanOrEqual(result: CompareResult): boolean;
    function isGreaterThan(result: CompareResult): boolean;
    function isNeitherLessOrGreaterThan(result: CompareResult): boolean;
    const greaterThan = 1;
    const lessThan = -1;
    const neitherLessOrGreaterThan = 0;
}
/**
 * A comparator `c` defines a total order `<=` on `T` as following:
 * `c(a, b) <= 0` iff `a` <= `b`.
 * We also have `c(a, b) == 0` iff `c(b, a) == 0`.
*/
type Comparator<T> = (a: T, b: T) => CompareResult;
export declare function compareBy<TItem, TCompareBy>(selector: (item: TItem) => TCompareBy, comparator: Comparator<TCompareBy>): Comparator<TItem>;
/**
 * The natural order on numbers.
*/
export declare const numberComparator: Comparator<number>;
export declare function reverseOrder<TItem>(comparator: Comparator<TItem>): Comparator<TItem>;
export {};
