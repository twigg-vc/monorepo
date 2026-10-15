---
name: twigg
description: Use the Twigg Version Control CLI (`tw`).
---

# Twigg

Twigg is a version control for trunk-based development with stacked commits.

## Commit syntax

- `3` local commit 3, latest version. `3v1` local commit 3, version 1.
- `c/7` (or `c7`) server commit 7, latest version. `c/7v2` version 2.

Local ids are per machine. Server ids are global and assigned on push.
Every commit starts at version `0`; amend, restore, rebase and submit create a
new version.

## Commands

- `tw log <n>` shows the commit tree (`@` marks the current commit).
  `tw status` shows workdir changes.
- `tw goto <commit>` loads a commit into the workdir, overwriting it.
- `tw up` / `tw down` move to a child / the parent commit.
- `tw top` moves to the newest submitted commit (the trunk tip).
- `tw commit <message>` creates a commit. The message must be a single line.
- `tw amend <optional: message>` amends the current commit. Children are
  auto-rebased.
- `tw rebase <source> <target>` rebases source onto target.
- `tw restore <commit-with-version>` creates a new version copying an old one
  (e.g. `1v2`).
- `tw diff [<c1> [<c2>]]` lists changed files. Add `--all` for a unified diff.
  It compares commits, not the workdir: run `tw status` for unsaved changes.
- `tw clean` discards workdir changes after confirmation (`-f` skips it).
- `tw push` pushes the current commit and its non-pushed ancestors.
- `tw pull c/99` downloads the latest version of server commit 99.
- `tw description-set <optional: commit> <description>` sets the review
  description of a pushed commit. If the commit is omitted, it uses the current
  commit.
- `tw bug-get b/5` shows bug 5 and its events.

## Statuses

`tw log` shows a status after each commit id:

- No status: this version is local only (not pushed yet).
- `Pushed`: on the server, under review. It can still be amended and re-pushed.
- `Submitted`: in trunk. It can't be modified anymore.
- `Conflicts`: has unresolved conflicts (see below).
- `Hidden`: hidden with `tw hide`.
- `Obsolete`: replaced by a new version (by an amend, restore, rebase).

## Conflicts

Rebase, amend and restore may conflict. Twigg then creates a new commit version
with conflict markers. `goto` it, fix the files listed by `tw status`, and
`tw amend`.
