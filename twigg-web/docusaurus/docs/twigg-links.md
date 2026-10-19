# Twigg Links
Twigg Links is a browser extension that turns short links typed in the address bar into pages of your Twigg repository:

| You type | You land on |
|---|---|
| `go/docs` | The URL your repository's `GOLINKS` file defines for `docs` |
| `b/1234` | Bug 1234 |
| `c/1234` | Commit 1234 |

## Install

Install it from the [Chrome Web Store](https://chromewebstore.google.com/detail/fdbalodlehkpnmaocfhhjfdbhhbeoelg).
Firefox is also supported: see the [extension's README](https://github.com/twigg-vc/monorepo/tree/master/twigg-chrome) to load it from source.

After installing, click the extension's toolbar icon, enter your repository as `user/repo` (for example `twigg/monorepo`) or paste a link to it, and click **Save**.

## Go links

Go links are short aliases for URLs your team uses often: `go/docs`, `go/dashboard`, `go/oncall`.
They are defined in a `GOLINKS` file at the root of your repository, one alias per line:

```
# <alias> <url>
docs https://twigg.vc/docs/v/2/
dashboard https://grafana.example.com/d/main
```

Blank lines and lines starting with `#` are ignored. Aliases may only contain letters, numbers, `_` and `-`, and must be unique. URLs must start with `http://` or `https://`. A badly formatted line makes every go link of the repository fail with an error pointing at that line, so keep the file tidy.

Twigg resolves the aliases from the repository's trunk at `https://twigg.vc/<user>/<repo>/go/<alias>`, so the links also work without the extension. The extension just saves you the typing.

## Tips

- Type `go`, press <kbd>Space</kbd>, then the alias, `b/1234` or `c/1234`, and press <kbd>Enter</kbd> to open a link explicitly. This also works when the automatic redirect does not kick in.
