// Pure logic shared by the service worker, the options page and the tests.
// No chrome.* APIs here so it can be tested with `node --test`.

export const DEFAULT_ORIGIN = "https://twigg.vc";

// Mirrors the server: usernames and repo names are [A-Za-z0-9_-]+.
const NAME_RE = /^[A-Za-z0-9_-]+$/;

// Short links typed as "<prefix>/<id>". Each redirects to
// "<repo url>/<prefix>/<id>", which twigg-web serves:
//   go/docs -> go link alias defined in the repo's GOLINKS file
//   b/1234  -> bug
//   c/1234  -> commit
// `id` mirrors what the server accepts.
export const LINK_KINDS = [
  { prefix: "go", id: "[A-Za-z0-9_-]+" },
  { prefix: "b", id: "[0-9]+" },
  { prefix: "c", id: "[0-9]+" },
];

// Search engines that receive "b/1234" when Chrome decides the omnibox
// input is a search instead of a URL. The query param is matched below.
const SEARCH_ENGINES = [
  { host: "www.google.com", prefix: "^https://www\\.google\\.com/search\\?" },
  { host: "www.bing.com", prefix: "^https://www\\.bing\\.com/search\\?" },
  { host: "duckduckgo.com", prefix: "^https://duckduckgo\\.com/\\?" },
];

// parseRepo accepts "user/repo", "twigg.vc/user/repo" or a full URL like
// "https://twigg.vc/user/repo/anything" and returns the repo base URL
// (e.g. "https://twigg.vc/user/repo"). Returns undefined if invalid.
export function parseRepo(input) {
  var text = String(input || "").trim();
  if (text === "") {
    return undefined;
  }

  var origin = undefined;
  var path = undefined;
  if (/^https?:\/\//i.test(text)) {
    var url = undefined;
    try {
      url = new URL(text);
    } catch {
      return undefined;
    }
    origin = url.origin;
    path = url.pathname;
  } else if (/^twigg\.vc\//i.test(text)) {
    origin = DEFAULT_ORIGIN;
    path = text.slice("twigg.vc".length);
  } else {
    origin = DEFAULT_ORIGIN;
    path = text;
  }

  var parts = path.split("/").filter((p) => p !== "");
  if (parts.length < 2) {
    return undefined;
  }
  var user = parts[0];
  var repo = parts[1];
  if (!NAME_RE.test(user) || !NAME_RE.test(repo)) {
    return undefined;
  }
  return `${origin}/${user}/${repo}`;
}

// parseLink parses omnibox `text`: a short link like "b/1234" or "go/docs",
// or anything else as a go link alias (e.g. "docs").
export function parseLink(text) {
  var trimmed = text.trim();
  for (var kind of LINK_KINDS) {
    var re = new RegExp(`^${kind.prefix}/(${kind.id})/?$`);
    var match = trimmed.match(re);
    if (match !== null) {
      return { prefix: kind.prefix, id: match[1] };
    }
  }
  return { prefix: "go", id: trimmed };
}

// linkUrl returns the twigg URL for omnibox `text` (see parseLink).
export function linkUrl(repoUrl, text) {
  var link = parseLink(text);
  return `${repoUrl}/${link.prefix}/${encodeURIComponent(link.id)}`;
}

// hostPermissions lists the hosts the redirect rules need access to.
// Keep in sync with "host_permissions" in manifest.json.
export function hostPermissions() {
  var perms = [];
  for (var kind of LINK_KINDS) {
    perms.push(`*://${kind.prefix}/*`);
  }
  for (var engine of SEARCH_ENGINES) {
    perms.push(`https://${engine.host}/*`);
  }
  return perms;
}

// buildRules returns the declarativeNetRequest rules that redirect short
// links to `repoUrl`. Returns no rules if `repoUrl` is undefined.
export function buildRules(repoUrl) {
  if (repoUrl === undefined) {
    return [];
  }

  var rules = [];
  for (var kind of LINK_KINDS) {
    var substitution = `${repoUrl}/${kind.prefix}/\\1`;

    // http://b/1234: typed with a scheme, a trailing slash, or once Chrome
    // has learned that "b" is a host.
    var urlFilter = `^https?://${kind.prefix}/(${kind.id})/?$`;
    rules.push(redirectRule(rules.length + 1, urlFilter, substitution));

    // https://www.google.com/search?q=b%2F1234&...: Chrome treated the
    // input as a search. Only exact "b/1234" queries are redirected, so
    // searches like "b/1234 is great" still reach the search engine.
    var query = `(?:[^#]*&)?q=${kind.prefix}(?:%2[Ff]|/)(${kind.id})(?:[&#].*)?$`;
    for (var engine of SEARCH_ENGINES) {
      rules.push(redirectRule(rules.length + 1, engine.prefix + query, substitution));
    }
  }
  return rules;
}

function redirectRule(id, regexFilter, substitution) {
  return {
    id: id,
    priority: 1,
    action: {
      type: "redirect",
      redirect: { regexSubstitution: substitution },
    },
    condition: {
      regexFilter: regexFilter,
      resourceTypes: ["main_frame"],
    },
  };
}
