# Server
Sets the path to a repository on the Twigg server (https://twigg.vc):
```
tw server <OWNER>/<REPO-NAME>
```

Output:
```
server url updated to https://twigg.vc/<OWNER>/<REPO-NAME>
```

This tells Twigg which server-side repository this local project belongs to.

Example:
```
tw server my-user/my-repo
```
Now [push](./push.md) and [pull](./pull.md) commands will use `https://twigg.vc/my-user/my-repo` to perform their actions.