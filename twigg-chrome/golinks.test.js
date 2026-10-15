import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import { buildRules, hostPermissions, linkUrl, parseLink, parseRepo } from "./golinks.js";

const REPO = "https://twigg.vc/twigg/monorepo";

// redirect applies `rules` to `url` like declarativeNetRequest does and
// returns the redirect target, or undefined if no rule matches.
function redirect(rules, url) {
  for (var rule of rules) {
    var re = new RegExp(rule.condition.regexFilter);
    if (re.test(url)) {
      var substitution = rule.action.redirect.regexSubstitution.replace("\\1", "$1");
      return url.replace(re, substitution);
    }
  }
  return undefined;
}

test("parseRepo accepts user/repo forms", () => {
  var cases = [
    ["twigg/monorepo", REPO],
    ["  twigg/monorepo/  ", REPO],
    ["/twigg/monorepo", REPO],
    ["twigg.vc/twigg/monorepo", REPO],
    ["https://twigg.vc/twigg/monorepo", REPO],
    ["https://twigg.vc/twigg/monorepo/c/2198", REPO],
    ["https://twigg.vc/twigg/monorepo?x=1#y", REPO],
    ["http://localhost:8080/me/my_repo-2", "http://localhost:8080/me/my_repo-2"],
  ];
  for (var [input, want] of cases) {
    assert.equal(parseRepo(input), want, input);
  }
});

test("parseRepo rejects invalid input", () => {
  var cases = [
    undefined,
    "",
    "   ",
    "twigg",
    "https://twigg.vc/twigg",
    "twigg/mono repo",
    "twigg/mono.repo",
    "ftp://twigg.vc/twigg/monorepo",
    "https://",
  ];
  for (var input of cases) {
    assert.equal(parseRepo(input), undefined, String(input));
  }
});

test("parseLink", () => {
  var cases = [
    ["docs", { prefix: "go", id: "docs" }],
    ["  go/docs/ ", { prefix: "go", id: "docs" }],
    ["b/1234", { prefix: "b", id: "1234" }],
    ["c/1234", { prefix: "c", id: "1234" }],
    ["b/abc", { prefix: "go", id: "b/abc" }],
  ];
  for (var [text, want] of cases) {
    assert.deepEqual(parseLink(text), want, text);
  }
});

test("linkUrl", () => {
  var cases = [
    ["docs", `${REPO}/go/docs`],
    ["go/docs", `${REPO}/go/docs`],
    ["b/1234", `${REPO}/b/1234`],
    ["c/1234", `${REPO}/c/1234`],
    ["a b", `${REPO}/go/a%20b`],
  ];
  for (var [text, want] of cases) {
    assert.equal(linkUrl(REPO, text), want, text);
  }
});

test("buildRules without a repo has no rules", () => {
  assert.deepEqual(buildRules(undefined), []);
});

test("buildRules has unique ids", () => {
  var ids = buildRules(REPO).map((r) => r.id);
  assert.equal(new Set(ids).size, ids.length);
});

test("buildRules redirects short links", () => {
  var rules = buildRules(REPO);
  var cases = [
    ["http://go/tw", `${REPO}/go/tw`],
    ["https://go/tw", `${REPO}/go/tw`],
    ["http://go/tw/", `${REPO}/go/tw`],
    ["http://go/my_alias-2", `${REPO}/go/my_alias-2`],
    [
      "https://www.google.com/search?q=go%2Ftw&oq=go%2Ftw&sourceid=chrome&ie=UTF-8",
      `${REPO}/go/tw`,
    ],
    ["https://www.google.com/search?client=ubuntu&q=go%2ftw", `${REPO}/go/tw`],
    ["https://www.bing.com/search?q=go%2Ftw&form=QBLH", `${REPO}/go/tw`],
    ["https://duckduckgo.com/?q=go%2Ftw&t=h_", `${REPO}/go/tw`],
    ["https://duckduckgo.com/?q=go/tw", `${REPO}/go/tw`],
    ["http://b/1234", `${REPO}/b/1234`],
    ["http://b/1234/", `${REPO}/b/1234`],
    ["http://c/1234", `${REPO}/c/1234`],
    ["https://www.google.com/search?q=b%2F1234&sourceid=chrome", `${REPO}/b/1234`],
    ["https://www.bing.com/search?q=c%2F1234", `${REPO}/c/1234`],
    ["https://duckduckgo.com/?q=c/1234", `${REPO}/c/1234`],
  ];
  for (var [url, want] of cases) {
    assert.equal(redirect(rules, url), want, url);
  }
});

test("buildRules leaves other requests alone", () => {
  var rules = buildRules(REPO);
  var cases = [
    "http://go/",
    "http://go/tw/extra",
    "http://go.dev/tw",
    "https://golang.org/go/tw",
    "http://go/tw?x=1",
    "https://www.google.com/search?q=go%2Ftw+is+great",
    "https://www.google.com/search?q=golang",
    "https://www.google.com/search?q=lets+go%2Ftw",
    "https://www.google.com/search?oq=go%2Ftw&q=something",
    "https://www.google.com/maps?q=go%2Ftw",
    "https://www.google.com.br/search?q=go%2Ftw",
    "http://b/abc",
    "http://c/12v1",
    "http://b/",
    "https://www.google.com/search?q=b%2Fabc",
    "https://www.google.com/search?q=ab%2F1234",
    "https://www.google.com/search?q=c%2F1234+error",
  ];
  for (var url of cases) {
    assert.equal(redirect(rules, url), undefined, url);
  }
});

test("manifest host_permissions match hostPermissions()", () => {
  var manifest = JSON.parse(readFileSync(new URL("./manifest.json", import.meta.url)));
  assert.deepEqual(manifest.host_permissions, hostPermissions());
});
