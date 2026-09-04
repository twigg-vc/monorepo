import DocCardList from '@theme/DocCardList';

# CI/CD

Twigg can run jobs for you automatically whenever you push or submit a commit. There is no separate CI/CD UI to configure: you just commit a `CI.json` and/or `CD.json` file anywhere in your repository, and Twigg takes care of the rest.

## CI vs CD

Both `CI.json` and `CD.json` describe jobs made of one or more **steps** (shell commands run in sequence), but they're built for different purposes:

| | CI (`CI.json`) | CD (`CD.json`) |
|---|---|---|
| Structural unit | a flat list of independent **jobs** | a list of named **pipelines**, each made of ordered **stages** |
| Steps per job/stage | multiple steps allowed | multiple steps allowed, per stage |
| Ordering | jobs run independently of each other | stages run **sequentially**, in the order they're declared |
| Manual approval | not supported | a stage can require a human to explicitly resume the pipeline before it continues |
| Allowed triggers | `push`, `submit` | `submit`, `manual` (never `push`) |
| Typical use | fast feedback on every push (tests, lint) | deployments (build → staging → production) |

In short: **CI** is a set of independent checks that give you quick feedback. **CD** is a pipeline of stages that need to happen in order, optionally pausing for a human to say "go ahead" — the natural shape for a deployment.

## Quick example

A minimal `CI.json` that runs on every submit:

```
{
    "Name": "log-hi",
    "On": ["submit"],
    "Steps": [
        { "Run": "echo HI!" }
    ],
    "TimeoutMinutes": 5
}
```

A `CD.json` pipeline with an automatic first stage and a manually-gated second stage:

```
{
    "Name": "sleep",
    "On": ["submit", "manual"],
    "Stages": [
        { "CanAutoStart": true,  "Name": "auto sleep for 30s", "Steps": [{"Run": "sleep 30"}], "TimeoutMinutes": 5 },
        { "CanAutoStart": false, "Name": "sleep for 30s after manual ok", "Steps": [{"Run": "sleep 30"}], "TimeoutMinutes": 5 }
    ]
}
```

See the [CI.json reference](./ci-json-reference.md) and [CD.json reference](./cd-json-reference.md) for the full schema.

## How CI/CD jobs are triggered

A `CI.json` or `CD.json` file "owns" the folder it lives in. It triggers whenever **anything changes anywhere inside that folder** — including nested subfolders — when comparing a commit to its parent. You don't need to edit the `CI.json`/`CD.json` file itself; changing any sibling file, or a file several levels deep in a subfolder, is enough to trigger it.

Folders with no changes at all (identical to the parent commit) are skipped entirely, so this stays cheap even in a large monorepo. If a single commit touches several folders that each own a `CI.json`/`CD.json`, every one of them fires — up to a hard limit of 100 jobs created per commit.

### Previewing which jobs will run

Before pushing, you can see which `CI.json` files would be triggered by your current commit:

```
tw ci-list
```

(short form: `tw cil`). This only previews `CI.json` triggering, not `CD.json`. See the [`ci-list` command reference](../commands/ci-list.md) for details.

## Where jobs run

Each job or stage runs in its own isolated environment, chosen with the `ImageName` field: a Docker container (`base`, `go`, `node-20`, `bun`) or an LXD virtual machine (`vm`). Twigg automatically injects a `TWIGG_TOKEN`, `COMMIT_ID` and `REPO_ID` environment variable into every step, and the `get-code` step template expands into the commands needed to pull your repository's code onto the runner. See the [CI.json reference](./ci-json-reference.md#step-fields) for details.

<DocCardList />
