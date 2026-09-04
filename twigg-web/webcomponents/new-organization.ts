import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import {
    GetCsrfHeaders,
    NewOrganizationNameParamName,
    NewOrganizationPattern,
    PathToOrganization
} from './routes';
import { UsernameIsValid } from './interfaces';

export class NewOrganization extends LitElement {
    static properties = {
        nameErrMsg: { type: String, state: true },
        submitIsLoading: { type: Boolean, state: true }
    };

    declare nameErrMsg: string;
    declare submitIsLoading: boolean;

    constructor() {
        super();
        this.nameErrMsg = '';
        this.submitIsLoading = false;
    }

    render() {
        return html`
            <section class="card">
                <h2 class="title">Create a new organization</h2>
                
                ${this.nameErrMsg
                ? html`<span class="name-err-msg">${this.nameErrMsg}</span>`
                : ''}

                <form @submit=${this.submit}>
                    <div class="field">
                        <label for="org-name">Organization Name</label>
                        <input 
                            id="org-name" 
                            name=${NewOrganizationNameParamName} 
                            ?disabled=${this.submitIsLoading}
                            placeholder="my-org-name"
                            @input=${() => this.nameErrMsg = ''}
                        />
                        <small class="hint">Use only letters, numbers, hyphens, or underscores.</small>
                    </div>

                    <button type="submit" class="create-btn" ?disabled=${this.submitIsLoading}>
                        <twigg-icon class="tab-icon" icon="DocumentPlus"></twigg-icon>
                        <span>${this.submitIsLoading ? 'Creating...' : 'Create organization'}</span>
                    </button>
                </form>
            </section>
        `;
    }

    private async submit(event: Event) {
        event.preventDefault();
        if (this.submitIsLoading) return;

        const form = event.currentTarget as HTMLFormElement;
        const data = new FormData(form);
        const name = String(data.get(NewOrganizationNameParamName) ?? '').trim();

        const { isValid, errorMsg } = UsernameIsValid(name)
        if (!isValid) {
            this.nameErrMsg = errorMsg;
            return;
        }

        this.submitIsLoading = true;

        try {
            const resp = await fetch(NewOrganizationPattern, {
                method: 'POST',
                body: data,
                headers: GetCsrfHeaders(),
            });

            if (!resp.ok) {
                const errorMsg = await resp.text();
                throw new Error(errorMsg);
            }
            // Since the handler grants owner perms and commits, 
            // we redirect to the new organization's page.
            window.location.replace(PathToOrganization(name));
        } catch (error) {
            console.error("Error creating organization:", error);
            this.nameErrMsg = error || "Failed to create organization";
        } finally {
            this.submitIsLoading = false;
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
            display: flex;
            flex-direction: column;
            gap: var(--space1);
            margin-bottom: var(--space4);
        }

        .hint {
            color: var(--color-text-muted);
            margin-top: var(--space1);
        }

        label {
            font-weight: bold;
            color: var(--color-text);
        }

        input {
            width: 100%;
            background: var(--color-surface-alt);
            padding: var(--space2) var(--space3);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            color: var(--color-text);
            outline: none;
            box-sizing: border-box;
        }

        input:focus {
            border-color: var(--color-primary);
        }

        input:disabled {
            opacity: 0.6;
            cursor: not-allowed;
        }

        .create-btn {
            background: var(--color-primary);
            color: var(--color-text-on-primary);
            border: none;
            padding: var(--space2) var(--space4);
            font-size: var(--font-size-button);
            border-radius: var(--radius3);
            display: inline-flex;
            align-items: center;
            gap: var(--space2);
            cursor: pointer;
            transition: filter 0.2s;
        }

        .create-btn:hover:not(:disabled) {
            filter: brightness(1.1);
        }

        .create-btn:disabled {
            background: var(--color-disabled);
            cursor: not-allowed;
        }

        .name-err-msg {
            display: block;
            margin-bottom: var(--space4);
            padding: var(--space2) var(--space3);
            border-radius: var(--radius1);
            background: var(--color-danger-soft);
            color: var(--color-danger);
            border-left: 4px solid var(--color-danger);
        }
    `];
}

customElements.define('new-organization', NewOrganization);

declare global {
    interface HTMLElementTagNameMap {
        'new-organization': NewOrganization;
    }
}