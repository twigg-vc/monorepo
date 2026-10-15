import { linkUrl, parseRepo } from "./golinks.js";

const REPO_URL_KEY = "repoUrl";

var form = document.getElementById("form");
var input = document.getElementById("repo");
var status = document.getElementById("status");

function showStatus(text, isError) {
  status.textContent = text;
  if (isError) {
    status.classList.add("error");
  } else {
    status.classList.remove("error");
  }
}

function showConfigured(repoUrl) {
  showStatus(`go/docs → ${linkUrl(repoUrl, "go/docs")}`, false);
}

async function load() {
  var stored = await chrome.storage.sync.get(REPO_URL_KEY);
  var repoUrl = stored[REPO_URL_KEY];
  if (repoUrl === undefined) {
    showStatus("Not configured: go links are disabled.", false);
    return;
  }
  input.value = repoUrl;
  showConfigured(repoUrl);
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  if (input.value.trim() === "") {
    await chrome.storage.sync.remove(REPO_URL_KEY);
    showStatus("Cleared: go links are disabled.", false);
    return;
  }
  var repoUrl = parseRepo(input.value);
  if (repoUrl === undefined) {
    showStatus("Invalid repository. Use user/repo.", true);
    return;
  }
  await chrome.storage.sync.set({ [REPO_URL_KEY]: repoUrl });
  input.value = repoUrl;
  showConfigured(repoUrl);
});

load();
