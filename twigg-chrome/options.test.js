import assert from "node:assert/strict";
import { test } from "node:test";

var moduleId = 0;
for (const namespace of ["chrome", "browser"]) {
  test(`${namespace}: settings load, save, reject invalid input and clear`, async (t) => {
    var previous = { chrome: globalThis.chrome, browser: globalThis.browser, document: globalThis.document };
    t.after(() => Object.assign(globalThis, previous));
    var repoUrl = "https://twigg.vc/user/first";
    var submit = undefined;
    var elements = {
      form: { addEventListener(event, listener) { submit = listener; } },
      repo: { value: "" },
      status: { textContent: "", classList: { add() {}, remove() {} } },
    };
    globalThis.document = { getElementById(id) { return elements[id]; } };
    globalThis.chrome = undefined;
    globalThis.browser = undefined;
    globalThis[namespace] = {
      storage: {
        sync: {
          async get() { return { repoUrl: repoUrl }; },
          async set(values) { repoUrl = values.repoUrl; },
          async remove() { repoUrl = undefined; },
        },
      },
    };
    moduleId += 1;
    await import(`./options.js?test=${moduleId}`);
    assert.equal(elements.repo.value, repoUrl);
    elements.repo.value = "user/second";
    await submit({ preventDefault() {} });
    assert.equal(repoUrl, "https://twigg.vc/user/second");
    assert.equal(elements.repo.value, repoUrl);
    elements.repo.value = "invalid";
    await submit({ preventDefault() {} });
    assert.equal(repoUrl, "https://twigg.vc/user/second");
    assert.match(elements.status.textContent, /Invalid repository/);
    elements.repo.value = "";
    await submit({ preventDefault() {} });
    assert.equal(repoUrl, undefined);
    assert.match(elements.status.textContent, /disabled/);
  });
}
