# Set server ID
Sets the *server id* the server will give to the next commit pushed to this repository. See [commit ids](../tutorial/4-create-a-commit.md#commit-ids).
```
tw set-server-id <number>
```
Output:
```
next server id set to 1000
```
Normally server ids just increase by 1 and you never need this. It's useful when migrating a repository and you want Twigg's commit numbers to continue from a known value, or to leave a gap.

## Rules
 * The number must be greater than the server's current next id. Server ids never go backwards.
 * It can be at most 10000 ahead of the current next id.
 * You need [server](./server.md) and [key](./key.md) configured, and write permission on the repository.

Example error, when the number is too small:
```
id (5) must be greater than the next id (12)
```
