# Work In Progress (WIP) commits

Twigg supports marking commits as **Work In Progress (WIP)** directly through the commit title.

A commit is considered **WIP** when its **title** starts with:
```
WIP
```
(case-insensitive), **as long as it is not immediately followed by a letter or number**.

## Examples:

These commits titles mark a commit as **WIP**:
```
WIP
wip
WIP add validation
wip: refactor commit logic
wip - temporary workaround
```
These commits titles **do NOT** mark commit as **WIP**: 
```
wip1
wipFix
wiper
fix wip issue
```

## How WIP affects the workflow

WIP commits behave like a normal pending commits, but with one important difference:
* **WIP commits cannot be submitted**.
* UI display **WIP tag**.
* You must remove WIP to submit the commit.

This prevents accidentally submitting unfinished work while still allowing you to:
* Stack more commits on top.
* Push the commit.
* Ask for early feedback.

## Removing WIP status

To remove the WIP status, simply **edit the commit title** and remove 'WIP' prefix:
```
tw amend "New commit title"
tw push
```
Once the commit title no longer starts with 'WIP', the commit can be submitted normally.
