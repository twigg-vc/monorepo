import { expect } from '@open-wc/testing';
import { fetchGetWithRetry } from './fetch-get-with-retry';

var fetchResults: (Response | Error)[] = []
var fetchCalls = 0

function response(status: number): Response {
    return new Response("", { status })
}
const networkError = new Error("network error")

async function fakeFetch(): Promise<Response> {
    fetchCalls++
    const result = fetchResults.shift()
    if (result === undefined) {
        throw new Error("fetch called more times than expected")
    }
    if (result instanceof Error) {
        throw result
    }
    return result
}

describe('fetchGetWithRetry', () => {
    const realFetch = globalThis.fetch
    beforeEach(() => {
        fetchResults = []
        fetchCalls = 0
        globalThis.fetch = fakeFetch
    })
    afterEach(() => { globalThis.fetch = realFetch })

    it('returns a successful response without retrying', async () => {
        fetchResults = [response(200)]
        const resp = await fetchGetWithRetry('/x')
        expect(resp.status).to.equal(200)
        expect(fetchCalls).to.equal(1)
    });

    it('retries after a network error', async () => {
        fetchResults = [networkError, response(200)]
        const resp = await fetchGetWithRetry('/x', { method: 'GET' })
        expect(resp.status).to.equal(200)
        expect(fetchCalls).to.equal(2)
    });

    it('retries after 5xx and 429 responses', async () => {
        fetchResults = [response(503), response(429), response(200)]
        const resp = await fetchGetWithRetry('/x')
        expect(resp.status).to.equal(200)
        expect(fetchCalls).to.equal(3)
    });

    it('does not retry 4xx responses', async () => {
        fetchResults = [response(404)]
        const resp = await fetchGetWithRetry('/x')
        expect(resp.status).to.equal(404)
        expect(fetchCalls).to.equal(1)
    });

    it('returns the last response when attempts are exhausted', async () => {
        fetchResults = [response(500), response(502)]
        const resp = await fetchGetWithRetry('/x', undefined, { attempts: 2 })
        expect(resp.status).to.equal(502)
        expect(fetchCalls).to.equal(2)
    });

    it('rethrows the network error when attempts are exhausted', async () => {
        fetchResults = [response(503), networkError]
        var thrown: unknown = undefined
        try {
            await fetchGetWithRetry('/x', undefined, { attempts: 2 })
        } catch (err) {
            thrown = err
        }
        expect(thrown).to.equal(networkError)
        expect(fetchCalls).to.equal(2)
    });

    it('throws on non-GET requests without calling fetch', async () => {
        var thrown: unknown = undefined
        try {
            await fetchGetWithRetry('/x', { method: 'POST' })
        } catch (err) {
            thrown = err
        }
        expect((thrown as Error).message).to.contain("only supports GET")
        expect(fetchCalls).to.equal(0)
    });
});
