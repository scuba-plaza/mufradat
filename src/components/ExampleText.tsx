"use client";
import { useEffect, useRef, useState, type FocusEvent } from "react";
import { autoMark, markSelection, splitExample } from "@/lib/validate";

export default function ExampleText(props: { value: string; words: string[]; onSave: (v: string) => void }) {
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

  function leave(e: FocusEvent<HTMLDivElement>) {
    if (e.currentTarget.contains(e.relatedTarget)) return;
    setEditing(false);
    const final = /[[\]]/.test(draft) ? draft : (autoMark(draft, props.words) ?? draft);
    setDraft(final);
    if (final !== props.value) props.onSave(final);
  }

  function markSelected() {
    const el = box.current;
    if (el && el.selectionStart !== el.selectionEnd) setDraft(markSelection(draft, el.selectionStart, el.selectionEnd));
  }

  const unmarked = props.value !== "" && !splitExample(props.value);
  const warning = unmarked && (
    <p className="mt-1 text-sm text-warn">It is not clear which word to highlight. Click the sentence and select the letters you are teaching.</p>
  );

  if (!editing) {
    const parts = splitExample(draft);
    return (
      <>
        <div role="group" aria-label="Example">
          <button type="button" onClick={() => setEditing(true)} onFocus={() => setEditing(true)} className="block min-h-[calc(var(--ar-lg)*1.8+2px)] w-full rounded-control border border-transparent px-2 text-center hover:bg-hover">
            {draft === "" ? (
              <span className="text-sm text-quiet">Add an example, or generate one</span>
            ) : (
              <span className="ar text-ar-lg" lang="ar" dir="rtl">
                {parts ? <>{parts.before}<mark>{parts.target}</mark>{parts.after}</> : draft}
              </span>
            )}
          </button>
        </div>
        {warning}
      </>
    );
  }
  return (
    <>
      <div onBlur={leave}>
        <textarea
          ref={box}
          aria-label="Example"
          rows={1}
          value={draft}
          lang="ar"
          dir="rtl"
          title="Select the letters you are teaching to mark them"
          onChange={(e) => setDraft(e.target.value)}
          onMouseUp={markSelected}
          onKeyUp={markSelected}
          onKeyDown={(e) => {
            if (e.key !== "Enter" || e.nativeEvent.isComposing) return;
            e.preventDefault();
            e.currentTarget.blur();
          }}
          className="edit ar block min-h-[calc(var(--ar-lg)*1.8+2px)] resize-none text-center text-ar-lg [field-sizing:content]"
        />
      </div>
      {warning}
    </>
  );
}
