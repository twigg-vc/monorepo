# Goto
Loads a commit into the working directory and makes it your current commit. Use any valid [commit syntax](./commit-syntax.md) or [alias](./commit-syntax.md#aliases):
```
tw goto <commit>
```
Examples:
```
tw goto 2
tw goto 2v0
tw goto c/7
tw goto down
```
Output:
```
switched to commit #2v0
```
To move one step at a time, see [up and down](./up-down.md). To jump to the last submitted commit, see [top](./top.md).

## Dirty working directory
`goto` refuses to run if you have uncommitted changes, because loading another commit would overwrite them:
```
workdir is dirty. Create a commit or use -f to discard
```
Either [commit](./commit.md) or [amend](./amend.md) your changes first, or pass `--force` (short form: `-f`) to discard them:
```
tw goto 2 --force
```
