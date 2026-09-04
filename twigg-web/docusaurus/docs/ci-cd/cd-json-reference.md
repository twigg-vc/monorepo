# CD.json reference

A `CD.json` file can be placed anywhere in your repository (max size 256 kB). It describes one pipeline, or a list of pipelines, made of ordered **stages** that run sequentially. See [CI vs CD](./ci-cd.md#ci-vs-cd) for how this differs from `CI.json`, and [how CI/CD jobs are triggered](./ci-cd.md#how-cicd-jobs-are-triggered) for when it fires.

## Top-level shape

A single pipeline:
```
{ "Name": "...", "On": ["submit"], "Stages": [ ... ] }
```

Or a list of pipelines:
```
[
    { "Name": "pipeline-one", "On": ["submit"], "Stages": [ ... ] },
    { "Name": "pipeline-two", "On": ["manual"], "Stages": [ ... ] }
]
```

## Job fields

| Field | Type | Required | Notes |
|---|---|---|---|
| `Name` | string | yes | |
| `On` | array of strings | yes | only `"submit"` and/or `"manual"` (never `"push"`), max 10 entries |
| `Stages` | array of [stages](#stage-fields) | yes | at least 1, max 100 |

## Stage fields

| Field | Type | Required | Notes |
|---|---|---|---|
| `CanAutoStart` | boolean | yes | see [Sequential execution & manual approval gates](#sequential-execution--manual-approval-gates) |
| `Name` | string | yes | |
| `ImageName` | string | no | same values as CI, see [Images](./ci-json-reference.md#images) |
| `Steps` | array of steps | yes | same schema as CI steps, see [Step fields](./ci-json-reference.md#step-fields), max 100 |
| `TimeoutMilliSeconds` / `TimeoutSeconds` / `TimeoutMinutes` | number | yes | set exactly one of these three |

### Sequential execution & manual approval gates

Stages always run in the order they're declared — a stage only starts once the previous one has finished successfully.

- `"CanAutoStart": true` — the stage starts automatically as soon as the previous stage finishes.
- `"CanAutoStart": false` — the pipeline pauses after the previous stage finishes, and waits for a human to explicitly resume it before this stage runs.

This is what makes `CD.json` suitable for deployments: for example, an automatic build-and-test stage followed by a manually-gated production deploy stage.

## Full example

A pipeline with an automatic first stage and a manually-gated second stage:

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

A pipeline that only ever runs when manually started:

```
{
    "Name": "go-test",
    "On": ["manual"],
    "Stages": [
        {
            "CanAutoStart": false,
            "Name": "get code and test",
            "ImageName": "go",
            "Steps": [
                { "TemplateName": "get-code" },
                { "Run": "go test ./..." }
            ],
            "TimeoutMinutes": 30
        }
    ]
}
```

## Validation limits

| Limit | Value |
|---|---|
| `CD.json` file size | 256 kB |
| Stages per pipeline | 1–100 |
| Steps per stage | 100 |
| `On` entries per pipeline | 10 |
| Jobs created per commit (CI + CD combined) | 100 |

A stage occupies the same parallel-job slot as a CI job — see [Parallelism & concurrency](./concurrency-and-plans.md).
