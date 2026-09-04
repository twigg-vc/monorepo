import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { IconName } from './icons';

declare global {
    interface HTMLElementEventMap {
        "md-input-submit": CustomEvent<MdInputSubmit>;
    }
}
export interface MdInputSubmit {
    NewContent: string
}

/**
 * Component that has a text input for typing markdown and renders the markdown
 * on top.
 * 
 * @fires md-input-submit CustomEvent<MdInputSubmit>
 * @description Emitted when the submit btn is clicked. Automatically puts
 * the element in a loading state.
 * 
 * @fires md-input-changed CustomEvent<MdInputSubmit>
 * @description Emitted when the content changes
 * 
 * Slots (with defaults provided):
 * reset-btn -> Reset button. Only shown if HasResetBtn.
 * submit-disabled-btn -> Submit button for when the submit is disabled.
 * submit-btn -> Submit button.
 * open-input-btn -> Button to open the input.
 * extra-btn -> Extra btns shown next (before) the submit/reset btns.
 */
export class MdInput2 extends LitElement {
    // Raw markdown text content
    declare public Content: string; 
    // Placeholder when the input is empty
    declare public ContentPlaceholder: string
    // If true, puts the component in a loading state
    declare public IsLoading: boolean
    // If true, the input to edit is open
    declare public InputIsOpen: boolean
    // If true, the input is disabled
    declare public InputIsDisabled: boolean
    // If true, shows a reset-btn slot
    declare public HasResetBtn: boolean
    // If true, shows a the submit-btn slot
    declare public HasSubmitBtn: boolean
    // If true, the icon at the top that shows the close icon is hidden
    declare public CloseInputBtnIsHidden: boolean
    // If true, the open-input-btn is centered.
    declare public OpenInputBtnIsCentered: boolean
    // If true, hides the open-input-btn.
    declare public OpenInputBtnIsHidden: boolean

    // Text used in the submit btn text (if the submit btn is not provided)
    declare public SubmitBtnText: string
    // Text used in the submit btn text (if the submit btn is not provided)
    declare public SubmitBtnIcon: IconName

    // Text used in the open btn text (if slot is not provided)
    declare public OpenInputBtnText: string
    // Text used in the open input btn text (if slot is not provided)
    declare public OpenInputBtnIcon: IconName
    static properties = {
        Content: { type: String },
        ContentPlaceholder: { type: String },
        IsLoading: { type: Boolean },
        InputIsOpen: { type: Boolean },
        InputIsDisabled: { type: Boolean },
        CloseInputBtnIsHidden: { type: Boolean },
        HasSubmitBtn: { type: Boolean },
        HasResetBtn: { type: Boolean },
        OpenInputBtnIsCentered: { type: Boolean },
        OpenInputBtnIsHidden: { type: Boolean },
        SubmitBtnText: { type: String },
        SubmitBtnIcon: { type: String },
        OpenInputBtnText: { type: String },
        OpenInputBtnIcon: { type: String },
        

        newContent: { type: String, state: true },
    };
    constructor(){
        super();
        this.Content = ""
        this.ContentPlaceholder = "Start typing ..."
        this.InputIsOpen = false
        this.IsLoading = false
        this.InputIsDisabled = false
        this.CloseInputBtnIsHidden = false
        this.HasSubmitBtn = true
        this.HasResetBtn = false
        this.OpenInputBtnIsCentered = false
        this.OpenInputBtnIsHidden = false
        this.SubmitBtnText = "Submit"
        this.SubmitBtnIcon = "Check"
        this.OpenInputBtnText = "Edit"
        this.OpenInputBtnIcon = "PencilIcon"
    }
    declare public newContent: string
    connectedCallback() {
        super.connectedCallback();
        this.newContent = this.Content
    }

