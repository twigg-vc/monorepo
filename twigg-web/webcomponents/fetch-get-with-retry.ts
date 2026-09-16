export interface RetryOptions {
    // Total number of attempts, including the first one. Default: 3.
    attempts?: number
}

const baseDelayMs = 300
const defaultNumberOfAttempts = 3

// fetch for GET requests that retries on network errors, 5xx and 429.
// Non-GET requests throw a error
export async function fetchGetWithRetry(
    input: RequestInfo | URL,
    init?: RequestInit,
    options?: RetryOptions,
): Promise<Response> {
    if (!isGetRequest(init)) {
        throw new Error("fetchGetWithRetry only supports GET requests")
    }
    const attempts = options?.attempts ?? defaultNumberOfAttempts

    for (let attempt = 1; attempt <= attempts; attempt++) {
        const isLastAttempt = attempt === attempts
        try {
            const resp = await fetch(input, init)
            if (!isRetryableStatus(resp.status) || isLastAttempt) {
                return resp
            }
        } catch (err) {
            if (isLastAttempt) {
                throw err
            }
        }
        await sleep(baseDelayMs * attempt)
    }
    throw new Error("unreachable")
}

function isGetRequest(init?: RequestInit): boolean {
    if (init?.method === undefined) {
        return true
    }
    return init.method.toUpperCase() === 'GET'
}

function isRetryableStatus(status: number): boolean {
    return status === 429 || (status >= 500 && status <= 599)
}

function sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms))
}
