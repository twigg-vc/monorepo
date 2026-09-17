import { Job, JobStatus } from './interfaces';

// Overall CI verdict for one commit version.
//  - "none": no CI job ran for this version
//  - "unfinished": at least one CI hasn't finished yet
//  - "failed": at least one didn't succeed
//  - "succeeded": every CI finished successfully
export type CiVerdict = "none" | "unfinished" | "failed" | "succeeded"

export interface CiSummary {
    Verdict: CiVerdict
    // Only the CIs whose latest run is not a success.
    NotSuccessful: Job[]
}

// Job statuses that mean the job hasn't finished yet.
const unfinishedStatuses: JobStatus[] = [
    "waiting-manual-start", "waiting", "queued", "posted", "running", "cancel",
]

// Computes the CI verdict of `version` from `jobs`.
export function SummarizeCi(jobs: Job[], version: number): CiSummary {
    const seenJobs = new Set<number>()
    const notSuccessful: Job[] = []
    var sawAny = false
    var sawUnfinished = false
    var sawFailed = false

    for (const job of jobs) {
        if (job.CommitVersion != version) { 
            continue
        }
        if (seenJobs.has(job.InternalId)) {
            continue
        }
        seenJobs.add(job.InternalId)
        sawAny = true
        
        if (job.Status == "success") { 
            continue
        }

        notSuccessful.push(job)
        if (unfinishedStatuses.includes(job.Status)) {
            sawUnfinished = true
        } else {
            sawFailed = true
        }
    }
    var verdict: CiVerdict = undefined
    if (!sawAny) {
        verdict = "none"
    } else if (sawFailed) {
        verdict = "failed"
    } else if (sawUnfinished) {
        verdict = "unfinished"
    } else {
        verdict = 'succeeded'
    }
    return { Verdict: verdict, NotSuccessful: notSuccessful }
}
