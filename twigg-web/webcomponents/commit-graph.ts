import { LitElement, html, css } from 'lit';
import cytoscape, { Core } from 'cytoscape';
import { Commit } from './interfaces';
import { Fifo } from './queue';

export class CommitGraph extends LitElement {
    static properties = {
        PendingCommits: { type: Array },
        SubmittedCommits: { type: Array },
    };
    declare PendingCommits: Commit[];
    declare SubmittedCommits: Commit[];

    static styles = css`
        :host {
            display: block;
            position: relative;
        }
        #cy {
            width: 100%;
            height: 600px;
            border: 1px solid #ccc;
            position: relative;
        }
        #tooltip {
            display: none;
            position: absolute;
            transform: translate(0, -50%);
            pointer-events: none;
            z-index: 10;
            max-width: var(--size0);
            white-space: pre-wrap;
            background: var(--color-surface-alt);
            color: var(--color-text);
            border-radius: var(--radius0);
            padding: var(--space1) var(--space2);
        }
    `;

    private cy?: Core;

    firstUpdated() {
        // Crate lists and maps that will be used in the next lines
        var allCommits: Commit[] = []
        for (let i = 0; i<this.PendingCommits.length; i++){
            allCommits.push(this.PendingCommits[i])
        }
        for (let i = 0; i < this.SubmittedCommits.length; i++) {
            allCommits.push(this.SubmittedCommits[i])
        }
        const commitById = new Map<number, Commit>();
        // The commit in the frontend does not have the children, so we must
        // build a map with all children of each commit. We could send the list
        // of children per commit; but we DON'T want to do that because we want
        // to keep the data exchanged (the commit) as light as possible for speed.
        const commitIdToChildren = new Map<number, Commit[]>();
        for (let i = 0; i < allCommits.length; i++) {
            commitById.set(allCommits[i].L, allCommits[i])

            // For each commit we attached the parent. 
            // The root commit has no parent, so it cannot be attached
            // as a child of any other commit. We skip it here.
            if (allCommits[i].L == 0){
                continue
            }
            if (!commitIdToChildren.has(allCommits[i].ParentL)){
                commitIdToChildren.set(allCommits[i].ParentL, [])
            }
            commitIdToChildren.get(allCommits[i].ParentL).push(allCommits[i])
        }

        // Compute the location of each commit
        const commitIdToDiscretePosition = new Map<number, DiscretePos>();
        const rect = this.renderRoot.querySelector('#cy').getBoundingClientRect()
        const xLength = rect.width
        const yLength = rect.height
        const discretization = 10
        const xStep = Math.floor(xLength / discretization);
        const yStep = Math.floor(yLength / discretization);
        const yMax = discretization
        const grid = new Grid();
        
        var rootsId: number[] = []
        const oldestSubmittedCommitId = this.SubmittedCommits[this.SubmittedCommits.length - 1].L
        rootsId.push(oldestSubmittedCommitId)
        for (let i = 0; i < allCommits.length; i++) {
            if (allCommits[i].L == oldestSubmittedCommitId){
                continue
            }
            if (!commitById.has(allCommits[i].ParentL)){
                rootsId.push(allCommits[i].L)
            }
        }
        for(let i=0; i<rootsId.length; i++){
            AddTree(rootsId[i],commitById,
                commitIdToChildren, yMax, grid, commitIdToDiscretePosition)
        }
        var edges: Edge[] = []
        for (let i = 0; i < allCommits.length; i++) {
            // The Root has no parent.
            if (allCommits[i].L == 0) {
                continue
            }
            if (!commitById.has(allCommits[i].ParentL)) {
                continue
            }
            edges.push({
                source: String(allCommits[i].ParentL),
                target: String(allCommits[i].L)
            })
        }
        type PhantomNode = {
            data: { id: string; label: string; isPhantom: 1 };
            position: { x: number; y: number };
        };

        const phantomNodes: PhantomNode[] = [];
        const phantomEdges: Edge[] = [];

        for (const rootId of rootsId) {
            const root = commitById.get(rootId);

            if (root.L !== 0 && !commitById.has(root.ParentL)) {
                const rootPos = commitIdToDiscretePosition.get(root.L)

                const phantomId = `missing-${root.L}`
                const px = rootPos.x
                const py = rootPos.y + 1

                let fx = px;
                while (grid.isTaken(fx, py)) {
                    fx += 1;
                }
                grid.take(fx, py);

                phantomNodes.push({
                    data: { id: phantomId, label: `c/${root.ParentL} not loaded`, isPhantom: 1 },
                    position: { x: fx * xStep, y: py * yStep },
                });

                phantomEdges.push({
                    source: phantomId,
                    target: String(root.L),
                });
            }
        }
        this.cy = cytoscape({
            container: this.renderRoot.querySelector('#cy') as HTMLElement,
            elements: {
                nodes: [
                    ...allCommits.map((c) => ({
                        data: {
                            id: String(c.L),
                            label: String(c.L),
                            isSubmitted: c.IsSubmitted ? 1 : 0,
                            message: c.Message,
                        },
                        position: {
                            x: commitIdToDiscretePosition.get(c.L)!.x * xStep,
                            y: commitIdToDiscretePosition.get(c.L)!.y * yStep,
                        },
                    })),
                    ...phantomNodes,
                ],
                edges: [
                    ...edges.map((e) => ({ data: e })),
                    ...phantomEdges.map((e) => ({ data: e })),
                ],
            },
            style: [
                {
                    selector: 'node',
                    style: {
                        'label': 'data(label)',
                        'background-opacity': 0,
                        'color': '#6b4eff',
                        'text-valign': 'center',
                        'text-halign': 'center',
                        'font-size': '12px',
                        'font-weight': 'bold',
                        'width': 40,
                        'height': 40,
                        'border-width': 2,
                        'border-color': '#6b4eff',
                        'border-style': 'dashed',
                    }
                },
                {
                    selector: 'edge',
                    style: {
                        'width': 2,
                        'line-color': '#aaa',
                        'target-arrow-shape': 'triangle',
                        'target-arrow-color': '#aaa',
                        'curve-style': 'bezier'
                    }
                },
                {
                    selector: 'node[isSubmitted = 1]',
                    style: {
                        'background-opacity': 1,
                        'background-color': '#6b4eff', // purple
                        'color': '#fff',
                        'border-width': 2,
                        'border-color': '#392c78',
                        'border-style': 'solid',
                    },
                },
                {
                    selector: 'node[isPhantom = 1]',
                    style: {
                        'label': 'data(label)',

                        // Wrap + break long tokens
                        'text-wrap': 'wrap',
                        'text-max-width': '80px',        // max text width in px

                        // Make the node fit the label (responsive)
                        'width': '70px',
                        'height': '9px',
                        'padding': '8px',

                        'background-color': '#999',
                        'background-opacity': 1,
                        'shape': 'round-rectangle',
                        'font-size': '10px',
                        'color': '#fff',
                        'border-width': 0,
                    },
                }
            ],
            layout: { name: 'preset' },
        });


        this.cy.on('tap', 'node', (evt) => {
            const data = evt.target.data();
            // Ignore phantom nodes
            if (data.isPhantom == 1) {
                return;
            }
            const L = Number(data.id);
            // Just append /c/{L} to the current page
            const base = window.location.pathname.replace(/\/$/, '');
            const url = `${base}/c/${L}`;
            window.open(url, "_blank", "noopener,noreferrer");
        });

        const cyEl = this.renderRoot.querySelector('#cy') as HTMLElement;
        const tooltipEl = this.renderRoot.querySelector('#tooltip') as HTMLElement;
        this.cy.on('mouseover', 'node', (evt) => {
            const data = evt.target.data();
            if (data.isPhantom == 1) {
                cyEl.style.cursor = 'not-allowed';
                return;
            }
            cyEl.style.cursor = 'pointer';

            if (!data.message) {
                return;
            }
            // Show the tooltip just right of the node, vertically
            // centered on it. All coordinates are relative to #cy.
            const nodeCenter = evt.target.renderedPosition();
            const nodeRightEdgeX = nodeCenter.x + evt.target.renderedOuterWidth() / 2;
            const gapPx = 4;
            tooltipEl.textContent = data.message;
            tooltipEl.style.left = `${nodeRightEdgeX + gapPx}px`;
            tooltipEl.style.top = `${nodeCenter.y}px`;
            tooltipEl.style.display = 'block';
        });
        this.cy.on('mouseout', 'node', (evt) => {
            cyEl.style.cursor = 'default';
            evt.target.removeClass('hover');
            tooltipEl.style.display = 'none';
        });
        // The tooltip does not follow the graph, so hide it when nodes move.
        this.cy.on('pan zoom', () => {
            tooltipEl.style.display = 'none';
        });
        this.cy.on('drag', 'node', () => {
            tooltipEl.style.display = 'none';
        });

        const fixPointerOffset = () => {
            if (!this.cy) {
                return
            }
            const anyCy = this.cy as any;
            anyCy.renderer?.()?.invalidateContainerClientCoordsCache?.();
            this.cy.resize();
        };

        window.addEventListener('scroll', fixPointerOffset, true);
    }

