export declare function onUnexpectedError(e: any): undefined;
/**
 * Error that when thrown won't be logged in telemetry as an unhandled error.
 */
export declare class ErrorNoTelemetry extends Error {
    readonly name: string;
    constructor(msg?: string);
    static fromError(err: Error): ErrorNoTelemetry;
    static isErrorNoTelemetry(err: Error): err is ErrorNoTelemetry;
}
/**
 * This error indicates a bug.
 * Do not throw this for invalid user input.
 * Only catch this error to recover gracefully from bugs.
 */
export declare class BugIndicatingError extends Error {
    constructor(message?: string);
}
