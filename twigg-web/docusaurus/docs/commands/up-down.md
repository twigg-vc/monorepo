# Up and Down
Move through the commit stack one step at a time. Both commands load the target commit into the working directory, like [goto](./goto.md).

## Down
Goes to the parent of the current commit:
```
tw down
```
Pass a number to go down several commits at once:
```
tw down 3
```
If you're already at the root commit, you'll see `parent not found`.

## Up
Goes to a child of the current commit:
```
tw up
```
Pass a number to go up several commits at once:
```
tw up 2
```
If the current commit has no children, you'll see `child not found`.

When a commit has more than one child, Twigg picks one for you:
 * A submitted child always wins.
 * Otherwise, the most recently created child that is still visible (latest version and not [hidden](./commit-syntax.md#aliases)).

To go to a specific child instead, use [goto](./goto.md) with its id.

## Dirty working directory
Like `goto`, `up` and `down` refuse to run if you have uncommitted changes. Pass `--force` (short form: `-f`) to discard them:
```
tw down --force
```
