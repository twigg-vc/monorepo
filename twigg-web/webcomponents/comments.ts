import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { Comment, Commit, Thread, User } from './interfaces';
import { CommentParameterName, GetCsrfHeaders, IsNotResolvedParamValue, IsResolvedParamValue, ResolvedParamName, UrlToPostToThread } from './routes';
import { IconName } from './icons';
import { MdInput2, MdInputSubmit } from './md-input2';
import { FormatDateTime } from './helpers';

// Events fired
declare global {
    interface HTMLElementEventMap {
        "new-thread": CustomEvent<Thread>;
    }
}
export interface NewComment {
    C: Comment
    ThreadIsResolved: boolean
    ChangedThreadResolveStatus: boolean
}
declare global {
    interface HTMLElementEventMap {
        "new-comment": CustomEvent<NewComment>;
    }
}

/**
* Element that shows MANY threads and a button to create a
* new one at the end
* 
* @fires new-thread - See `new-thread` above; fired when a thread is created.
* @fires new-comment - See `new-comment` above; fired when a comment is added
* to a thread.
*/
export class CommentThreads extends LitElement {
    declare public RepoOwnerName: string
    declare public RepoName: string
    declare public Commit: Commit
    // Url to which the comment will be posted to create a new thread.
    // The post request will have`comment` as the parameter name for the
    // comment. The returned html will be places in the list of threads.
    declare public newThreadPostUrl: string
    // All the threads
    declare public Threads: Thread[];
    declare CurrentUser: User;
    static properties = {
        newThreadPostUrl: { type: String },
        Threads: { type: Array },
        RepoName:  { type: String },
        RepoOwnerName: { type: String },
        Commit: { type: Object },
        CurrentUser: { type: Object },
    };
    constructor() {
        super();
        this.newThreadPostUrl = "";
        this.Threads = []
        this.RepoName = ""
    }
    render() {
        return html`
            <div>
                ${this.Threads.map((thread: Thread) => {
                    return html`
                    <div class="comment-thread-container">
                        <comment-thread
                        .Thread=${thread}
                        .RepoOwnerName=${this.RepoOwnerName}
                        .RepoName=${this.RepoName}
                        .Commit=${this.Commit}
                        .CurrentUser=${this.CurrentUser}
                        >
                        </comment-thread>
                    </div>
                    `
                    }
                )}
            </div>
            
            <div class="new-comment-input-container">
                <new-comment-input
                    postUrl=${this.newThreadPostUrl}
                    @md-input-submit=${this.onSubmit}
                    btnText="Add Comment"
                    btnIsCentered
                    btnIcon="ChatBubbleLeftRight"
                >
                </new-comment-input>
            </div>
        `;
    }

    private async onSubmit(event: CustomEvent<MdInputSubmit>) {
        event.stopPropagation()

        const target = (event.target as NewCommentInput);
        const formData = new FormData();
        const newCommentText = event.detail.NewContent;
        formData.append(CommentParameterName, newCommentText);
        // Thread doesnt exist yet so we just use "willChangeResolveStatus"
        // as the same thing as "IsResolved"
        if (target.willChangeResolveStatus){
            formData.append(ResolvedParamName, IsResolvedParamValue);
        }else{
            formData.append(ResolvedParamName, IsNotResolvedParamValue);
        }

        try {
            const resp = await fetch(this.newThreadPostUrl, {
                method: 'POST',
                body: formData,
                headers: GetCsrfHeaders(),
            });
            if (!resp.ok) {
                throw new Error(`request failed with status ${resp.status}`)
            }
            const newThread = (await resp.json() as Thread);
            const now = new Date();
            const isoString = now.toISOString();
            newThread.Comments = [{ 
                ThreadId: newThread.Id, 
                AuthorUsername: this.CurrentUser.Username, 
                Text: newCommentText, 
                T: isoString 
            }];
            this.dispatchEvent(new CustomEvent<Thread>("new-thread", {
                detail: newThread,
                bubbles: true,
                composed: true,
            }));
            target.Clear()
        } catch (error) {
            console.log("request failed", error)
            target.StopLoading()
            alert("Failed to post comment. Please try again.")
        }
    }

    static styles = [
        TwiggCss,
        css`
        .new-comment-input-container{
            padding: var(--space4) 0;
        }
        .comment-thread-container{
            padding: var(--space4) var(--space3) 0 var(--space3);
        }
        `
    ];
}
customElements.define('comment-threads', CommentThreads);
declare global {
    interface HTMLElementTagNameMap {
        'comment-threads': CommentThreads;
    }
}

/**
* Element that shows ONE thread (which contains many comments)
*/
export class CommentThread extends LitElement {
    declare RepoOwnerName: string;
    declare RepoName: string;
    declare Commit: Commit;
    declare Thread: Thread;
    declare CurrentUser: User;
    static properties = {
        Thread: { type: Object },
        RepoName: { type: String },
        RepoOwnerName: { type: String },
        Commit: { type: Object },
        CurrentUser: { type: Object }
    };

