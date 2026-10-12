# Top
Goes to the last submitted commit. That's the tip of the repository's main history, which is where you usually want to start a new stack of commits.
```
tw top
```
Output:
```
switched to commit #5v1 c/12v1
```
If you're already there, you'll see `you're already at the top`.

`top` loads the commit into the working directory, like [goto](./goto.md). Twigg finds the commit by walking down from your current commit to the nearest submitted one, then up through submitted children until there are none left.

## Dirty working directory
`top` refuses to run if you have uncommitted changes. Pass `--force` (short form: `-f`) to discard them:
```
tw top --force
```

`top` is also an [alias](./commit-syntax.md#aliases) you can pass to other commands, for example `tw rebase top`.
