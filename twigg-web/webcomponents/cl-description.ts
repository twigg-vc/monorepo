import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { MdInput2, MdInputSubmit } from './md-input2';
import { GetCsrfHeaders } from './routes';

declare global {
    interface HTMLElementEventMap {
        "description-saved": CustomEvent<DescriptionSaved>;
    }
}
export interface DescriptionSaved {
    NewDescription: string
    Response: Response
}

/**
 * @fires description-saved CustomEvent<DescriptionSaved>
 * @description Emitted after the new description was posted successfully
 */
export class ClDescription extends LitElement {
    static properties = {
        canEdit: { type: Boolean },
        // Description content
        description: { type: String },
        // Url to which the new description is posted.
        // Request parameter names:
        // `description`: New commit description
        postDescriptionUrl: { type: String },
        // Indicates this element is in a loading state
        isLoading: { type: Boolean },
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        // Shown while the description is empty
        placeholder: { type: String },
    };
    constructor() {
        super();
        this.description = "";
        this.postDescriptionUrl = "about:blank"
        this.isLoading = false;
        this.canEdit = true;
        this.RepoOwnerName = "";
        this.RepoName = "";
        this.placeholder = "Enter the commit description here ...";
    }
    declare public canEdit: boolean
    declare public description: string
    declare public postDescriptionUrl: string
    declare public isLoading: boolean
    declare public placeholder: string
    declare private isEditing: boolean
    declare public RepoOwnerName: string
    declare public RepoName: string
    public descriptionParameterName = "description"

    render() {
        return html`
            <md-input2
            Content=${this.description}
            ContentPlaceholder=${this.placeholder}
            ?InputIsOpen=${this.description == ""}
            ?HasResetBtn=${this.description != ""}
            ?CloseInputBtnIsHidden=${this.description == ""}
            @md-input-submit=${this.onSubmit}
            SubmitBtnText="Save"
            ?OpenInputBtnIsHidden=${!this.canEdit}
            .RepoOwnerName=${this.RepoOwnerName}
            .RepoName=${this.RepoName}
            ?Highlighted=${this.description != ""}
            >
            </md-input2>
        `;
    }


    connectedCallback() {
        super.connectedCallback();
        // Open on edit mode if there's no description
        if (this.description.length == 0){
            this.isEditing = true;
        }
    }
    
    private async onSubmit(event: CustomEvent<MdInputSubmit>){
        this.isLoading = true;
        const target = event.target as MdInput2;

        const formData = new FormData();
        formData.append(
            this.descriptionParameterName,
            event.detail.NewContent);
        try {
            const resp = await fetch(this.postDescriptionUrl, {
                method: 'POST',
                body: formData,
                headers: GetCsrfHeaders(),
            });
            if (!resp.ok) {
                throw new Error(`request failed with status ${resp.status}`)
            }
            this.description = event.detail.NewContent
            this.dispatchEvent(new CustomEvent<DescriptionSaved>('description-saved', {
                detail: { NewDescription: this.description, Response: resp },
                bubbles: true,
                composed: true,
            }));
        } catch (error) {
            console.log("error submitting new desc:", error)
            target.StopLoading()
            alert("Failed to save description. Please try again.")
            return
        }

        target.UpdateContent(this.description)
    }

    static styles = [
        TwiggCss,
        css`
        `
    ];
}
customElements.define('cl-description', ClDescription);
declare global {
    interface HTMLElementTagNameMap {
        'cl-description': ClDescription;
    }
}