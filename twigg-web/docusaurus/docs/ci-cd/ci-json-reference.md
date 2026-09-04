# CI.json reference

A `CI.json` file can be placed anywhere in your repository (max size 256 kB). It describes one job, or a list of jobs, that run independently of each other whenever the folder it lives in changes — see [how CI/CD jobs are triggered](./ci-cd.md#how-cicd-jobs-are-triggered).

## Top-level shape

A single job:
```
{ "Name": "...", "On": ["push"], "Steps": [ ... ] }
```

Or a list of jobs:
```
[
    { "Name": "job-one", "On": ["push"], "Steps": [ ... ] },
    { "Name": "job-two", "On": ["submit"], "Steps": [ ... ] }
]
```

## Job fields

| Field | Type | Required | Notes |
|---|---|---|---|
| `Name` | string | yes | |
| `ImageName` | string | no | see [Images](#images), defaults to the base image |
| `Steps` | array of [steps](#step-fields) | yes | max 100 steps |
| `On` | array of strings | yes | only `"push"` and/or `"submit"` (never `"manual"`), max 10 entries |
| `TimeoutMilliSeconds` / `TimeoutSeconds` / `TimeoutMinutes` | number | yes | set exactly one of these three |

### Images

| `ImageName` | Runs in |
|---|---|
| `""` | alias for `"base"` |
| `"base"` | base Docker image |
| `"go"` | Docker image with the Go toolchain |
| `"node-20"` | Docker image with node-20 toolchain |
| `"bun"` | Docker image with the Bun/JS toolchain |
| `"vm"` | a full LXD virtual machine instead of a container |

### Timeouts

Set exactly one of `TimeoutMilliSeconds`, `TimeoutSeconds` or `TimeoutMinutes`. The value is also capped by your subscription plan's maximum job duration — see [Parallelism & concurrency](./concurrency-and-plans.md#limits-by-plan).

## Step fields

Each entry in `Steps` is one command run in sequence (a step failing aborts the rest of the job):

| Field | Type | Required | Notes |
|---|---|---|---|
| `TemplateName` | string | no | expands into a predefined sequence of steps, see [Step templates](#step-templates) |
| `Run` | string | no | a shell command |
| `Env` | map of string to string | no | extra environment variables for this step, max 50 entries |
| `Secrets` | array of strings | no | names of secrets to inject as env vars and scrub from logs, max 50 entries |
| `Dir` | string | no | working directory for this step, see [Working directory](#working-directory) |

### Step templates

- `get-code` — expands to:
  ```
  tw init
  tw key $TWIGG_TOKEN
  tw server $REPO_ID
  tw pull $COMMIT_ID
  ```
  This is the idiomatic first step of a job: it gets your repository's code onto the runner. Without it, the runner starts with an empty workdir.
- `debug-get-code` — same as `get-code`, but every command runs with `--debug`, useful for troubleshooting a job that fails while pulling code.

### Secrets

Secrets are configured per-repository in the repository's Settings page. Listing a secret's name in a step's `Secrets` field injects its value as an environment variable for that step and scrubs the value out of the job's logs.

### Working directory

If a step doesn't set `Dir`, it defaults to the folder containing the `CI.json` file. Steps produced by the `get-code` template always run in `.` (the runner's root workdir) regardless of where the `CI.json` lives, since they set up the checkout itself.

### Auto-injected environment variables

Every step automatically receives:

| Variable | Value |
|---|---|
| `TWIGG_TOKEN` | a short-lived token scoped to this job |
| `COMMIT_ID` | the commit being built, e.g. `c123v456` |
| `REPO_ID` | the repository id, e.g. `id/id` |

## Full examples

A job that runs on every submit:
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

A job that checks out the code and runs tests on every push:
```
{
    "Name": "test-on-push",
    "ImageName": "go",
    "On": ["push"],
    "Steps": [
        { "TemplateName": "get-code" },
        { "Run": "go test ./..." }
    ],
    "TimeoutMilliSeconds": 1200000
}
```

## Validation limits

| Limit | Value |
|---|---|
| `CI.json` file size | 256 kB |
| Steps per job | 100 |
| `Env` entries per step | 50 |
| `Secrets` entries per step | 50 |
| `On` entries per job | 10 |
| Jobs created per commit (CI + CD combined) | 100 |
