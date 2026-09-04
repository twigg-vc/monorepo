import * as vscode from 'vscode';
import path from 'path';
import { FileProvider } from './file-provider';
import { run } from './tw';

const fileProviderCurrentCommit = '';
const fileProviderParentCommit = 'parent';

function createResourceUri(repoRoot: string, filePathRelativeToRoot: string): vscode.Uri {
	const absolutePath = path.join(repoRoot, filePathRelativeToRoot);
	return vscode.Uri.file(absolutePath);
}

// Returns true if the open folder is an repository
export function isRepository(): boolean{
    const workspaceFolders = vscode.workspace.workspaceFolders;
    if (!workspaceFolders) {
        vscode.window.showErrorMessage('No workspace folder is open.');
        return false;
    }
    const currentDir = workspaceFolders[0].uri.fsPath;
    const output = run(currentDir, 'is-init');
    return output.replaceAll("\n", "") === "ok";
}

export function runStatus( 
    scm: vscode.SourceControl,changes: vscode.SourceControlResourceGroup,
    parent: vscode.SourceControlResourceGroup){
    changes.resourceStates = [];
    parent.resourceStates = [];
    scm.count = 0;

    const workspaceFolders = vscode.workspace.workspaceFolders;
    if (!workspaceFolders) {
        vscode.window.showErrorMessage('No workspace folder is open.');
        return;
    }
    const currentDir = workspaceFolders[0].uri.fsPath;
    const repoRoot = run(currentDir, 'root').replace(/\n$/, "");

    // Extracts filenames from CLI output using:
    // ^(.+)    -> Start of line; capture everything greedily (filename)
    // :        -> The final colon separator
    // \s*      -> Any optional whitespace
    // (?:...)  -> Matches status keywords without capturing them
    // $        -> End of line (via 'm' flag)
    const regex = /^(.+):\s*(?:modified|created|deleted)$/gm;

    const statusOut = run(currentDir, 'status');
    const statusFilenames = [...statusOut.matchAll(regex)].map(match => match[1]);
    for (const filename of statusFilenames) {
        const currentUri = createResourceUri(repoRoot, filename);
        const lastCommitUri = FileProvider.Uri(filename, fileProviderCurrentCommit);
        changes.resourceStates = changes.resourceStates.concat(
            {
                resourceUri: currentUri,
                command: {
                    title: 'Diff',
                    command: 'vscode.diff',
                    arguments: [lastCommitUri, currentUri]
                },
            }
        );
        scm.count += 1;
    }

    const diffOut = run(currentDir, 'diff');
    const diffFilenames = [...diffOut.matchAll(regex)].map(match => match[1]);
    for (const filename of diffFilenames) {
        const lastCommitUri = FileProvider.Uri(filename, fileProviderCurrentCommit);
        const parentCommitUri = FileProvider.Uri(filename, fileProviderParentCommit);
        const displayUri = createResourceUri(repoRoot, filename);
        parent.resourceStates = parent.resourceStates.concat(
            {
                resourceUri: displayUri,
                command: {
                    title: 'Diff',
                    command: 'vscode.diff',
                    arguments: [parentCommitUri, lastCommitUri]
                },
            }
        );
    }

}