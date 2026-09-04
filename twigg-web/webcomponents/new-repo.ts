import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { GetCsrfHeaders, HomeUrl, NewRepoDescriptionParameterName, NewRepoNameParameterName, NewRepoUrl, OrganizationNameParamName } from './routes';
import { HomePage } from './home-page';

/**
 * Simple span that shows the commit number with some style
 */
export class NewRepo extends LitElement {
    static properties = {
        Name: { type: String },
        Description: { type: String },
        OrgName: { type: String },
        nameErrMsg: { type: String, state: true }
    };

    declare Name: string;
    declare Description: string;
    declare OrgName: string;
    declare nameErrMsg: string;

    constructor() {
        super();
        this.Name = '';
        this.Description = '';
        this.OrgName = '';
        this.nameErrMsg = '';
    }

    render() {
        return html`
            <section class="card">
                <h2 class="title">Create a new repository</h2>
                <h3 class="name-err-msg">${this.nameErrMsg}</h3>
                    <form @submit=${this.onSubmit}>
                        <div class="field">
                            <label for="repo-name">Name</label>
                            <input id="repo-name" name=${NewRepoNameParameterName} />
                        </div>

                        <div class="field">
                            <label for="repo-desc">Description</label>
                            <textarea id="repo-desc" name=${NewRepoDescriptionParameterName}></textarea>
                        </div>

                        <button type="submit" class="create-btn">
                            <twigg-icon class="tab-icon" icon="DocumentPlus"></twigg-icon>
                            <span>Create new</span>
                        </button>
                    </form>
                <slot name="actions"></slot>
            </section>
        `;
    }

    private async onSubmit(event: Event){
        event.preventDefault();
        const form = event.currentTarget as HTMLFormElement;
        const data = new FormData(form);
        const name = String(data.get(NewRepoNameParameterName) ?? '').trim();
        const description = String(data.get(NewRepoDescriptionParameterName) ?? '').trim();

        if (name.length === 0 || name.length > 64) {
            this.nameErrMsg = "Name: must be 1 – 64 characters"
            return;
        }
        if (!/^[A-Za-z0-9_-]+$/.test(name)) {
            this.nameErrMsg = "Name: use only letters, numbers, _ and -"
            return;
        }
        if (description.length > 100) {
            this.nameErrMsg = "Description: must be 100 characters or less"
            return;
        }

        data.set(NewRepoNameParameterName, name);
        data.set(NewRepoDescriptionParameterName, description);
        if (this.OrgName) {
            data.set(OrganizationNameParamName, this.OrgName);
        }

        try {
            const resp = await fetch(NewRepoUrl, {
                method: 'POST',
                body: data,
                headers: GetCsrfHeaders(),
            });
            if (resp.ok){
                window.location.replace(HomeUrl);
            }
        } catch (error) {
            console.log("error submitting new desc:", error)
        }
    }

    static styles = [
        TwiggCss,
        css`
        .card {
            max-width: var(--size1);
            margin: var(--space6) auto;
            padding: var(--space5p);
            background: var(--color-surface);
            color: var(--color-text);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            box-shadow: var(--shadow-surface);
        }

        .title {
            margin-bottom: var(--space4);
        }

        .field {
            gap: var(--space2);
            margin-bottom: var(--space4);
        }

        label {
            color: var(--color-text);
        }

        input,
        textarea {
            width: 100%;
            background: var(--color-surface-alt);
            padding: var(--space2) var(--space3);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            color: var(--color-text);
            outline: none;
        }

        input::placeholder,
        textarea::placeholder {
            color: var(--color-text-muted);
        }

        textarea {
            resize: vertical;
            min-height: var(--fixedSpace7);
        }

        .create-btn{
            background: var(--color-primary);
            color: var(--color-text-on-primary);
            border-color: transparent;
            padding: var(--space0) var(--space2);
            font-size: var(--space4m);
            line-height: var(--line-height);
            border-radius: var(--radius3);
            display: inline-flex;
            align-items: center;
            gap: var(--space1);
            text-decoration: none;
            box-shadow: var(--shadow-surface);
        }

        .create-btn:hover{
            color: var(--color-text-on-primary);
            transform: translateY(-1px);
            box-shadow: var(--shadow-surface);
            text-decoration: none !important;
        }

        .name-err-msg {
            display: block;
            margin: 0 0 var(--space3);
            padding: var(--space2) var(--space3);
            border-radius: var(--radius1);
            background: var(--color-danger-soft);
            color: var(--color-danger);
            font: inherit;
            box-shadow: var(--shadow-surface);
        }

        .name-err-msg:empty {
            display: none; 
        }
    `];
}
customElements.define('new-repo', NewRepo);
declare global {
    interface HTMLElementTagNameMap {
        'new-repo': NewRepo;
    }
}