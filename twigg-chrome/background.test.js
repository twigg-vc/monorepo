import assert from "node:assert/strict";
import { describe, test } from "node:test";

import { buildRules } from "./golinks.js";

const FIRST_REPO = "https://twigg.vc/user/first";
const SECOND_REPO = "https://twigg.vc/user/second";
var moduleId = 0;

async function loadBackground(t, namespace) {
  var rules = [];
  var onChanged = undefined;
  var failNextUpdate = false;
  var onInstalled = undefined;
  var onInputEntered = undefined;
  var openedOptions = 0;
  var tabs = [];
  var repoUrl = FIRST_REPO;
  var previousChrome = globalThis.chrome;
  var previousBrowser = globalThis.browser;
  t.after(() => {
    globalThis.chrome = previousChrome;
    globalThis.browser = previousBrowser;
  });
  globalThis.chrome = undefined;
  globalThis.browser = undefined;
  globalThis[namespace] = {
    runtime: {
      onInstalled: { addListener(listener) { onInstalled = listener; } },
      openOptionsPage() { openedOptions += 1; },
    },
    tabs: {
      update(tab) { tabs.push(tab); },
      create(tab) { tabs.push(tab); },
    },
    storage: {
      sync: { async get() { return { repoUrl: repoUrl }; } },
      onChanged: {
        addListener(listener) {
          onChanged = listener;
        },
      },
    },
    omnibox: {
      onInputChanged: { addListener() {} },
      onInputEntered: { addListener(listener) { onInputEntered = listener; } },
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
    install: onInstalled,
    enter: onInputEntered,
    tabs: tabs,
    setRepo(value) { repoUrl = value; },
    getOpenedOptions() { return openedOptions; },
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

for (const namespace of ["chrome", "browser"]) {
  describe(namespace, () => {
    test("overlapping settings changes leave redirects for the latest repo", async (t) => {
      var background = await loadBackground(t, namespace);
      await Promise.all([
        background.changeRepo(FIRST_REPO),
        background.changeRepo(SECOND_REPO),
      ]);
      assert.deepEqual(background.getRules(), buildRules(SECOND_REPO));
    });

    test("clearing settings while a save is pending removes redirects", async (t) => {
      var background = await loadBackground(t, namespace);
      await Promise.all([
        background.changeRepo(FIRST_REPO),
        background.changeRepo(undefined),
      ]);
      assert.deepEqual(background.getRules(), []);
    });

    test("a failed update does not prevent later settings changes", async (t) => {
      var background = await loadBackground(t, namespace);
      background.failNextUpdate();
      var failed = background.changeRepo(FIRST_REPO);
      var next = background.changeRepo(SECOND_REPO);
      await assert.rejects(failed, /Update failed/);
      await next;
      assert.deepEqual(background.getRules(), buildRules(SECOND_REPO));
    });

    test("installation restores redirects and only prompts on an unconfigured first install", async (t) => {
      var background = await loadBackground(t, namespace);
      await background.install({ reason: "install" });
      assert.deepEqual(background.getRules(), buildRules(FIRST_REPO));
      assert.equal(background.getOpenedOptions(), 0);
      background.setRepo(undefined);
      await background.install({ reason: "update" });
      assert.deepEqual(background.getRules(), []);
      assert.equal(background.getOpenedOptions(), 0);
      await background.install({ reason: "install" });
      assert.equal(background.getOpenedOptions(), 1);
    });

    test("omnibox opens links in the requested tab or prompts for configuration", async (t) => {
      var background = await loadBackground(t, namespace);
      await background.enter("docs", "currentTab");
      await background.enter("b/123", "newForegroundTab");
      await background.enter("c/456", "newBackgroundTab");
      assert.deepEqual(background.tabs, [
        { url: `${FIRST_REPO}/go/docs` },
        { url: `${FIRST_REPO}/b/123`, active: true },
        { url: `${FIRST_REPO}/c/456`, active: false },
      ]);
      background.setRepo(undefined);
      await background.enter("docs", "currentTab");
      assert.equal(background.tabs.length, 3);
      assert.equal(background.getOpenedOptions(), 1);
    });
  });
}
