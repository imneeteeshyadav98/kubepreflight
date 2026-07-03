import "@testing-library/jest-dom/vitest";

// jsdom doesn't implement <dialog> show/close behavior — the finding
// drawer (FindingDialog.tsx) relies on the native dialog element, so tests
// need a minimal polyfill rather than avoiding real dialog interaction.
if (typeof HTMLDialogElement !== "undefined") {
  if (!HTMLDialogElement.prototype.showModal) {
    HTMLDialogElement.prototype.showModal = function (this: HTMLDialogElement) {
      this.setAttribute("open", "");
    };
  }
  if (!HTMLDialogElement.prototype.close) {
    HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
      this.removeAttribute("open");
      this.dispatchEvent(new Event("close"));
    };
  }
}
