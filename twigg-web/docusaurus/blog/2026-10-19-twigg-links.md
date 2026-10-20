---
slug: twigg-links
title: Introducing Twigg Links
authors: [marcos]
tags: [announcement]
---

Today we are releasing **Twigg Links**, a browser extension that turns short links into pages of your repository.

Type `go/docs`, `go/dashboard`, `go/oncall` or `go/whatever-you-want` in the address bar. It redirects you to the URL you defined in your `GOLINKS` file. Type `b/1234` and you are on bug 1234. Type `c/1234` and you are on commit 1234.

It is available now for Chrome and Firefox, on all plans.
{/* truncate */}

## What it does

| You type | You land on |
|---|---|
| `go/docs` | Your team's docs |
| `go/oncall` | The on-call page |
| `b/1234` | Bug 1234 |
| `c/1234` | Commit 1234 |

Where is the dashboard link again? Someone sent it on Slack last week. Or was it in the wiki? Or in a doc?

With a go link you don't need to know. You remember what a thing is, not where it lives.

Twigg Links gives every page your team uses a short name. Nobody remembers `grafana.example.com/d/abc123?orgId=4`, but everybody remembers `go/dashboard`. Say it in a meeting, paste it in a chat, type it next month. It just works, and nobody has to go hunting for a link again.

## Links live in your repo

There is no admin page and no extra tool. Your go links are a `GOLINKS` file at the root of your repository, one link per line:

```
docs https://twigg.vc/docs/v/2/
dashboard https://grafana.example.com/d/main
oncall https://example.com/oncall
```

To add a link, add a line and push. It goes through code review like any other change, and the whole team gets it at once.

Go links also work without the extension at `https://twigg.vc/<user>/<repo>/go/<alias>`. The extension just saves you the typing.

## Get started

1. Install it for [Chrome](https://chromewebstore.google.com/detail/fdbalodlehkpnmaocfhhjfdbhhbeoelg) or [Firefox](https://addons.mozilla.org/firefox/addon/twigg-links/).
2. Click the extension icon, enter your repository as `user/repo`, and click **Save**.
3. Open a new tab and type `go/docs`.

Read more in the [Twigg Links docs](/twigg-links).