    render() {
        if (this.Thread.Type == "AddLGTM" || this.Thread.Type == "RemoveLGTM"){
            return html`
                <lgtm-thread .Thread=${this.Thread}>
                    <slot name="go-to" slot="go-to"></slot>
                </lgtm-thread>
            `;
        }

        return html`
            <div class="thread ${this.Thread.IsResolved ? "resolved" : "not-resolved"}">
                <slot name="go-to"></slot>
                ${this.renderTags()}

                <div>
                    ${this.Thread.Comments.map((cm: Comment) => {
                        return html`<comment-display
                            .Comment=${cm}>
                            </comment-display>`
                    })}
                </div>

                <new-comment-input
                    .threadIsResolved=${this.Thread.IsResolved}
                    @md-input-submit=${this.onSubmit}
                >   
                </new-comment-input>
            </div>
        `;
    }

    private renderTags(){
        return html`
        <div id="tags-container">
            <div id="tags-container-left"></div>
            <div id="tags-container-center"></div>
            <div id="tags-container-right">${this.renderResolvedStatusTag()}</div>
        </div>
        `
    }

    private renderResolvedStatusTag(){
        if (this.Thread.IsResolved){
            return html`<span class="resolved-unresolved-tag resolved-tag" > Resolved </span>`
        }
        return html`<span class="resolved-unresolved-tag unresolved-tag" > Unresolved </span>`
    }

    private async onSubmit(event: CustomEvent<MdInputSubmit>) {
        event.stopPropagation();
        const target = (event.target as NewCommentInput);
        const formData = new FormData();
        const newCommentText = event.detail.NewContent
        formData.append(CommentParameterName, newCommentText);

        let currentResolveStatus = ""
        if (this.Thread.IsResolved){
            currentResolveStatus = IsResolvedParamValue
        }else{
            currentResolveStatus = IsNotResolvedParamValue
        }
        let finalResolveStatus = currentResolveStatus
        if (target.willChangeResolveStatus){
            if (finalResolveStatus == IsResolvedParamValue) {
                finalResolveStatus = IsNotResolvedParamValue
            } else {
                finalResolveStatus = IsResolvedParamValue
            }
        }
        formData.append(ResolvedParamName, finalResolveStatus);
        var respOk = false
        var respErr = undefined
        try {
            const resp = await fetch(UrlToPostToThread(
                this.RepoOwnerName,this.RepoName, this.Commit.L,
                this.Thread.Id), {
                method: 'POST',
                body: formData,
                headers: GetCsrfHeaders(),
            });
            respOk = resp.ok
        } catch (error) {
            respOk = false
            respErr = error
        }
        if (!respOk) {
            console.log("request failed: ", respErr)
            target.StopLoading()
            alert("Failed to post comment. Please try again.")
            return
        }
        const now = new Date();
        const isoString = now.toISOString();
        let finalResolveStatusBool = finalResolveStatus == "1" ? true : false
        const newComment = {
            C:{
            ThreadId: this.Thread.Id,
            AuthorUsername: this.CurrentUser.Username,
            Text: newCommentText, T: isoString},
            ThreadIsResolved: finalResolveStatusBool,
            ChangedThreadResolveStatus: target.willChangeResolveStatus
        }
        this.dispatchEvent(new CustomEvent<NewComment>("new-comment", {
            detail: newComment,
            bubbles: true,
            composed: true,
        }));
        
        if (target.willChangeResolveStatus){
            this.Thread.IsResolved = !this.Thread.IsResolved
        }
        target.Clear()
    }

    static styles = [
        TwiggCss,
        css`
        .thread{
            margin-top: var(--space4);
            padding: var(--space2) var(--space4) var(--space2) var(--space4);
            background: var(--color-surface);
            border-radius: var(--radius1);
            border: 1px solid var(--color-border);
        }
        .thread.not-resolved{
            box-shadow: var(--shadow-pop-yellow);
            border: 3px solid var(--color-warning);
        }
        .thread.resolved{
            box-shadow: var(--shadow-surface);
        }
        #tags-container{
            width: 100%;
            display: flex;
        }
        #tags-container-left,
        #tags-container-center,
        #tags-container-right {
            flex: 1;
        }
        #tags-container-left{
            display: flex;
            justify-content: flex-start;
            align-items: flex-start;
        }
        #tags-container-center{
            display: flex;
            justify-content: center;
        }
        #tags-container-right{
            display: flex;
            justify-content: flex-end;
            align-items: flex-start;
        }
        .resolved-unresolved-tag{
            border-radius: 9999px;
            padding: var(--space1) var(--space2);
            font-size: var(--space3);
        }
        .resolved-tag{
            background-color: var(--color-surface-alt);
            color: var(--color-text-muted);
        }
        .unresolved-tag{
            background-color: var(--color-warning);
            color: var(--color-status-text);
            font-weight: var(--weight-bold);
        }
        `,
    ];
}
customElements.define('comment-thread', CommentThread);
declare global {
    interface HTMLElementTagNameMap {
        'comment-thread': CommentThread;
    }
}


