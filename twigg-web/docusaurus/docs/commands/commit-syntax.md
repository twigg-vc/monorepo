# Commit syntax

When running commands, a commit can be identified by its **local ID** (just the number, e.g. `3`) or its **server ID** (prefixed with `c/` or `c`, e.g. `c/7` and `c7`). Both forms may optionally append a version with `v<number>`.

## Examples
- `3` → local commit 3, latest version
- `3v1` → local commit 3, version 1
- `c/7` → server commit 7, latest version
- `c/7v2` → server commit 7, version 2

## Server syntax

Some commands expect a server commit. For those, use the `c/7` or `c/7v2` syntax.

## Aliases

Shortcuts for referencing specific commits.

 * `top` → Last submitted commit.    
Example: `tw rebase top` rebase the current commit onto the last submitted commit.
 * `down` →  Parent commit.  
Example: `tw goto down`, navigate to parent commit latest version.
 * `up` → One of the children of the current commit.
Example: `tw hide up`, hides a child commit. Note: when there's more than one child,
heuristics are used to determine which one is probably more relevant.