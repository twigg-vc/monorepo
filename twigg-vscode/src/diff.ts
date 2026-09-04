import { run } from './tw';

// A file changed by a commit, as printed by `tw diff --json`. The field names
// are the ones the cli prints, so that nothing has to be translated.
export interface DiffFile {
    Path: string;
    // "created", "deleted" or "modified"
    Status: string;
}

// Reads the files a commit changed against its parent. commitId is in commit
// syntax, so that it is the same id the graph shows.
export function diff(cwd: string, commitId: string): DiffFile[] {
    const out = run(cwd, 'diff', commitId, '--json');
    return JSON.parse(out).Files;
}
