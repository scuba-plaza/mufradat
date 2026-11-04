"use client";
import { useEffect, useRef, useState } from "react";
import { plainExample, splitExample } from "@/lib/validate";

/** The example sentence: shown with its target word marked, and edited in the same place as `[target]` markup. */
export default function ExampleText(props: { value: string; onSave: (v: string) => void }) {
  const [draft, setDraft] = useState(props.value);
  useEffect(() => setDraft(props.value), [props.value]);
  const [editing, setEditing] = useState(false);
  const box = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    const el = box.current;
    if (!editing || !el) return;
    el.focus();
    el.setSelectionRange(el.value.length, el.value.length);
  }, [editing]);

  function leave() {
    setEditing(false);
    if (draft !== props.value) props.onSave(draft);
  }

  /** Puts the brackets around the selected text, wherever the old ones were. */
  function mark() {
    const el = box.current;
    if (!el || el.selectionStart === el.selectionEnd) return;
    const plain = plainExample(draft);
    const from = plainExample(draft.slice(0, el.selectionStart)).length;
    const to = from + plainExample(draft.slice(el.selectionStart, el.selectionEnd)).length;
    setDraft(`${plain.slice(0, from)}[${plain.slice(from, to)}]${plain.slice(to)}`);
  }

  if (!editing) {
    const parts = splitExample(draft);
    return (
      <>
        <div role="group" aria-label="Example">
          <button type="button" onClick={() => setEditing(true)} onFocus={() => setEditing(true)} className="block w-full rounded px-1 text-center hover:bg-ink/5">
            {draft === "" ? (
              <span className="text-sm text-quiet">Add an example, or generate one</span>
            ) : (
              <span className="ar text-ar-lg" lang="ar" dir="rtl">
                {parts ? <>{parts.before}<mark>{parts.target}</mark>{parts.after}</> : draft}
              </span>
            )}
          </button>
        </div>
        {draft !== "" && !parts && (
          <p className="mt-1 text-sm text-saffron-ink">It is not clear which word to highlight. Click the sentence, select the word and choose Mark target.</p>
        )}
      </>
    );
  }
  return (
    <div>
      <textarea
        ref={box}
        aria-label="Example"
        rows={2}
        value={draft}
        lang="ar"
        dir="rtl"
        onChange={(e) => setDraft(e.target.value)}
        onBlur={leave}
        onKeyDown={(e) => {
          if (e.key !== "Enter" || e.ctrlKey || e.metaKey || e.nativeEvent.isComposing) return;
          e.preventDefault();
          e.currentTarget.blur();
        }}
        className="edit ar resize-none border-lapis text-center text-ar-lg"
      />
      <p className="mt-1 flex flex-wrap items-center justify-center gap-x-3 text-sm text-quiet">
        <span>Select the word you are teaching, or put it in [square brackets].</span>
        {/* Pressing the button must not take the focus, or the selection it works on is lost. */}
        <button type="button" className="action" onMouseDown={(e) => e.preventDefault()} onClick={mark}>Mark target</button>
      </p>
    </div>
  );
}
