import * as vscode from 'vscode';
import { runStatus, isRepository } from './commands';
import { FileProvider } from './file-provider';
import { GraphView } from './graph-view';

export function activate(context: vscode.ExtensionContext) {
    const twiggSCM = vscode.scm.createSourceControl(
        'twigg', 'Twigg', vscode.workspace.workspaceFolders![0].uri);
    context.subscriptions.push(twiggSCM);
    twiggSCM.inputBox.visible = false;  // Hide the commit input box
    const changes = twiggSCM.createResourceGroup('changes', 'Changes not committed');
    const parent = twiggSCM.createResourceGroup('parent', 'Changes in current commit');
    
    const fileProvider = new FileProvider();
	const fileProviderRegistration = vscode.workspace.registerTextDocumentContentProvider(
		FileProvider.scheme, fileProvider);
    context.subscriptions.push(fileProviderRegistration);
 
    const graphView = new GraphView(context.extensionUri);
    context.subscriptions.push(vscode.window.registerWebviewViewProvider(
        GraphView.viewType, graphView));

    vscode.workspace.onDidSaveTextDocument(document => {
        runStatus(twiggSCM, changes, parent);
        graphView.refresh();
    });

    vscode.window.onDidEndTerminalShellExecution(view => {
        runStatus(twiggSCM, changes, parent);
        graphView.refresh();
    });
    
	const statusCommand = vscode.commands.registerCommand('twigg.status', () => {
        // The progress is shown by the Source Control panel, over its title.
        return vscode.window.withProgress(
            { location: vscode.ProgressLocation.SourceControl },
            async () => {
                // hack: add an immediate promise because the following
                // statements run synchronously
                await new Promise(resolve => setTimeout(resolve, 0));
                if (!isRepository()) {
                    return;
                }
                runStatus(twiggSCM, changes, parent);
                graphView.refresh();
            });
	});
	context.subscriptions.push(statusCommand);

	// Execute once right after activation
	if (isRepository()){
        vscode.commands.executeCommand('twigg.status');
	}
}

// This method is called when your extension is deactivated
export function deactivate() {}