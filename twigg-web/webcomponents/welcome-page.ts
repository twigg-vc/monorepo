import { LitElement, html, css } from 'lit';
import { DocumentationPage, HomeUrl } from './routes';

export class WelcomePage extends LitElement {
  static styles = css`
    main {
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        text-align: center;
        min-height: 70vh;
        padding: var(--space6);
        color: var(--color-text);
        background: var(--color-bg);
      }

      h1 {
        font-size: var(--space6);
        font-weight: var(--weight-bold);
        margin-bottom: var(--space3);
      }


      a {
        color: var(--color-primary);
        text-decoration: none;
        font-weight: var(--weight-semi-bold);
        box-shadow: var(--shadow-pop);
        padding: var(--space1) var(--space2);
        border-radius: var(--radius1);
        display: inline-block;
        margin: var(--space2) 0;
      }

      a:hover {
        text-decoration: underline;
        background-color: var(--color-surface);
      }
  `;

  render() {
    return html`
      <main>
        <h1>Welcome to Twigg!</h1>
        <a href="${DocumentationPage +"/category/tutorial"}">Docs tutorial</a>
        <a href="${HomeUrl}">Home</a>
      </main>
    `;
  }
}

customElements.define('welcome-page', WelcomePage);

declare global {
  interface HTMLElementTagNameMap {
    'welcome-page': WelcomePage;
  }
}