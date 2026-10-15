# Twigg Links (Chrome and Firefox extension)

One manifest and shared JavaScript support Chrome 121+ and Firefox 140+.

Type a short link in the address bar to open it in your Twigg repo:

| You type | You land on |
|---|---|
| `go/docs` | The URL your repo's `GOLINKS` file defines for `docs` (via `https://twigg.vc/<user>/<repo>/go/docs`) |
| `b/1234` | Bug 1234: `https://twigg.vc/<user>/<repo>/b/1234` |
| `c/1234` | Commit 1234: `https://twigg.vc/<user>/<repo>/c/1234` |

To add a new kind of link, add it to `LINK_KINDS` in `golinks.js` and add its
host to `host_permissions` in `manifest.json`.

## Install in Chrome (unpacked)

1. Open `chrome://extensions` and turn on **Developer mode** (top right).
2. Click **Load unpacked** and pick this `twigg-chrome` folder.
3. The settings popup opens on install. You can also open it from the
   extension's toolbar icon. Enter `user/repo` (e.g. `twigg/monorepo`) or paste
   a repo link and click **Save**.

After you edit any file, click the reload icon on the extension's card in
`chrome://extensions`.

## Install in Firefox (temporary)

1. Open `about:debugging#/runtime/this-firefox`.
2. Click **Load Temporary Add-on** and select this folder's `manifest.json`.
3. Open the extension's toolbar popup and save your repository as above.

After editing files, click **Reload** on the extension in `about:debugging`.
Temporary add-ons are removed when the browser restarts. For permanent
installation, submit `twigg-firefox.zip` to
[Mozilla Add-ons](https://addons.mozilla.org/developers/) for signing and
distribution. An unlisted signed XPI
can also be distributed directly.

## How it works

The extension is plain JavaScript with no build step (Manifest V3). The manifest
declares both a Chrome service worker and Firefox background scripts, loading
the same module. `browser-api.js` selects Firefox's promise-based `browser`
APIs or Chrome's `chrome` APIs.

| File | Purpose |
|---|---|
| `manifest.json` | Extension metadata and permissions |
| `browser-api.js` | Shared browser API selection |
| `golinks.js` | Pure logic: parses the repo link and short links, and builds the redirect rules |
| `background.js` | Background module: installs the rules and handles the `go` omnibox keyword |
| `options.html/js/css` | Settings page (also the toolbar popup) |

Browsers can handle `b/1234` in two ways, and the extension redirects both with
[declarativeNetRequest](https://developer.chrome.com/docs/extensions/reference/api/declarativeNetRequest)
rules:

- **As a URL** (`http://b/1234`): this happens when you type `http://b/1234`
  or `b/1234/`, or once Chrome has learned that `b` is a host.
- **As a search** (`https://www.google.com/search?q=b%2F1234`, and the same on
  Bing and DuckDuckGo): this is what Chrome usually does the first time. The
  extension redirects only exact `b/1234` queries. This is why it needs
  access to those search sites. Google's country domains (e.g. `google.com.br`)
  aren't covered.

Go link aliases must match `[A-Za-z0-9_-]+`, as on the server. Bug and commit
links take a number only.

**Explicit keyword:** type `go`, press <kbd>Space</kbd>, type an
alias (`docs`), `b/1234` or `c/1234`, and press <kbd>Enter</kbd>.
Chrome also accepts <kbd>Tab</kbd> to activate the keyword. Other search engines
and Google's country domains aren't covered by the automatic redirects; use
the keyword in those cases. Allow the extension access to its requested sites
in the browser's extension permissions for automatic redirects to work.

The repo is stored as its base URL. If you paste a link from another host (e.g.
`http://localhost:8080/user/repo`), the extension redirects there instead,
which is handy for testing twigg-web locally.

Repository settings use the browser's sync storage. They do not sync between
Chrome and Firefox. The extension has no telemetry; it navigates to the
configured repository when opening a link.

## Development

```bash
make test     # node --test
make package  # twigg-chrome.zip and twigg-firefox.zip, with identical contents
```

The Gecko ID `links@twigg.vc` identifies the Firefox extension and its sync
storage. Keep it stable after publishing. Mozilla signing and store publication
are separate from `make package`; the generated ZIPs are unsigned.
