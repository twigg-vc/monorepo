// Runs inside the webview of the Twigg Graph view. See src/graph-view.ts for
// the messages exchanged with the extension.
(function () {
    const vscode = acquireVsCodeApi();

    // Sizes of the drawing, in pixels. The height of a row is set by the
    // drawing, so that the lines of two rows meet.
    const laneWidth = 14;
    const rowHeight = 22;
    const dotRadius = 3.5;


    // The element holding the files of an open commit, by commit id.
    const openFilesByCommitId = new Map();

    // The file row that is shown in the diff editor.
    var openFileRow = undefined;

    // Set while a goto is running, so that a second one cannot be started
    // before the graph is drawn again.
    var isGotoRunning = false;

    window.addEventListener('message', event => {
        const message = event.data;
        if (message.type === 'rows') {
            drawRows(message.rows);
        } else if (message.type === 'files') {
            drawFiles(message.commitId, message.files);
        } else if (message.type === 'error') {
            drawError(message.message);
        }
    });

    // Replaces the whole graph with the rows, and scrolls to the commit the
    // workdir is on, which may be far below the newest one.
    function drawRows(rows) {
        const graph = document.getElementById('graph');
        const numLanes = laneCount(rows);
        const elements = [];
        var currentElement = undefined;
        openFilesByCommitId.clear();
        openFileRow = undefined;
        isGotoRunning = false;
        for (const row of rows) {
            const element = commitRow(row, numLanes);
            if (row.commit.IsCurrent) {
                currentElement = element;
            }
            elements.push(element);
        }
        graph.replaceChildren(...elements);
        if (currentElement !== undefined) {
            currentElement.scrollIntoView({ block: 'nearest' });
        }
    }

    // Every row is drawn as wide as the widest one, so that the lanes of all
    // rows line up.
    function laneCount(rows) {
        let count = 1;
        for (const row of rows) {
            if (row.lane + 1 > count) {
                count = row.lane + 1;
            }
        }
        return count;
    }

    // Middle of the lane, in the coordinates of the drawing.
    function laneX(lane) {
        return lane * laneWidth + laneWidth / 2;
    }

    // Draws the lines and the dot of one row. The lines of a row end where
    // the lines of the row below start, so together they run down the graph.
    function laneDrawing(row, numLanes) {
        const drawing = svgElement('svg');
        drawing.setAttribute('class', 'lanes');
        drawing.setAttribute('width', numLanes * laneWidth);
        drawing.setAttribute('height', rowHeight);
        // Lines of the commits that are not in this row, from top to bottom.
        for (const lane of row.crossingLanes) {
            drawing.appendChild(
                line(laneX(lane), 0, laneX(lane), rowHeight));
        }
        // Lines coming down from the children, ending at the commit.
        for (const lane of row.childLanes) {
            drawing.appendChild(
                line(laneX(lane), 0, laneX(row.lane), rowHeight / 2));
        }
        // Line leaving towards the parent.
        if (row.hasParent) {
            drawing.appendChild(line(
                laneX(row.lane), rowHeight / 2, laneX(row.lane), rowHeight));
        }
        drawing.appendChild(
            dot(laneX(row.lane), rowHeight / 2, row.commit.IsCurrent));
        return drawing;
    }

    function line(x1, y1, x2, y2) {
        const line = svgElement('line');
        line.setAttribute('x1', x1);
        line.setAttribute('y1', y1);
        line.setAttribute('x2', x2);
        line.setAttribute('y2', y2);
        return line;
    }

    function dot(x, y, isCurrent) {
        const dot = svgElement('circle');
        dot.setAttribute('cx', x);
        dot.setAttribute('cy', y);
        dot.setAttribute('r', dotRadius);
        if (isCurrent) {
            dot.setAttribute('class', 'current');
        }
        return dot;
    }

    // Svg elements are only drawn when created in the svg namespace.
    function svgElement(name) {
        return document.createElementNS('http://www.w3.org/2000/svg', name);
    }

    // Opens the files of a commit under its row, or closes them when they
    // are already open.
    function toggleFiles(commit, row) {
        const open = openFilesByCommitId.get(commit.Id);
        if (open !== undefined) {
            open.element.remove();
            openFilesByCommitId.delete(commit.Id);
            return;
        }
        const files = document.createElement('div');
        files.className = 'files message';
        files.textContent = 'Loading files…';
        row.after(files);
        openFilesByCommitId.set(commit.Id, { element: files, commit: commit });
        vscode.postMessage({ type: 'requestFiles', commitId: commit.Id });
    }

    function drawFiles(commitId, files) {
        const open = openFilesByCommitId.get(commitId);
        if (open === undefined) {
            // The row was closed again before the files arrived.
            return;
        }
        open.element.className = 'files';
        open.element.replaceChildren(
            ...files.map(file => fileRow(file, open.commit)));
    }

    // The id of the parent is sent along, because it is the other side of the
    // diff the file is opened in.
    function fileRow(file, commit) {
        const row = document.createElement('div');
        row.className = 'file';
        row.addEventListener('click', () => {
            openFileRow = selectFileRow(row, openFileRow);
            vscode.postMessage({
                type: 'openFileDiff',
                path: file.Path,
                commitId: commit.Id,
                parentId: commit.ParentId,
            });
        });

        const path = document.createElement('span');
        path.className = 'file-path';
        path.textContent = file.Path;
        row.appendChild(path);

        const status = document.createElement('span');
        status.className = 'file-status file-' + file.Status;
        status.textContent = file.Status;
        row.appendChild(status);

        return row;
    }

    function selectFileRow(row, wasOpen) {
        if (wasOpen !== undefined) {
            wasOpen.className = 'file';
        }
        row.className = 'file open';
        return row;
    }

    // Replaces the whole graph with the reason tw could not answer.
    function drawError(text) {
        const graph = document.getElementById('graph');
        const error = document.createElement('div');
        error.className = 'message error';
        error.textContent = text;
        graph.replaceChildren(error);
    }

    // The elements are built one by one, instead of from a string of html,
    // so that a commit message is never read as html.
    function commitRow(graphRow, numLanes) {
        const commit = graphRow.commit;
        const row = document.createElement('div');
        if (commit.IsCurrent) {
            row.className = 'row current';
        } else {
            row.className = 'row';
        }
        row.appendChild(laneDrawing(graphRow, numLanes));
        row.addEventListener('click', () => toggleFiles(commit, row));

        const id = document.createElement('span');
        id.className = 'id';
        id.textContent = '#' + commit.Id;
        row.appendChild(id);

        if (commit.ServerId !== '') {
            const serverId = document.createElement('span');
            serverId.className = 'server-id';
            serverId.textContent = commit.ServerId;
            row.appendChild(serverId);
        }

        const title = document.createElement('span');
        title.className = 'title';
        title.textContent = commit.Message;
        row.appendChild(title);

        for (const name of badges(commit)) {
            const badge = document.createElement('span');
            badge.className = 'badge badge-' + name.toLowerCase();
            badge.textContent = name;
            row.appendChild(badge);
        }

        row.appendChild(gotoButton(commit));
        return row;
    }

    function gotoButton(commit) {
        const button = document.createElement('button');
        button.className = 'goto';
        button.textContent = 'goto';
        button.title = 'Load ' + commit.Id + ' into the working directory';
        button.addEventListener('click', event => {
            // The row would open its files otherwise.
            event.stopPropagation();
            if (isGotoRunning) {
                return;
            }
            isGotoRunning = true;
            button.className = 'goto spinner';
            button.textContent = '';
            vscode.postMessage({ type: 'goto', commitId: commit.Id });
        });
        return button;
    }

    // The states of a commit, named as `tw log` names them. A submitted
    // commit is only shown as submitted, like in the cli.
    function badges(commit) {
        if (commit.IsSubmitted) {
            return ['Submitted'];
        }
        const names = [];
        if (commit.HasConflicts) {
            names.push('Conflicts');
        }
        if (commit.IsPushed) {
            names.push('Pushed');
        }
        if (commit.IsHidden) {
            names.push('Hidden');
        }
        if (commit.IsObsolete) {
            names.push('Obsolete');
        }
        return names;
    }

    vscode.postMessage({ type: 'ready' });
}());