import assert from "node:assert/strict";
import { test } from "node:test";

import { buildRules } from "./golinks.js";

const FIRST_REPO = "https://twigg.vc/user/first";
const SECOND_REPO = "https://twigg.vc/user/second";
var moduleId = 0;

async function loadBackground(t) {
  var rules = [];
  var onChanged = undefined;
  var failNextUpdate = false;
  var previousChrome = globalThis.chrome;
  t.after(() => {
    globalThis.chrome = previousChrome;
  });
  globalThis.chrome = {
    runtime: { onInstalled: { addListener() {} } },
    storage: {
      onChanged: {
        addListener(listener) {
          onChanged = listener;
        },
      },
    },
    omnibox: {
      onInputChanged: { addListener() {} },
      onInputEntered: { addListener() {} },
    },
    declarativeNetRequest: {
      async getDynamicRules() {
        return [...rules];
      },
      async updateDynamicRules({ removeRuleIds, addRules }) {
        if (failNextUpdate) {
          failNextUpdate = false;
          throw new Error("Update failed");
        }
        var remaining = rules.filter((rule) => !removeRuleIds.includes(rule.id));
        for (var rule of addRules) {
          if (remaining.some((existing) => existing.id === rule.id)) {
            throw new Error("Duplicate rule ID");
          }
        }
        rules = remaining.concat(addRules);
      },
    },
  };
  moduleId += 1;
  await import(`./background.js?test=${moduleId}`);
  return {
    changeRepo(repoUrl) {
      return onChanged({ repoUrl: { newValue: repoUrl } }, "sync");
    },
    getRules() {
      return rules;
    },
    failNextUpdate() {
      failNextUpdate = true;
    },
  };
}

test("overlapping settings changes leave redirects for the latest repo", async (t) => {
  var background = await loadBackground(t);
  await Promise.all([
    background.changeRepo(FIRST_REPO),
    background.changeRepo(SECOND_REPO),
  ]);
  assert.deepEqual(background.getRules(), buildRules(SECOND_REPO));
});

test("clearing settings while a save is pending removes redirects", async (t) => {
  var background = await loadBackground(t);
  await Promise.all([
    background.changeRepo(FIRST_REPO),
    background.changeRepo(undefined),
  ]);
  assert.deepEqual(background.getRules(), []);
});

test("a failed update does not prevent later settings changes", async (t) => {
  var background = await loadBackground(t);
  background.failNextUpdate();
  var failed = background.changeRepo(FIRST_REPO);
  var next = background.changeRepo(SECOND_REPO);
  await assert.rejects(failed, /Update failed/);
  await next;
  assert.deepEqual(background.getRules(), buildRules(SECOND_REPO));
});
