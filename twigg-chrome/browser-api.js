// Firefox's chrome namespace uses callbacks. Prefer its promise-based browser
// namespace so the same async code also works with Chrome's Manifest V3 APIs.
export function getBrowserApi() {
  if (globalThis.browser !== undefined) {
    return globalThis.browser;
  } else {
    return globalThis.chrome;
  }
}
