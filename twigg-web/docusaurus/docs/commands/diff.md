# Diff
Shows which files were created, deleted or modified between two commits.

With no arguments, compares the current commit with its parent:
```
tw diff
```
Output example:
```
example.txt: modified
new.txt: created
```

With one commit, compares that commit with its parent. Use any valid [commit syntax](./commit-syntax.md):
```
tw diff 2
tw diff c/7v1
```

With two commits, compares the first against the second:
```
tw diff 3 1
```

## Unified diff
`--all` (short form: `-a`) prints the full unified diff of every file instead of only the list of files:
```
tw diff --all
tw diff 3 1 --all
```

## JSON output
`--json` prints the list of files as JSON, which is useful for scripts and editor integrations:
```
tw diff --json
```
`--all` and `--json` can't be combined.
