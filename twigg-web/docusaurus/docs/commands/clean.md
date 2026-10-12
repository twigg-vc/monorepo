# Clean
Discards all uncommitted changes by reloading the current commit's files into the working directory.
```
tw clean
```
Twigg first shows what would be discarded and asks for confirmation:
```
changes in working directory:
example.txt: modified
new.txt: created
this will discard the changes above. Continue? [y/N]
```
Answer `y` to continue. Anything else aborts with `clean aborted`.

On success:
```
workdir cleaned
```
If there's nothing to discard, you'll see `workdir is already clean`.

## Skipping the confirmation
Pass `--force` (short form: `-f`) to discard the changes without asking:
```
tw clean --force
```
Keep in mind that `clean` only affects the working directory. Commits are never touched, so anything you've already committed or amended stays safe.
