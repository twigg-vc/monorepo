# Log
Shows the commit tree around your current commit. Running `tw` with no command is the same as `tw log`.
```
tw log
```
Output example:
```
@ #1v0 c/1v0 Pushed
| My first commit!
* #0v0 c/0v0 Submitted
| [Initialize]
```
`@` marks your current commit. See [4. Create a commit](../tutorial/4-create-a-commit.md#see-your-current-commit) for how to read each line.

## Number of parents
By default, the tree starts 2 commits below the current one. Pass a number to start further down (at most 50):
```
tw log 10
```

## All versions
`--all` (short form: `-a`) also shows previous versions of each commit and how they changed (amend, rebase, restore):
```
tw log --all
```
Since `log` is the default command, `tw -a` does the same.
```
 @ #1v1
├╯ commit 1
|* #1v0 --[amend]--> #1v1
├╯ commit 1
* #0v0 c/0v0 Submitted
| [Initialize]
```

## JSON output
`--json` prints the tree as JSON, which is useful for scripts and editor integrations:
```
tw log --json
```
