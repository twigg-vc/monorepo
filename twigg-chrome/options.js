import { getBrowserApi } from "./browser-api.js";
import { parseRepo } from "./golinks.js";

const api = getBrowserApi();
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
  showStatus(`Links will open in ${repoUrl}.`, false);
}

async function load() {
  var stored = await api.storage.sync.get(REPO_URL_KEY);
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
    await api.storage.sync.remove(REPO_URL_KEY);
    showStatus("Cleared: go links are disabled.", false);
    return;
  }
  var repoUrl = parseRepo(input.value);
  if (repoUrl === undefined) {
    showStatus("Invalid repository. Use user/repo.", true);
    return;
  }
  await api.storage.sync.set({ [REPO_URL_KEY]: repoUrl });
  input.value = repoUrl;
  showConfigured(repoUrl);
});

load();
