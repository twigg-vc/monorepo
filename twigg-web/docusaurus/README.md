# Docusaurus Documentation

This folder contains the **documentation site** for the project, built with
**Docusaurus (classic preset)**.

It is currently used in **docs-only mode**, but the structure already supports
adding a **blog** in the future.

---

## Structure

This follows the standard Docusaurus layout:

```
docusaurus/
├── blog/ # Blog content (Markdown / MDX)
├── docs/ # Documentation content (Markdown / MDX)
├── src/ # Docusaurus UI components and theme customizations
├── static/ # Static assets copied into the final build
├── build/ # Generated output (DO NOT EDIT MANUALLY)
├── docusaurus.config.js # Docusaurus config file (routes, presets, plugins etc) 
├── sidebars.js
├── package.json
└...
```

- `blog/` is where all blogs lives
- `docs/` is where all documentation lives
- `src/` is only needed if we customize the UI
- `build/` is **generated** and should not be manually edited

---

## Running locally

To run the documentation site locally:

```bash
task run-docs-locally
```
This will build the Docusaurus and start the local dev server.


## Integration with the Go server

* Go embeds the generated `./build` using go:embed.
* In `aa_public.go`, the function `AddDocsHandler(mux wrappers.RlMux)` registers
all HTTP endpoints required by Docusaurus.
* Once registered, the Go server serves the documentation files directly
from the embedded `./build` directory.