# Parallelism & concurrency

A single commit can touch several folders at once, and each folder can own its own `CI.json`/`CD.json` — so one push or submit can create several jobs (up to 100 per commit). How many of those jobs actually run **at the same time** depends on your plan; the rest wait in a queue until a slot frees up.

## What counts as a parallel job

Every running CI job, and every running CD stage, occupies one parallel-job slot for your account. If you have more jobs ready to run than free slots, the extra ones queue and start as soon as a slot opens up (i.e. as soon as an earlier job finishes).

## Limits by plan

| Plan | Parallel jobs | Max duration per job | Combined running-timeout budget |
|---|---|---|---|
| Free / Trial | 1 | 15 minutes | 1 minute |
| Solo (1x concurrency) | 1 | 1 hour | 1 minute |
| Team (3x concurrency) | 3 | 3 hours | 3 hours |

The **combined running-timeout budget** is a second, separate ceiling: it caps the *sum* of the declared timeouts of jobs running in parallel at any given moment, in addition to the parallel-job count. This is why, for example, the Solo plan already allows job durations of up to 1 hour but still only lets a large-timeout job run essentially alone.

## Why more parallelism matters

In a monorepo, one commit can trigger jobs in several unrelated folders at once. With a single parallel-job slot, those jobs — and any jobs from separate pushes/submits that happen to land close together — run strictly one after another. With Team's 3x concurrency, three of them run at the same time instead: CI feedback comes back sooner, CD pipelines advance faster, and one team's slow build doesn't block another team's quick test job from starting.

## Upgrading

See the [Plans page](https://twigg.vc/plans) to compare plans and upgrade. Its "Nx concurrency" bullets refer to the parallel-job counts in the table above.
