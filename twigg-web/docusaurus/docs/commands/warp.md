# Warp
Makes a commit your current commit **without** touching the working directory. Use any valid [commit syntax](./commit-syntax.md):
```
tw warp <commit>
```
Output:
```
switched to commit #2v0 (workding directory was not modified)
```
Compare with [load](./load.md), which loads the files but doesn't change the current commit.

Because the files stay as they are, `tw status` will show the difference between your working directory and the commit you warped to. This is useful when you want to turn your current changes into an amend of a different commit:
```
tw warp 3
tw amend
```
`warp` works even when the working directory is dirty, so there is no `--force` flag.
