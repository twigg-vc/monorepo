# Commit Stacking

Twigg makes it easy to collaborate on *stacked commits*.
That workflow consists of making commits on top of each other while the base ones are under review.

By running `commit`, you create a commit on top of the current one.
Do this many times and you'll create a stack of commits.
If you need to amend a commit from the base of the stack, you just `goto` it
(`tw down` or `tw goto 1`, for example) and [amend](./commands/amend.md) it.
The descendent commits will be auto [rebased](./commands/rebase.md).