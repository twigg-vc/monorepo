# Pull
Pulls commits from the server to local.

Pull all submitted commits.
```
tw pull
```

Pull specific pending commit from server. Use the [server commit syntax](./commit-syntax.md#server-syntax).
```
tw pull <server commit>
```

## Staying on your current version
After pulling, if your working directory is clean, Twigg moves you to the newest version of your current commit (or to the pulled commit, when you pulled a specific one) and loads its files.

Use `--stay` to skip that. Your current commit and working directory are left exactly as they were:
```
tw pull --stay
tw pull c/7 --stay
```
This is useful when you want the server's updates locally but aren't ready to switch what you're working on.

If the working directory is dirty, Twigg never switches, with or without `--stay`.
