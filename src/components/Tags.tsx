"use client";
import { useState, type Ref } from "react";

export default function Tags(props: {
  tags: string[];
  problem: string;
  inputRef: Ref<HTMLInputElement>;
  onChange: (change: (tags: string[]) => string[]) => void;
}) {
  const [text, setText] = useState("");

  function add() {
    const tag = text.trim();
    setText("");
    if (tag !== "") props.onChange((tags) => [...tags, tag]);
  }

  return (
    <div>
      {props.tags.length > 0 && (
        <ul className="flex flex-wrap gap-2">
          {props.tags.map((tag) => (
            <li key={tag} className="inline-flex items-center rounded-control bg-hover pl-3 text-sm">
              {tag}
              <button className="action-quiet min-w-control-sm px-2" aria-label={`Remove tag ${tag}`} onClick={() => props.onChange((tags) => tags.filter((t) => t !== tag))}>
                <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true" focusable="false" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M18 6L6 18M6 6l12 12" />
                </svg>
              </button>
            </li>
          ))}
        </ul>
      )}
      <input
        ref={props.inputRef}
        aria-label="Add tag"
        value={text}
        onChange={(e) => setText(e.target.value)}
        onBlur={add}
        onKeyDown={(e) => e.key === "Enter" && !e.nativeEvent.isComposing && add()}
        className="field mt-3"
      />
      <p className="mt-1 text-sm text-quiet">Press Enter to add a tag. A space becomes _.</p>
      {props.problem && <p role="alert" className="mt-1 text-sm text-danger">{props.problem}</p>}
    </div>
  );
}
