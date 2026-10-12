# Status
Shows the changes in your working directory compared to the current commit, followed by the commit tree.
```
tw status
```
(short form: `tw st`)

Output example:
```
changes in working directory:
example.txt: modified
new.txt: created

current commit tree:
@ #1v0
| commit 1
* #0v0 c/0v0 Submitted
| [Initialize]
```
When there's nothing to commit, you'll see `no changes`.

## Conflicts
If the current commit has [conflicts](../resolving-conflicts.md), `status` first lists each conflicted file and whether you've resolved it:
```
conflicts:
example.txt has unresolved conflicts
changes in working directory:
no changes
```
