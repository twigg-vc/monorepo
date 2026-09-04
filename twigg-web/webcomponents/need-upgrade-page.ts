import { LitElement, html, css } from 'lit';
import { PlansPagePath } from './routes';

export class NeedUpgradePage extends LitElement {

  render() {
    return html`
      <main>
        <h1>Hi there! Your account is currently inactive — upgrade your plan to restore access.</h1>
        <a href="${PlansPagePath}">Plans</a>
      </main>
    `;
  }
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
}

customElements.define('need-upgrade-page', NeedUpgradePage);

declare global {
  interface HTMLElementTagNameMap {
    'need-upgrade-page': NeedUpgradePage;
  }
}
