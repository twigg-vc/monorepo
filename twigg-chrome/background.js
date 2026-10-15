import { buildRules, linkUrl, parseLink } from "./golinks.js";

// The configured repo is stored as its base URL, e.g.
// "https://twigg.vc/user/repo" (see parseRepo in golinks.js).
const REPO_URL_KEY = "repoUrl";
var rulesQueue = Promise.resolve();

async function getRepoUrl() {
  var stored = await chrome.storage.sync.get(REPO_URL_KEY);
  return stored[REPO_URL_KEY];
}

// syncRules replaces all dynamic redirect rules with the ones for `repoUrl`.
function syncRules(repoUrl) {
  var update = rulesQueue.then(async () => {
    var existing = await chrome.declarativeNetRequest.getDynamicRules();
    await chrome.declarativeNetRequest.updateDynamicRules({
      removeRuleIds: existing.map((r) => r.id),
      addRules: buildRules(repoUrl),
    });
  });
  // Keep later settings changes running even if an update fails.
  rulesQueue = update.catch(() => {});
  return update;
}

chrome.runtime.onInstalled.addListener(async (details) => {
  var repoUrl = await getRepoUrl();
  await syncRules(repoUrl);
  if (details.reason === "install" && repoUrl === undefined) {
    chrome.runtime.openOptionsPage();
  }
});

chrome.storage.onChanged.addListener(async (changes, area) => {
  if (area !== "sync" || !(REPO_URL_KEY in changes)) {
    return;
  }
  await syncRules(changes[REPO_URL_KEY].newValue);
});

// Fallback that always works: type "go", space, then an alias, "b/1234" or
// "c/1234".
chrome.omnibox.onInputChanged.addListener((text) => {
  var link = parseLink(text);
  // The description is XML: drop characters that would break it.
  var id = link.id.replace(/[&<>"']/g, "");
  chrome.omnibox.setDefaultSuggestion({
    description: `Open <match>${link.prefix}/${id}</match>`,
  });
});

chrome.omnibox.onInputEntered.addListener(async (text, disposition) => {
  var repoUrl = await getRepoUrl();
  if (repoUrl === undefined) {
    chrome.runtime.openOptionsPage();
    return;
  }
  var url = linkUrl(repoUrl, text);
  if (disposition === "currentTab") {
    chrome.tabs.update({ url: url });
  } else {
    chrome.tabs.create({ url: url, active: disposition === "newForegroundTab" });
  }
});
