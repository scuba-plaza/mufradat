"use client";
import { useEffect, useState } from "react";
import InlineText from "./InlineText";

/**
 * PROTOTYPE (Task 1, UI/UX proposals only — replaced by the chosen layout in the implementation task).
 * Three two-hint editor layouts, picked with ?proposal=a|b|c, so screenshots can compare them.
 * The Arabic hint is a local stand-in until the data layer exists.
 */
export function useProposal() {
  const [proposal, setProposal] = useState("a");
  const [hintAr, setHintAr] = useState("");
  useEffect(() => {
    const q = new URLSearchParams(window.location.search);
    const p = q.get("proposal");
    if (p && ["b", "b1", "b2", "c", "none"].includes(p)) setProposal(p);
    const h = q.get("hintAr");
    if (h) setHintAr(h);
  }, []);
  return { proposal, hintAr };
}

/** Proposal B variants: the headword size and the margins it gives back. */
export const B_VARIANTS: Record<string, { head: string; hint: string; forms: string; english: string; example: string }> = {
  b: { head: "text-ar-xl", hint: "mt-1", forms: "mt-2", english: "mt-1", example: "mt-4" },
  b1: { head: "text-[length:clamp(2.75rem,13vw,4.0625rem)] leading-[1.5]", hint: "mt-1.5", forms: "mt-3", english: "mt-2", example: "mt-5" },
  b2: { head: "text-[length:clamp(2.75rem,13vw,4.0625rem)] leading-[1.5]", hint: "mt-2", forms: "mt-4", english: "mt-3", example: "mt-5" },
};

export function HintArabic({ value }: { value: string }) {
  return <InlineText label="Hint Arabic" placeholder="Add a hint for the Arabic side" value={value} arabic className="text-center text-sm text-quiet" onSave={() => {}} />;
}

export function HintEnglish({ value, onSave, className = "" }: { value: string; onSave: (v: string) => void; className?: string }) {
  return <InlineText label="Hint English" placeholder="Add a hint for the English side" value={value} className={`text-center text-sm text-quiet ${className}`} onSave={onSave} />;
}

/** Layout A: both hints side by side under the headword, each marked with its language. */
export function HintsSideBySide({ hintEn, saveHintEn, hintAr, tagged }: { hintEn: string; saveHintEn: (v: string) => void; hintAr: string; tagged: boolean }) {
  const ar = <HintArabic value={hintAr} />;
  const en = <HintEnglish value={hintEn} onSave={saveHintEn} />;
  if (!tagged) {
    return (
      <div className="mt-2 flex items-center justify-center gap-2">
        <div className="min-w-0 flex-1">{ar}</div>
        <span aria-hidden="true" className="text-quiet">·</span>
        <div className="min-w-0 flex-1">{en}</div>
      </div>
    );
  }
  return (
    <div className="mt-2 grid grid-cols-2 gap-x-4">
      <div className="flex min-w-0 items-center justify-end gap-2">
        <span aria-hidden="true" className="shrink-0 text-xs font-medium text-quiet">AR</span>
        <div className="min-w-0 flex-1 [&_input]:text-right">{ar}</div>
      </div>
      <div className="flex min-w-0 items-center gap-2">
        <span aria-hidden="true" className="shrink-0 text-xs font-medium text-quiet">EN</span>
        <div className="min-w-0 flex-1 [&_input]:text-left">{en}</div>
      </div>
    </div>
  );
}
