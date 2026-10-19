# Bug Get
Shows a bug of the repository, with its description and timeline:
```
tw bug-get b/12
```
The bug can be referenced as `12`, `b12` or `b/12`.

Outputs the bug description and events in a human readable format.

If the bug doesn't exist:
```
b/12 not found
```

The command needs the server and the API key to be configured (see
[server](server.md) and [key](key.md)). It works for users with write
permission on the repository, the same ones that can `push`.