    // Use this method to update the content after handling a submit event.
    // It updates the "original content" that was passed on creation, closes
    // the input and sets to not loading.
    public UpdateContent(content: string){
        this.Content = content
        this.newContent = content
        this.InputIsOpen = false
        this.IsLoading = false
    }
    // Returns the current content that is shown
    public GetCurrentDisplayedContent():string {
        return this.newContent
    }
    // Sets the content that is being shown
    public SetCurrentDisplayedContent(c: string) {
        this.newContent = c
    }
    // Clears the loading state but keeps the input open with its content.
    public StopLoading(){
        this.IsLoading = false
    }
    // Sets this as an empty input
    public Clear(){
        this.Content = ""
        this.newContent = ""
        this.InputIsOpen = false
        this.IsLoading = false
    }

    render() {
        return html`
        ${this.renderMain()}
        ${this.renderOpenInputBtn()}
        `
    }
    private renderMain(){
        if (!this.InputIsOpen && this.newContent.length == 0){
            return html``
        }
        if (this.IsLoading){
            return html`<simple-loader></simple-loader>`
        }
        return html`
        <div class="main ${this.InputIsOpen ? "open" : "closed"}">
            ${this.renderCloseIconContainer()}
            <div id="content-div">
                ${this.renderMarkdownDisplay()}
                ${this.renderTextInputArea()}
            </div>
            ${this.renderSlotsDiv()}
        </div>
        `
    }
    private renderMarkdownDisplay() {
        if (this.newContent.length == 0) {
            return html``
        }
        return html`
            <div class="md-display-container ${this.InputIsOpen ? "open" : "closed"}">
                <md-display .content=${this.newContent}>
                </md-display>
            </div>
        `
    }
    private renderCloseIconContainer(){
        if (!this.InputIsOpen) {
            return html``
        }
        if (this.CloseInputBtnIsHidden){
            return html`
                <div id="close-icon-container">
                    <twigg-icon
                    class="close-icon-or-placeholder"
                    icon="None"></twigg-icon>
                </div>
            `   
        }
        return html`
            <div id="close-icon-container">
                <twigg-icon
                class="close-icon-or-placeholder"
                id=close-icon
                icon="XMark" @click=${this.onCloseIconClick}></twigg-icon>
            </div>
        `
    }
    private renderTextInputArea() {
        if (!this.InputIsOpen){
            return html``
        }
        return html`
            <textarea
            placeholder=${this.ContentPlaceholder}
            .value=${this.newContent}
            .disabled=${this.InputIsDisabled}
            @input=${this.onInput}
            >
            </textarea>
        `
    }
    private renderSlotsDiv(){
        if (!this.InputIsOpen) {
            return html``
        }
        return html`
        <div id="btn-slots-container">
            <slot name="extra-btn"></slot>
            ${this.renderResetBtn()}
            ${this.renderSubmitBtn()}
        </div>
        `
    }
    private renderResetBtn(){
        if (!this.HasResetBtn){
            return html``
        }
        if (!this.InputIsOpen) {
            return html``
        }
        if (this.Content == this.newContent){
            return html``
        }
        return html`
            <slot @click=${this.onResetBtnClick} name="reset-btn">
                <button class="reset-btn">Reset</button>
            </slot>
        `
    }
    private renderSubmitBtn() {
        if (!this.HasSubmitBtn){
            return html``
        }
        if (!this.InputIsOpen) {
            return html``
        }
        if (!this.canSubmit()){
            return html`
            <slot name="submit-disabled-btn">
                <button class="disabled disabled-submit-btn">
                    <twigg-icon .icon=${this.SubmitBtnIcon}>
                    ${this.SubmitBtnText}</twigg-icon>
                </button>
            </slot>
        `  
        }
        return html`
            <slot @click=${this.onSubmitClick} name="submit-btn">
                <button class="enabled-submit-btn">
                    <twigg-icon .icon=${this.SubmitBtnIcon}>
                    ${this.SubmitBtnText}</twigg-icon>
                </button>
            </slot>
        `
    }
    private renderOpenInputBtn(){
        if (this.InputIsOpen || this.OpenInputBtnIsHidden) {
            return html``
        }
        return html`
        <div id="open-input-btn-container" class="${this.OpenInputBtnIsCentered ? "center" : ""}">
            <slot @click=${this.onOpenInputClick} name="open-input-btn">
                <button class="open-input-btn">
                    <twigg-icon .icon=${this.OpenInputBtnIcon}>
                    ${this.OpenInputBtnText}</twigg-icon>
                </button>
            </slot>
        </div>
        `
    }

