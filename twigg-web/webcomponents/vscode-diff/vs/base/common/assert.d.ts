/**
 * Asserts that a condition is `truthy`.
 *
 * @throws provided {@linkcode messageOrError} if the {@linkcode condition} is `falsy`.
 *
 * @param condition The condition to assert.
 * @param messageOrError An error message or error object to throw if condition is `falsy`.
 */
export declare function assert(condition: boolean, messageOrError?: string | Error): asserts condition;
/**
 * condition must be side-effect free!
 */
export declare function assertFn(condition: () => boolean): void;
export declare function checkAdjacentItems<T>(items: readonly T[], predicate: (item1: T, item2: T) => boolean): boolean;
