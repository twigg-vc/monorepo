
# Twigg version control
This project uses Twigg for version control.
It's a version control for trunk-based development with stacked commits.

## Concepts
### Commit IDs

There are 2 type of commit Ids: `local` and `server`.
 * The *local id* (represented as `#<local id>` or `<local id>`) is a number that identifies a commit on someone's computer.
That number starts at 0 and increases by 1 for each commit created on that computer.
 * The *server id* (represented as `c/<server id>`) is also a number that starts
at 0 and is increased by 1 for every commit created on the server or pushed to it.

Thus, if two users create two commits on their machines, those commits will have
*local id* `1` and `2` on their machines. Once they push to the server, those commits
will get a *server id*. Note that the server Id **globally** identifies a commit.

### Commit versions

All commits have an immutable version. When a commit is created, it has version `0`.
When that commit is **modified** (amended, restored, rebased or submitted),
that version is increased by 1.

### Commit syntax

A commit can be identified by its **local ID** (just the number, e.g. `3`) or its **server ID** (prefixed with `c/` or `c`, e.g. `c/7` and `c7`). Both forms may optionally append a version with `v<number>`.

Examples:
- `3` — local commit 3, latest version
- `3v1` — local commit 3, version 1
- `c/7` — server commit 7, latest version
- `c/7v2` — server commit 7, version 2

### Conflicts
When performing commands like rebase, amend or restore, conflicts can occur. When this happens, Twigg automatically creates a new version of the commit and marks where the conflict happened.
To solve the conflicts, `goto` the commit, edit the files with conflicts (use `status` to see them), and `amend` (see the Commands section for detail).

## Commands

- `tw log <number-of-commits>` shows commit tree
- `tw status` shows workdir modifications
- `tw goto <target>` loads the target commit (using commit syntax) to the working dir, overwriting its state.
- `tw down` "checkout" to the parent commit
- `tw up` "goto" a child commit. It uses heuristics to choose a child when there are many.
- `tw commit <message>` create a commit
- `tw amend <optional: new-message>` amend the commit. Children are auto-rebased.
- `tw rebase <source> <target>` rebases source commit into target (both using commit syntax)
- `tw diff` shows the created/deleted/modified files between the current commit and its parent
- `tw diff <c1>` shows the created/deleted/modified files between commit c1 (using commit syntax) and its parent
- `tw diff <c1> <c2>` shows the created/deleted/modified files between commit c1 and c2 (both using commit syntax)
- `tw diff <c1> <c2> --all` shows the complete unified diff between commit c1 and c2 (both using commit syntax). "--all" can be used in all `diff` commands to show the unified diff instead of only the created/deleted/modified files
- `tw push` pushes the current commit and non-pushed ancestors
- `tw restore <commit-with-version>` creates a new version of the commit which is a copy of an old version. Commit syntax *with the version* must be used (e.g. `1v2` or `c/3v4`).
- `tw pull c/99` - "downloads" latest version of commit 99 from the server.