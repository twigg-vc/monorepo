import * as vscode from 'vscode';
import { runRaw } from './tw';

// Reads a file as it is in a commit. The path of the uri is the file and the
// query is the commit, so that `twigg-commit:/a/b.go?3v1` is b.go as it is in
// commit 3v1.
export class FileProvider implements vscode.TextDocumentContentProvider{

    public static readonly scheme = 'twigg-commit';

    // Uri of the file as it is in the commit. commitId is in commit syntax,
    // and may be an alias such as `parent`. It is empty for the current
    // commit.
    public static Uri(path: string, commitId: string): vscode.Uri{
        return vscode.Uri.from({
            scheme: FileProvider.scheme,
            path: '/' + path,
            query: commitId,
        });
    }

    provideTextDocumentContent(uri: vscode.Uri): string {
        const workspaceFolders = vscode.workspace.workspaceFolders;
        if (!workspaceFolders) {
            vscode.window.showErrorMessage('No workspace folder is open.');
            return "";
        }
        const currentDir = workspaceFolders[0].uri.fsPath;

        const file = '"'+uri.path.replace(/^\//, '')+'"';
        var args = undefined;
        if (uri.query === ''){
            args = ['dump', file];
        } else {
            args = ['dump', file, uri.query];
        }
        return runRaw(currentDir, ...args);
    }
}
