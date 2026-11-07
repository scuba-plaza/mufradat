"use client";
import { useEffect, useRef, useState } from "react";
import { listPath } from "@/lib/paging";
import type { Note } from "@/lib/types";
import { plainExample } from "@/lib/validate";
import ActionBar from "./ActionBar";
import { api, post } from "./api";
import Clip from "./Clip";
import ExampleText from "./ExampleText";
import FormsDialog from "./FormsDialog";
import InlineText from "./InlineText";
import { keyPressed } from "./keys";
import ResetDialog from "./ResetDialog";
import TagsAndDetailsDialog from "./TagsAndDetailsDialog";

const SAVE_TEXT = { idle: "", saving: "Saving…", saved: "Saved", failed: "Not saved" };

/** One word as its card: the text is edited where it is shown, and the actions stay in the bar below. */
export default function CardScreen({ initial, total, q }: { initial: Note; total: number; q: string }) {
  const [note, setNote] = useState(initial);
  const [saved, setSaved] = useState<keyof typeof SAVE_TEXT>("idle");
  const [problems, setProblems] = useState<string[]>([]);
  const [exampleBusy, setExampleBusy] = useState(false);
  const [exampleProblem, setExampleProblem] = useState("");
  const [formsOpen, setFormsOpen] = useState(false);
  const [resetOpen, setResetOpen] = useState(false);
  const [tagsOpen, setTagsOpen] = useState(false);
  const [tagProblem, setTagProblem] = useState("");
  const [leaving, setLeaving] = useState(false);
  const queue = useRef<Promise<void>>(Promise.resolve());
  const tags = useRef(initial.tags);

  /** Changes happen one at a time, in the order they were made, so that approving never overtakes a save. */
  const later = (work: () => Promise<void>) => {
    queue.current = queue.current.then(work);
  };

  function save(patch: Record<string, unknown>) {
    setSaved("saving");
    later(async () => {
      try {
        setNote((await api<{ note: Note }>(`/api/notes/${note.id}`, {
          method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(patch),
        })).note);
        setProblems([]);
        if ("example" in patch) setExampleProblem("");
        setSaved("saved");
      } catch (e) {
        setProblems([(e as Error).message]);
        setSaved("failed");
      }
    });
  }

  function saveTags(change: (tags: string[]) => string[]) {
    setSaved("saving");
    later(async () => {
      try {
        const next = (await api<{ note: Note }>(`/api/notes/${note.id}/tags`, {
          method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ tags: change(tags.current) }),
        })).note;
        tags.current = next.tags;
        setNote(next);
        setTagProblem("");
        setSaved("saved");
      } catch (e) {
        setTagProblem((e as Error).message);
        setSaved("failed");
      }
    });
  }

  function generateExample() {
    if (exampleBusy) return;
    setExampleBusy(true);
    setExampleProblem("");
    later(async () => {
      try {
        setNote((await post<{ note: Note }>("/api/mistral", { id: note.id })).note);
        setProblems([]);
      } catch (e) {
        setExampleProblem((e as Error).message);
      } finally {
        setExampleBusy(false);
      }
    });
  }

  function approve() {
    setLeaving(true);
    later(async () => {
      try {
        await post(`/api/notes/${note.id}/approve`, {});
        window.location.assign("/next");
      } catch (e) {
        setLeaving(false);
        setProblems((e as Error & { errors?: string[] }).errors ?? [(e as Error).message]);
      }
    });
  }

  function reset() {
    setLeaving(true);
    later(async () => {
      try {
        await api(`/api/notes/${note.id}`, { method: "DELETE" });
        window.location.assign(listPath(q));
      } catch (e) {
        setLeaving(false);
        setResetOpen(false);
        setProblems([(e as Error).message]);
      }
    });
  }

  useEffect(() => {
    const play = (name: string) => document.querySelector<HTMLButtonElement>(`button[aria-label="Play ${name}"]`)?.click();
    const go = (direction: "previous" | "next") =>
      later(async () => {
        try {
          const { href } = await api<{ href: string | null }>(`/api/step?${new URLSearchParams({ id: note.id, dir: direction, q })}`);
          if (href) window.location.assign(href);
        } catch (e) {
          setProblems([(e as Error).message]);
        }
      });
    function onKey(e: KeyboardEvent) {
      const key = keyPressed(e);
      if (!key) return;
      e.preventDefault();
      if (e.repeat) return;
      if (key === "previous" || key === "next") go(key);
      else if (key === "playWord") play("the word");
      else if (key === "playExample") play("the example");
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [note.id, q]);

  const clip = (slot: string, name: string, text: string) => (
    <div className="mt-2">
      <Clip note={note} slot={slot} name={name} text={text} onNote={setNote} />
    </div>
  );

  return (
    <div className="pb-[var(--bar-h)]">
      <h1 className="sr-only">{note.lemma}</h1>
      <div className="flex items-center justify-between gap-4 px-1 py-1 text-sm text-quiet">
        <div className="flex items-center gap-1">
          <span>{note.rank > 0 ? `Word ${note.rank} of ${total}` : ""}</span>
          <button className="action-quiet min-w-control-sm px-2" title="Reset word" aria-haspopup="dialog" onClick={() => setResetOpen(true)}>
            <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M3 12a9 9 0 1 0 3-6.7L3 8" /><path d="M3 3v5h5" />
            </svg>
          </button>
          <button className="action-quiet min-w-control-sm px-2" title="Tags and details" aria-haspopup="dialog" onClick={() => setTagsOpen(true)}>
            <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" focusable="false" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z" /><path d="M7 7h.01" />
            </svg>
          </button>
        </div>
        <span role="status">{SAVE_TEXT[saved]}</span>
      </div>

      <div className="mt-6 text-center">
        <InlineText label="Arabic headword" value={note.lemma} arabic className="text-center text-ar-xl" onSave={(lemma) => save({ lemma })} />
        {clip("word", "the word", note.lemma)}
        <InlineText label="Hint" placeholder="Add a hint" value={note.hint} className="mt-2 text-center text-quiet" onSave={(hint) => save({ hint })} />
        {note.forms.length > 0 ? (
          <section aria-label="Forms" className="panel mx-auto mt-5 max-w-lg px-4 py-3">
            <div className="flex items-center justify-between">
              <h2 className="text-xs font-medium text-quiet">Forms</h2>
              <button className="action-quiet" aria-haspopup="dialog" onClick={() => setFormsOpen(true)}>Edit forms</button>
            </div>
            <div className="mt-2 flex flex-wrap items-start justify-center gap-x-8 gap-y-3">
              {note.forms.map((f) => (
                <div key={f.id} className="flex flex-col items-center">
                  <span className="text-xs text-quiet">{f.label}</span>
                  <span className="ar mt-1 text-ar-stacked" lang="ar" dir="rtl">{f.arabic}</span>
                  <Clip readOnly note={note} slot={f.id} name={f.label} text={f.arabic} onNote={setNote} />
                </div>
              ))}
            </div>
          </section>
        ) : (
          <button className="action mt-4" aria-haspopup="dialog" onClick={() => setFormsOpen(true)}>Add forms</button>
        )}
        <InlineText label="English meaning" placeholder="Add the English meaning" value={note.english} className="mt-5 text-center text-2xl font-medium tracking-title" onSave={(english) => save({ english })} />
      </div>

      <div className="mt-10 text-center">
        <ExampleText value={note.example} onSave={(example) => save({ example })} />
        {note.example.trim() !== "" && (
          <>
            <InlineText label="Example in English" placeholder="Add the English translation" value={note.exampleEn} className="mt-2 text-center text-lg" onSave={(exampleEn) => save({ exampleEn })} />
            {clip("example", "the example", plainExample(note.example))}
          </>
        )}
        <button className="action mt-5" disabled={exampleBusy} onClick={generateExample}>
          {exampleBusy ? "Asking Mistral…" : note.example ? "New example" : "Generate example"}
        </button>
        {(exampleProblem || note.exampleError) && <p className="mt-1 text-sm text-danger">{exampleProblem || note.exampleError}</p>}
      </div>

      <TagsAndDetailsDialog note={note} open={tagsOpen} onClose={() => setTagsOpen(false)} save={save} saveTags={saveTags} tagProblem={tagProblem} />
      <FormsDialog note={note} open={formsOpen} onClose={() => setFormsOpen(false)} save={save} onNote={setNote} />
      <ResetDialog note={note} open={resetOpen} busy={leaving} onClose={() => setResetOpen(false)} onReset={reset} />
      <ActionBar note={note} problems={problems} leaving={leaving} onApprove={approve} />
    </div>
  );
}
