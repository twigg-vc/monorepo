# Push
Pushes your current commit and all parent commits until it reaches one that’s already submitted.
```
tw push
```
**If a commit has never been pushed before** (meaning it does not have a *server id*, [commit ids?](../tutorial/4-create-a-commit.md#commit-ids)), it will automatically appear in `Pending Commits` section. Otherwise will just update current commit and all of it parents.

**If a commit already exists on the server**, Twigg simply updates the server with the latest state of your current commit and increases its version.

Pushing can also trigger [CI jobs](../ci-cd/ci-cd.md) for any folder your commit changed.