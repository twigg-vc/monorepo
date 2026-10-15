# Twigg Links (Chrome extension)

Type a short link in the address bar to open it in your Twigg repo:

| You type | You land on |
|---|---|
| `go/docs` | The URL your repo's `GOLINKS` file defines for `docs` (via `https://twigg.vc/<user>/<repo>/go/docs`) |
| `b/1234` | Bug 1234: `https://twigg.vc/<user>/<repo>/b/1234` |
| `c/1234` | Commit 1234: `https://twigg.vc/<user>/<repo>/c/1234` |

To add a new kind of link, add it to `LINK_KINDS` in `golinks.js` and add its
host to `host_permissions` in `manifest.json`.

## Install (unpacked)

1. Open `chrome://extensions` and turn on **Developer mode** (top right).
2. Click **Load unpacked** and pick this `twigg-chrome` folder.
3. The settings popup opens on install. You can also open it from the
   extension's toolbar icon. Enter `user/repo` (e.g. `twigg/monorepo`) or paste
   a repo link and click **Save**.

After you edit any file, click the reload icon on the extension's card in
`chrome://extensions`.

## How it works

The extension is plain JavaScript with no build step (Manifest V3).

| File | Purpose |
|---|---|
| `manifest.json` | Extension metadata and permissions |
| `golinks.js` | Pure logic: parses the repo link and short links, and builds the redirect rules |
| `background.js` | Service worker: installs the rules and handles the `go` omnibox keyword |
| `options.html/js/css` | Settings page (also the toolbar popup) |

Chrome can handle `b/1234` in two ways, and the extension redirects both with
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

**Always works:** type `go`, press <kbd>Space</kbd> or <kbd>Tab</kbd>, type an
alias (`docs`), `b/1234` or `c/1234`, and press <kbd>Enter</kbd>.

The repo is stored as its base URL. If you paste a link from another host (e.g.
`http://localhost:8080/user/repo`), the extension redirects there instead,
which is handy for testing twigg-web locally.

## Development

```bash
make test     # node --test
make package  # twigg-chrome.zip for the Chrome Web Store
```