    private onCloseIconClick(){
        this.newContent = this.Content
        this.InputIsOpen = false
    }
    private onOpenInputClick(){
        this.InputIsOpen = true
    }
    private onResetBtnClick(){
        this.newContent = this.Content
    }
    private onSubmitClick() {
        if (!this.canSubmit){
            return;
        }
        this.IsLoading = true;
        this.dispatchEvent(new CustomEvent<MdInputSubmit>('md-input-submit', {
            detail: {
                NewContent: this.newContent,
            },
            bubbles: true,
            composed: true
        }));
    }
    private onInput(event){
        this.newContent = event.target.value
        this.dispatchEvent(new CustomEvent<MdInputSubmit>('md-input-changed', {
            detail: {
                NewContent: this.newContent,
            },
            bubbles: true,
            composed: true
        }));
    }
    private canSubmit(): boolean {
        if (this.newContent == ""){
            return false
        }
        if (this.newContent == this.Content){
            return false
        }
        return true
    }

    static styles = [
        TwiggCss,
        css`
        .main{
            border-radius: var(--radius1);
            transition: transform 0.15s, box-shadow 0.15s;
        }
        .main.open{
            border: 1px solid var(--color-primary-pop);
            box-shadow: var(--shadow-pop);
        }
        /* .main.closed{
            border: 1px solid var(--color-border);
            box-shadow: var(--shadow-surface);
        } */
        #close-icon-container{
            display: flex;
            justify-content: flex-end;
        }
        .close-icon-or-placeholder{
            border-radius: 50%; /* makes it a circle */
            padding: var(--space0);
            cursor: pointer;
        }
        #close-icon:hover{
            background-color: var(--color-surface-alt)
        }
        #content-div{
            padding: 0px var(--space4) 0px var(--space4);
        }
        textarea{
            display: block;
            background-color: var(--color-surface-alt);
            border: 1px solid var(--color-border);
            outline: none;
            color: var(--color-text);
            width: 100%;
            min-height: 8em; /*min number of lines*/
            resize: vertical;
            font-family: monospace;
            font-size: var(--space4);
            overflow-wrap: break-word;
        }
        .md-display-container.closed{
            padding-top: 0px;
            padding-bottom: var(--space2);
            overflow-wrap: break-word;
        }
        .md-display-container.open{
            padding-top: 0px;
            padding-bottom: var(--space2);
            overflow-wrap: break-word;
        }
        #btn-slots-container{
            display:flex;
            justify-content:flex-end;
            align-items: center;
            padding: var(--space2) var(--space4);
            gap: var(--space2);
        }
        .disabled-submit-btn{
            background-color: var(--color-surface);
            font-size: var(--space5);
            color: var(--color-text-muted);
            border: var(--color-border);
        }
        .enabled-submit-btn{
            background-color: var(--color-surface-alt);
            color: var(--color-text);
            font-size: var(--space5);
        }
        .reset-btn{
            background-color: var(--color-surface);
            color: var(--color-text);
            font-size: var(--space5); 
        }
        #open-input-btn-container{
            display: flex;
            justify-content: flex-end;
            padding: 0px var(--space4);
        }
        #open-input-btn-container.center{
            justify-content: center
        }
        .open-input-btn{
            background-color: var(--color-surface);
            color: var(--color-text);
            font-size: var(--space5); 
        }
        `];
}
customElements.define('md-input2', MdInput2);
declare global {
    interface HTMLElementTagNameMap {
        'md-input2': MdInput2;
    }
}