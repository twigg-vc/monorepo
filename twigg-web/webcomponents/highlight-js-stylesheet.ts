import { css } from 'lit';

export const HighlightJsStyle = css`
/* ===== VS 2015 (dark) ===== */

/* Se quiser setar cor base sem sobrescrever background: */
/* :host-context(.dark) .hljs { color: #DCDCDC; } */

:host-context(.dark) .hljs-keyword,
:host-context(.dark) .hljs-literal,
:host-context(.dark) .hljs-symbol,
:host-context(.dark) .hljs-name {
  color: #569CD6;
}

:host-context(.dark) .hljs-link {
  color: #569CD6;
  text-decoration: underline;
}

:host-context(.dark) .hljs-built_in,
:host-context(.dark) .hljs-type {
  color: #4EC9B0;
}

:host-context(.dark) .hljs-number,
:host-context(.dark) .hljs-class {
  color: #B8D7A3;
}

:host-context(.dark) .hljs-string,
:host-context(.dark) .hljs-meta .hljs-string {
  color: #D69D85;
}

:host-context(.dark) .hljs-regexp,
:host-context(.dark) .hljs-template-tag {
  color: #9A5334;
}

:host-context(.dark) .hljs-subst,
:host-context(.dark) .hljs-function,
:host-context(.dark) .hljs-title,
:host-context(.dark) .hljs-params,
:host-context(.dark) .hljs-formula {
  color: #DCDCDC;
}

:host-context(.dark) .hljs-comment,
:host-context(.dark) .hljs-quote {
  color: #57A64A;
  font-style: italic;
}

:host-context(.dark) .hljs-doctag {
  color: #608B4E;
}

:host-context(.dark) .hljs-meta,
:host-context(.dark) .hljs-keyword, /* mantido para meta-keywords */
:host-context(.dark) .hljs-tag {
  color: #f500ff;
}

:host-context(.dark) .hljs-variable,
:host-context(.dark) .hljs-template-variable {
  color: #BD63C5;
}

:host-context(.dark) .hljs-attr,
:host-context(.dark) .hljs-attribute {
  color: #9CDCFE;
}

:host-context(.dark) .hljs-section {
  color: gold;
}

:host-context(.dark) .hljs-emphasis { font-style: italic; }
:host-context(.dark) .hljs-strong { font-weight: bold; }

:host-context(.dark) .hljs-bullet,
:host-context(.dark) .hljs-selector-tag,
:host-context(.dark) .hljs-selector-id,
:host-context(.dark) .hljs-selector-class,
:host-context(.dark) .hljs-selector-attr,
:host-context(.dark) .hljs-selector-pseudo {
  color: #D7BA7D;
}

:host-context(.dark) .hljs-addition {
  background-color: #144212;
  display: inline-block;
  width: 100%;
}

:host-context(.dark) .hljs-deletion {
  background-color: #600;
  display: inline-block;
  width: 100%;
}

/* ===== nnfx (light) ===== */

/* Se quiser cor base sem bg: */
/* :host-context(.light) .hljs { color: #000; } */

:host-context(.light) .language-xml .hljs-meta,
:host-context(.light) .language-xml .hljs-meta-string {
  font-weight: bold;
  font-style: italic;
  color: #48b;
}

:host-context(.light) .hljs-comment,
:host-context(.light) .hljs-quote {
  font-style: italic;
  color: #070;
}

:host-context(.light) .hljs-name,
:host-context(.light) .hljs-keyword,
:host-context(.light) .hljs-built_in {
  color: #808;
}

:host-context(.light) .hljs-name,
:host-context(.light) .hljs-attr {
  font-weight: bold;
}

:host-context(.light) .hljs-string {
  font-weight: normal;
}

:host-context(.light) .hljs-code,
:host-context(.light) .hljs-string,
:host-context(.light) .hljs-meta .hljs-string,
:host-context(.light) .hljs-number,
:host-context(.light) .hljs-regexp,
:host-context(.light) .hljs-link {
  color: #00f;
}

:host-context(.light) .hljs-title,
:host-context(.light) .hljs-symbol,
:host-context(.light) .hljs-bullet,
:host-context(.light) .hljs-variable,
:host-context(.light) .hljs-template-variable {
  color: #38b100;
}

:host-context(.light) .hljs-title.class_,
:host-context(.light) .hljs-class .hljs-title,
:host-context(.light) .hljs-type {
  font-weight: bold;
  color: #639;
}

:host-context(.light) .hljs-title.function_,
:host-context(.light) .hljs-function .hljs-title,
:host-context(.light) .hljs-attr,
:host-context(.light) .hljs-subst,
:host-context(.light) .hljs-tag {
  color: #000;
}

:host-context(.light) .hljs-formula {
  background-color: #eee;
  font-style: italic;
}

:host-context(.light) .hljs-addition { background-color: #beb; }
:host-context(.light) .hljs-deletion { background-color: #fbb; }

:host-context(.light) .hljs-meta { color: #269; }

:host-context(.light) .hljs-section,
:host-context(.light) .hljs-selector-id,
:host-context(.light) .hljs-selector-class,
:host-context(.light) .hljs-selector-pseudo,
:host-context(.light) .hljs-selector-tag {
  font-weight: bold;
  color: #48b;
}

:host-context(.light) .hljs-selector-pseudo { font-style: italic; }
:host-context(.light) .hljs-doctag,
:host-context(.light) .hljs-strong { font-weight: bold; }

:host-context(.light) .hljs-emphasis { font-style: italic; }
`;