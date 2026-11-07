"use client";
import { useEffect, useLayoutEffect, useRef, useState, type FocusEvent } from "react";
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

  const timer = useRef<number | undefined>(undefined);
  const composing = useRef(false);
  const keep = useRef<[number, number] | null>(null);
  const settled = useRef<[number, number] | null>(null);

  useLayoutEffect(() => {
    const el = box.current;
    if (!el || !keep.current) return;
    el.setSelectionRange(...keep.current);
    settled.current = keep.current;
    keep.current = null;
  }, [draft]);

  const hasMark = (text: string) => /[[\]]/.test(text);

  function leave(e: FocusEvent<HTMLDivElement>) {
    if (e.currentTarget.contains(e.relatedTarget)) return;
    window.clearTimeout(timer.current);
    setEditing(false);
    const typed = draft !== props.value && !hasMark(draft) && !hasMark(props.value);
    const final = typed ? (autoMark(draft, props.words) ?? draft) : draft;
    setDraft(final);
    if (final !== props.value) props.onSave(final);
  }

  function markSelected() {
    const el = box.current;
    if (!el || composing.current) return;
    const [from, to] = [el.selectionStart, el.selectionEnd];
    if (from === to || (settled.current?.[0] === from && settled.current[1] === to)) return;
    const next = markSelection(draft, from, to);
    if (next === draft) return;
    const parts = splitExample(next);
    const before = parts ? parts.before.length + 1 : (splitExample(draft)?.before.length ?? 0);
    const length = (parts ?? splitExample(draft))?.target.length ?? 0;
    keep.current = [before, before + length];
    setDraft(next);
  }

  const latest = useRef(markSelected);
  latest.current = markSelected;

  useEffect(() => {
    const el = box.current;
    if (!editing || !el) return;
    const changed = () => {
      window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => latest.current(), 350);
    };
    el.addEventListener("selectionchange", changed);
    return () => el.removeEventListener("selectionchange", changed);
  }, [editing]);

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
        <span id="example-hint" className="sr-only">Select the letters you are teaching to mark them.</span>
        <textarea
          ref={box}
          aria-label="Example"
          rows={1}
          value={draft}
          lang="ar"
          dir="rtl"
          title="Select the letters you are teaching to mark them"
          aria-describedby="example-hint"
          onChange={(e) => setDraft(e.target.value)}
          onPointerDown={() => (settled.current = null)}
          onCompositionStart={() => (composing.current = true)}
          onCompositionEnd={() => (composing.current = false)}
          onKeyDown={(e) => {
            settled.current = null;
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