/**
 * Element that shows ONE comment
 */
export class CommentDisplay extends LitElement {
    // Raw markdown content
    declare Comment: Comment
    static properties = {
        Comment: { type: Object },
    };
    constructor() {
        super();
    }

    render() {
        return html`
            <div class="user-name-tag">
                <username-tag username=${this.Comment.AuthorUsername}></username-tag>
                <span class="comment-time">${FormatDateTime(this.Comment.T)}</span>
            </div>
            <div class="user-comment">
                <md-display content=${this.Comment.Text}></md-display>
            </div>
        `;
    }

    static styles = [
        TwiggCss,
        css`
        .user-name-tag{
            display: flex;
            align-items: center;
            gap: var(--space2);
            margin-bottom: var(--space2);
        }
        .comment-time{
            color: var(--color-text-muted);
            font-size: var(--space3);
        }
        .user-comment{
            margin-bottom: var(--space4);
        }
        `,
    ];
}
customElements.define('comment-display', CommentDisplay);
declare global {
    interface HTMLElementTagNameMap {
        'comment-display': CommentDisplay;
    }
}


/**
* Element used to write a new comment. It starts out collapsed.
*
* @fires md-input-submit - CustomEvent<MdInputSubmit>
* @description Emitted when the user clicks "Save".
*/
export class NewCommentInput extends LitElement {
    // indicates the comment will resolve/unresolve the thread
    declare willChangeResolveStatus: boolean
    // indicates if the current thread is already resolved
    declare threadIsResolved: boolean
    // Text shown at the btn
    declare btnText: string
    // Centers the btn
    declare btnIsCentered: boolean
    // Icon shown at the btn
    declare btnIcon: IconName
    // Used to hide the resolve btn
    declare hideResolveBtn: boolean
    // Indicates loading state
    declare isLoading: boolean
    // True when the comment is being written
    declare isEditing: boolean
    // Text that appears on the "Save" btn (i.e. the last btn to the right)
    declare SaveBtnText: string

	static properties = {
        postUrl: { type: String },
        btnText: { type: String },
        btnIsCentered: { type: Boolean },
        btnIcon: { type: String },
        hideResolveBtn: { type: Boolean },
        SaveBtnText: { type: String },

        isLoading: { type: Boolean },
        content: { type: String },
        isEditing: { type: Boolean },
        willChangeResolveStatus: { type: Boolean },
        threadIsResolved: { type: Boolean },
	};
	constructor() {
		super();
        this.isLoading = false;
        this.isEditing = false;
        this.willChangeResolveStatus = false;
        this.threadIsResolved = false;
        this.btnText = "Reply"
        this.btnIcon = "ChatBubbleLeft"
        this.hideResolveBtn = false;
        this.SaveBtnText = "Save"
	}

    render() {
        return html`
            <div class="${this.isEditing ? "main editing" : "main"}">
                <md-input2
                    id=${this.mdInputId}
                    Content=""
                    ContentPlaceholder="Type your comment here..."
                    ?OpenInputBtnIsCentered=${this.btnIsCentered}
                    OpenInputBtnText=${this.btnText}
                    OpenInputBtnIcon=${this.btnIcon}
                    SubmitBtnText=${this.SaveBtnText}
                >
                ${this.renderResolveBtn()}
                </md-input2>
            </div>
        `;
    }

    // Reset to "empty" state
    public Clear(){
        this.willChangeResolveStatus = false
        const input = this.shadowRoot!.getElementById(this.mdInputId) as MdInput2
        input.UpdateContent("")
    }

    public StopLoading(){
        const input = this.shadowRoot!.getElementById(this.mdInputId) as MdInput2
        input.StopLoading()
    }

    mdInputId = "md-input"
    private renderResolveBtn(){
        if (this.hideResolveBtn){
            return html``
        }
        return html`
        <resolve-toggle
        .WillChangeResolveStatus=${this.willChangeResolveStatus}
        .IsCurrentlyResolved=${this.threadIsResolved}
        slot="extra-btn" @toggled=${this.toggleWillResolve}>
        </resolve-toggle>
        `
    }

    private toggleWillResolve(event) {
        event.stopPropagation();
        this.willChangeResolveStatus = !this.willChangeResolveStatus
        const el = this.shadowRoot?.getElementById(this.mdInputId) as MdInput2
        if (el.GetCurrentDisplayedContent() == ""){
            if (this.threadIsResolved){
                el.SetCurrentDisplayedContent("[unresolved]")
            }else{
                el.SetCurrentDisplayedContent("[resolved]")
            }
        }
    }

    static styles = [
        TwiggCss,
        css`
        .main.editing{
            padding: var(--space4);
        }
        `
    ];
}
customElements.define('new-comment-input', NewCommentInput);
declare global {
    interface HTMLElementTagNameMap {
        'new-comment-input': NewCommentInput;
    }
}