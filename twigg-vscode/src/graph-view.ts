import * as vscode from 'vscode';
import { diff } from './diff';
import { FileProvider } from './file-provider';
import { assignLanes } from './graph-layout';
import { errorText, gotoCommit, log } from './tw';

// How many commits below the current one are shown.
const numCommitsToShow = 25;

// Shows the commit tree in the Source Control panel.
//
// The tree is drawn by media/graph-view-script.js, which runs inside a
// webview. The webview cannot call this extension, so the two talk by
// posting messages:
//  1. The script posts "ready" as soon as it starts running.
//  2. This provider answers with a "rows" or an "error" message.
//  3. The script draws the rows, or the error.
//  4. The script posts "requestFiles" when a row is opened, and this
//     provider answers with a "files" message.
//  5. The script posts "openFileDiff" when a file is clicked.
//  6. The script posts "goto" when the button of a row is pressed.
export class GraphView implements vscode.WebviewViewProvider {

    public static readonly viewType = 'twigg.graph';

    constructor(extensionUri: vscode.Uri){
        this.extensionUri = extensionUri;
    }
    extensionUri: vscode.Uri;

    private view: vscode.WebviewView | undefined = undefined;

    resolveWebviewView(view: vscode.WebviewView) {
        this.view = view;
        view.webview.options = {
            enableScripts: true,
            localResourceRoots: [this.extensionUri],
        };
        view.webview.html = this.html(view.webview);
        view.webview.onDidReceiveMessage(message => {
            if (message.type === 'ready') {
                this.sendCommits(view.webview);
            } else if (message.type === 'requestFiles') {
                this.sendFiles(view.webview, message.commitId);
            } else if (message.type === 'goto') {
                this.goto(view.webview, message.commitId);
            } else if (message.type === 'openFileDiff') {
                openFileDiff(
                    message.path, message.commitId, message.parentId);
            }
        });
    }

    public refresh() {
        if (this.view === undefined || !this.view.visible) {
            return;
        }
        this.sendCommits(this.view.webview);
    }

    private sendCommits(webview: vscode.Webview) {
        const currentDir = workspaceDir();
        if (currentDir === undefined) {
            return;
        }
        var commits = undefined;
        try {
            commits = log(currentDir, numCommitsToShow);
        } catch (err) {
            webview.postMessage({ type: 'error', message: errorText(err) });
            return;
        }
        webview.postMessage({ type: 'rows', rows: assignLanes(commits) });
    }

    // A commit that cannot be read leaves the graph alone, because the rest
    // of it is still worth showing.
    private sendFiles(webview: vscode.Webview, commitId: string) {
        const currentDir = workspaceDir();
        if (currentDir === undefined) {
            return;
        }
        var files = undefined;
        try {
            files = diff(currentDir, commitId);
        } catch (err) {
            vscode.window.showErrorMessage(errorText(err));
            return;
        }
        webview.postMessage({
            type: 'files', commitId: commitId, files: files });
    }

    // Loads a commit and draws the graph again, because the commit the
    // workdir is on is one of the things it shows.
    private goto(webview: vscode.Webview, commitId: string) {
        const currentDir = workspaceDir();
        if (currentDir === undefined) {
            return;
        }
        try {
            gotoCommit(currentDir, commitId);
        } catch (err) {
            vscode.window.showErrorMessage(errorText(err));
            // The row is waiting on the graph to be drawn again.
            this.sendCommits(webview);
            return;
        }
        // The workdir changed, so the Source Control panel is stale as well.
        // The status command draws the graph again on its way out.
        vscode.commands.executeCommand('twigg.status');
    }

    private html(webview: vscode.Webview): string {
        // A webview cannot read files from disk, so every file it loads must
        // be turned into an uri it is allowed to fetch.
        const styleUri = webview.asWebviewUri(
            vscode.Uri.joinPath(this.extensionUri, 'media', 'graph.css'));
        const scriptUri = webview.asWebviewUri(
            vscode.Uri.joinPath(this.extensionUri, 'media', 'graph-view-script.js'));
        return `<!DOCTYPE html>
<html lang="en">
    <head>
        <meta charset="UTF-8">
        <meta http-equiv="Content-Security-Policy"
            content="default-src 'none'; style-src ${webview.cspSource};
                script-src ${webview.cspSource};">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <link href="${styleUri}" rel="stylesheet">
    </head>
    <body>
        <div id="graph"><div class="message">Loading commits…</div></div>
        <script src="${scriptUri}"></script>
    </body>
</html>`;
    }
}

// Directory the tw commands are run in.
function workspaceDir(): string | undefined {
    const workspaceFolders = vscode.workspace.workspaceFolders;
    if (!workspaceFolders) {
        vscode.window.showErrorMessage('No workspace folder is open.');
        return undefined;
    }
    return workspaceFolders[0].uri.fsPath;
}

// Diffs a file in a commit. parentId is empty for detached/root commits.
function openFileDiff(path: string, commitId: string, parentId: string) {
    const inCommit = FileProvider.Uri(path, commitId);
    if (parentId === '') {
        vscode.commands.executeCommand('vscode.open', inCommit);
        return;
    }
    vscode.commands.executeCommand('vscode.diff',
        FileProvider.Uri(path, parentId), inCommit,
        `${path} (${commitId})`);
}