export const KEYS = [
  { id: "previous", code: "ArrowLeft", label: "←", does: "Previous word" },
  { id: "next", code: "ArrowRight", label: "→", does: "Next word" },
  { id: "playWord", code: "ArrowUp", label: "↑", does: "Play the word" },
  { id: "playExample", code: "ArrowDown", label: "↓", does: "Play the example" },
] as const;

export type KeyId = (typeof KEYS)[number]["id"];

export function keyPressed(e: KeyboardEvent): KeyId | null {
  if (!e.ctrlKey || e.altKey || e.shiftKey || e.metaKey) return null;
  const t = e.target;
  const typing = t instanceof HTMLElement && (t.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName));
  if (typing || document.querySelector("dialog[open]")) return null;
  return KEYS.find((key) => key.code === e.code)?.id ?? null;
}
