import { expect } from '@open-wc/testing';
import { SummarizeCi } from './ci-summary';
import { Job, JobStatus } from './interfaces';

var nextId = 1000
function job(name: string, version: number, status: JobStatus): Job {
    nextId--
    return {
        InternalId: nextId,
        RepoId: 1,
        Commit: 7,
        CommitVersion: version,
        Path: "ci",
        Name: name,
        RunNumber: 1,
        Status: status,
        CreatedTime: "",
        Id: `${nextId}`,
    }
}

describe('SummarizeCi', () => {
    it('returns none when the version has no jobs', () => {
        const jobs = [job("build", 0, "fail")]
        const s = SummarizeCi(jobs, 1)
        expect(s.Verdict).to.equal("none")
        expect(s.NotSuccessful).to.deep.equal([])
    });

    it('returns succeeded when every job succeeded', () => {
        const jobs = [job("build", 1, "success"), job("test", 1, "success")]
        const s = SummarizeCi(jobs, 1)
        expect(s.Verdict).to.equal("succeeded")
        expect(s.NotSuccessful).to.deep.equal([])
    });

    it('returns failed when a job did not succeed', () => {
        const failed = job("test", 1, "fail")
        const jobs = [job("build", 1, "success"), failed]
        const s = SummarizeCi(jobs, 1)
        expect(s.Verdict).to.equal("failed")
        expect(s.NotSuccessful).to.deep.equal([failed])
    });

    it('returns unfinished when a job has not finished', () => {
        const running = job("test", 1, "running")
        const jobs = [running, job("build", 1, "success")]
        const s = SummarizeCi(jobs, 1)
        expect(s.Verdict).to.equal("unfinished")
        expect(s.NotSuccessful).to.deep.equal([running])
    });

    it('failed takes precedence over unfinished', () => {
        const jobs = [job("test", 1, "running"), job("build", 1, "fail")]
        const s = SummarizeCi(jobs, 1)
        expect(s.Verdict).to.equal("failed")
        expect(s.NotSuccessful.length).to.equal(2)
    });

    it('treats a cancelled job as unfinished', () => {
        const jobs = [job("test", 1, "cancel")]
        const s = SummarizeCi(jobs, 1)
        expect(s.Verdict).to.equal("unfinished")
    });

    it('counts every run, not only the latest one', () => {
        const older = job("test", 1, "fail")
        const jobs = [job("test", 1, "success"), older]
        const s = SummarizeCi(jobs, 1)
        expect(s.Verdict).to.equal("failed")
        expect(s.NotSuccessful).to.deep.equal([older])
    });

    it('counts a job listed twice only once', () => {
        const failed = job("test", 1, "fail")
        const s = SummarizeCi([failed, failed], 1)
        expect(s.NotSuccessful).to.deep.equal([failed])
    });

    it('ignores jobs of other versions', () => {
        const jobs = [job("test", 2, "fail"), job("test", 1, "success")]
        const s = SummarizeCi(jobs, 1)
        expect(s.Verdict).to.equal("succeeded")
    });
});
