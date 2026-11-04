/** True when a key press is meant for the page, not for a field being typed in or a dialog that is open. */
export function keyIsShortcut(e: KeyboardEvent): boolean {
  if (e.ctrlKey || e.metaKey || e.altKey) return false;
  const t = e.target;
  const typing = t instanceof HTMLElement && (t.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName));
  return !typing && !document.querySelector("dialog[open]");
}
