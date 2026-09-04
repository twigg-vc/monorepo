// Adds "ready" to the body when the web-components were loaded.
// Used to prevent flashes of unstyled content (see files/index.css)
document.addEventListener("DOMContentLoaded", () => {
    document.body.classList.add("ready");
});