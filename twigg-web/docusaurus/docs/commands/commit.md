# Commit
A commit can be understood as snapshot of all tracked files in the repository. Commits are the fundamental block in Twigg workflow, you first make changes you want then commit.
```
tw commit <commit title>
```
output example:
```
created commit #1v0
```

More on what is a commit [here](../tutorial/4-create-a-commit.md#what-is-a-commit)

## Commit only some paths
By default, `commit` snapshots every change in the working directory. Use `--filter` (short form: `-fi`) to include only the paths you list. Everything else stays as uncommitted changes in the working directory.
```
tw commit <commit title> --filter=<path>[, <path>...]
```
The paths are comma-separated and relative to the repository root. A path ending in `/` includes the whole directory.

Examples:
```
tw commit "Add parser" --filter=parser.go
tw commit "Add parser" --filter "parser.go, parser_test.go"
tw commit "Add docs" -fi docs/
```
If no listed path has changes, Twigg logs `nothing to commit`. You can list at most 100 paths.
