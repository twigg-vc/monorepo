This directory contains the compiled output (JS + TypeScript definitions) of
https://github.com/micnil/vscode-diff at version 3.0.1 (npm package
`vscode-diff`), an extraction of the diff engine used by VS Code
(https://github.com/microsoft/vscode, `src/vs/editor/common/diff`).

Both the extraction and the original VS Code sources are MIT licensed; see
LICENSE.txt.

The main entry point is `DefaultLinesDiffComputer` (VS Code's current diff
algorithm, with heuristics for better change alignment, character-level inner
changes and moved-code detection).