    render() {
        return html`
            <div id="cy"></div>
            <div id="tooltip"></div>
        `;
    }
}

customElements.define('commit-graph', CommitGraph);

type Edge = {
    source: string;
    target: string;
};

type DiscretePos = {
    x: number; // integer
    y: number // integer
};

// Helper class to store which "points" (int, int) on the grid are taken
export class Grid {
    private taken = new Set<string>();
    take(x: number, y: number): void {
        this.taken.add(this.key(x, y));
    }
    isTaken(x: number, y: number): boolean {
        return this.taken.has(this.key(x, y));
    }
    private key(x: number, y: number): string {
        return `${x},${y}`;
    }
}

// Adds the tree which has the root commit treeRootId to
// the grind and to commitIdToDiscretePos
function AddTree(treeRootId: number,
    commitById: Map<number, Commit>,
    commitIdToChildren: Map <number, Commit[]> ,
    maxDiscreteY: number,
    grid: Grid,
    commitIdToDiscretePosition: Map<number, DiscretePos>){

    let commitIdsQueue: Fifo<number> = new Fifo<number>();
    commitIdsQueue.push(treeRootId)
    var isFirst = true
    while (!commitIdsQueue.isEmpty()) {
        // Get commit and push all children to the queue
        const c = commitById.get(commitIdsQueue.pop())
        // Sort so that submitted commits are processed first
        const unsortedChildren = commitIdToChildren.get(c.L) ?? []
        const sortedChildren = unsortedChildren.sort((a, b) => {
            return Number(b.IsSubmitted) - Number(a.IsSubmitted);
        });
        
        for (let i = 0; i < sortedChildren.length; i++) {
            commitIdsQueue.push(sortedChildren[i].L)
        }
        // If commit is already placed, continue
        if (commitIdToDiscretePosition.has(c.L)) {
            continue
        }

        if (isFirst){
            var x = 0
            var y = maxDiscreteY
            while (grid.isTaken(x, y)) {
                // While taken, keep trying to the right
                x += 1
            }
            grid.take(x, y)
            commitIdToDiscretePosition.set(c.L, { x: x, y: y })
            isFirst = false
            continue
        }

        // Try to put the node on the top of the parent
        const parent = commitById.get(c.ParentL)
        const parentPosition = commitIdToDiscretePosition.get(parent.L)
        var x = parentPosition.x
        var y = parentPosition.y - 1
        while (grid.isTaken(x, y)) {

            // While taken, keep trying to the right
            x += 1
        }
        grid.take(x, y)
        commitIdToDiscretePosition.set(c.L, { x: x, y: y })
    }
}