import * as vscode from 'vscode';
import { execSync } from 'child_process';

function removeAnsiEscapeCodes(s: string): string {
    return s.replace(
        /\x1b\[[0-9;]*m/g, // Matches ANSI escape codes
        ''
    );
}

// Path of the tw binary, taken from the twigg.path setting.
function twPath(): string {
    const configured = vscode.workspace.getConfiguration('twigg').get<string>('path');
    if (configured) {
        return configured;
    } else {
        return 'tw';
    }
}

// Runs tw in cwd and returns exactly what it printed. Use it to read file
// contents, which must not be changed in any way.
// tw prints why it failed to stdout, so that text is thrown when it fails.
export function runRaw(cwd: string, ...args: string[]): string {
    const command = [twPath(), ...args].join(' ');
    try {
        return execSync(command, { cwd: cwd, encoding: 'utf-8' });
    } catch (err) {
        var reason = undefined;
        if (err instanceof Error && 'stdout' in err) {
            reason = removeAnsiEscapeCodes(String(err.stdout)).trim();
        } else {
            reason = '';
        }
        if (reason === '') {
            throw err;
        }
        throw new Error(`${command} failed: ${reason}`);
    }
}

// Same as runRaw, but without the ansi escape codes.
export function run(cwd: string, ...args: string[]): string {
    return removeAnsiEscapeCodes(runRaw(cwd, ...args));
}

// The text of an error thrown by run or runRaw.
export function errorText(err: unknown): string {
    if (err instanceof Error) {
        return err.message;
    }
    return String(err);
}

// A commit as printed by `tw log --json`. The field names are the ones the
// cli prints, so that nothing has to be translated.
export interface Commit {
    Id: string;
    ServerId: string;
    ParentId: string;
    Message: string;
    IsCurrent: boolean;
    IsSubmitted: boolean;
    IsPushed: boolean;
    HasConflicts: boolean;
    IsHidden: boolean;
    IsObsolete: boolean;
    HasDiffData: boolean;
    DiffDataLinesCreated: number;
    DiffDataLinesDeleted: number;
    DiffDataLinesModified: number;
}

// Reads the commit tree, newest commit first. numCommits is how many commits
// below the current one are read.
export function log(cwd: string, numCommits: number): Commit[] {
    const out = run(cwd, 'log', String(numCommits), '--json');
    return JSON.parse(out).Commits;
}

// Loads the commit into the working directory.
export function gotoCommit(cwd: string, commitId: string) {
    run(cwd, 'goto', commitId);
}
