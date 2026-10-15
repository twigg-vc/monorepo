import assert from "node:assert/strict";
import { beforeEach, test } from "node:test";

import { getBrowserApi } from "./browser-api.js";

beforeEach((t) => {
  var previousBrowser = globalThis.browser;
  var previousChrome = globalThis.chrome;
  t.after(() => {
    globalThis.browser = previousBrowser;
    globalThis.chrome = previousChrome;
  });
});

test("prefers Firefox's promise APIs even when its chrome alias is present", () => {
  globalThis.browser = { storage: {} };
  globalThis.chrome = { storage: {} };
  assert.equal(getBrowserApi(), globalThis.browser);
});

test("uses Chrome's APIs when browser is absent", () => {
  globalThis.browser = undefined;
  globalThis.chrome = { storage: {} };
  assert.equal(getBrowserApi(), globalThis.chrome);
});
