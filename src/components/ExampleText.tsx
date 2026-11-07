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

  const hasMark = (text: string) => /[[\]]/.test(text);

  function leave(e: FocusEvent<HTMLDivElement>) {
    if (e.currentTarget.contains(e.relatedTarget)) return;
    setEditing(false);
    const el = box.current;
    const selected = el && el.selectionStart !== el.selectionEnd ? markSelection(draft, el.selectionStart, el.selectionEnd) : draft;
    const typed = draft !== props.value && !hasMark(selected) && !hasMark(props.value);
    const final = typed ? (autoMark(selected, props.words) ?? selected) : selected;
    setDraft(final);
    if (final !== props.value) props.onSave(final);
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
        <span id="example-hint" className="sr-only">Select the letters you are teaching; they are marked when you leave the field.</span>
        <textarea
          ref={box}
          aria-label="Example"
          rows={1}
          value={draft}
          lang="ar"
          dir="rtl"
          title="Select the letters you are teaching; they are marked when you leave the field"
          aria-describedby="example-hint"
          onChange={(e) => setDraft(e.target.value)}
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
