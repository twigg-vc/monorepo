# CI List
Shows which `CI.json` files would be triggered if you pushed or submitted the current commit:
```
tw ci-list
```
(short form: `tw cil`)

Output, when a commit changes folders that own a `CI.json`:
```
c1 will affects the following CI jobs:
CI.json
subfolder/CI.json
```

Output, when nothing would trigger:
```
no CI job affected
```

You can also target a specific commit:
```
tw ci-list 1v0
```

This only previews `CI.json` triggering, not `CD.json`. See [How CI/CD jobs are triggered](../ci-cd/ci-cd.md#how-cicd-jobs-are-triggered) for the rule that decides which files fire.
