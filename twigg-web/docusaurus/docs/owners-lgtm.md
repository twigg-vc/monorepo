---
title: OWNERS and LGTM
---

This document explains how owners LGTM works in Twigg and how to configure it using OWNERS files.

## Overview
In Twigg, every commit requires at least one LGTM to be submitted.

In addition, commits that modify files governed by an `OWNERS` file must receive an OWNERs LGTM.
An OWNERs LGTM is an approval from a user listed in the `OWNERS` file of the modified directory or any of its parent directories.

The approving owner(s) must collectively cover all modified paths. This means the OWNERs LGTM may come from a single owner whose scope includes all changes, or from multiple owners whose scopes together cover every modified file.

This model enables fine-grained ownership and approval control across the repository.


## The `OWNERS` File

An `OWNERS` file is a plain text file that lists usernames, one per line.

Example:
```
alice
bob
```

Each listed user is an owner of:
- The directory where the `OWNERS` file exists
- **All child directories**

## Example Repository Structure
```
/
├── OWNERS
├── utils/
│   ├── OWNERS
│   └── math/
│       └── calc/
│           └── OWNERS
└── tests/
    └── OWNERS
```
| Path	| text |
|-----|------|
|Root `OWNERS`| `alice`|
|`utils/OWNERS`| `bob`|
|`utils/math/calc/OWNERS`| `charlie`|
|`tests/OWNERS`| `david`|


### Effective Ownership
| Path	| Valid owners |
|-----|------|
|/| alice|
|/utils/| alice, bob|
|/utils/math/	| alice, bob|
|/utils/math/calc/| alice, bob, charlie|
|/tests/| alice, david|

### Examples   
#### 1. Change in a deep subfolder

Changed path:
```
utils/math/calc/file.txt
```

**Added LGTM**
- `alice` → **Ready to submit**: alice is an owner at the repository root, which covers all paths
- `bob` → **Ready to submit**: bob owns `utils/` and all of its subdirectories
- `charlie` → **Ready to submit**: charlie directly owns `utils/math/calc/`
- `david` → **Missing Owners LGTM**: david is not an owner of the modified path


#### 2. Change across different depths

Changed paths:
```
utils/math/calc/file.txt
utils/helper.txt
```

**Added LGTM**
- `alice` → **Ready to submit**: alice is an owner at the repository root, which covers all paths
- `bob` → **Ready to submit**: bob owns `utils/` and all of its subdirectories
- `charlie` → **Missing Owners LGTM**: charlie only owns `utils/math/calc/` and does not cover `utils/helper`
- `david` → **Missing Owners LGTM**: david is not an owner of either modified path



#### 3. Multi-folder change

Changed paths:
```
utils/math/calc/file.txt
tests/test_calc.txt
```

**Added LGTM**
- `alice` → **Ready to submit**: alice is an owner at the repository root, which covers all paths
- `bob` → **Missing Owners LGTM**: bob owns `utils/` but does not cover `tests/`
- `charlie` → **Missing Owners LGTM**: charlie owns `utils/math/calc/` but does not cover `tests/`
- `david` → **Missing Owners LGTM**: david owns `tests/` but does not cover `utils/`
- `charlie` + `david` → **Ready to submit**: charlie covers `utils/math/calc/` and david covers `tests/`, together covering all modified paths
- `bob` + `david` → **Ready to submit**: bob covers `utils/` and david covers `tests/`, together covering all modified paths