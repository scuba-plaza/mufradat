"use client";
import { useRef, useState } from "react";
import { POS_LABELS, type Note } from "@/lib/types";
import { plainExample } from "@/lib/validate";
import ActionBar from "./ActionBar";
import { api, post } from "./api";
import Clip from "./Clip";
import Details from "./Details";
import ExampleText from "./ExampleText";
import FormsDialog from "./FormsDialog";
import InlineText from "./InlineText";

const SAVE_TEXT = { idle: "", saving: "Saving…", saved: "Saved", failed: "Not saved" };

/** One word as its card: the text is edited where it is shown, and the actions stay in the bar below. */
export default function CardScreen({ initial, total }: { initial: Note; total: number }) {
  const [note, setNote] = useState(initial);
  const [saved, setSaved] = useState<keyof typeof SAVE_TEXT>("idle");
  const [problems, setProblems] = useState<string[]>([]);
  const [exampleBusy, setExampleBusy] = useState(false);
  const [exampleProblem, setExampleProblem] = useState("");
  const [formsOpen, setFormsOpen] = useState(false);
  const [leaving, setLeaving] = useState(false);
  const queue = useRef<Promise<void>>(Promise.resolve());

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
        setSaved("saved");
      } catch (e) {
        setProblems([(e as Error).message]);
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

  const clip = (slot: string, name: string, text: string) => (
    <Clip note={note} slot={slot} name={name} text={text} onNote={setNote} />
  );

  return (
    <div className="pb-36">
      <h1 className="sr-only">{note.lemma}</h1>
      <div className="flex items-center justify-between gap-4 px-1 py-2 text-sm text-quiet">
        <span>{note.rank > 0 ? `Word ${note.rank} of ${total}` : ""}</span>
        <span role="status">{SAVE_TEXT[saved]}</span>
      </div>

      <div className="mt-8 text-center">
        <InlineText label="Arabic headword" value={note.lemma} arabic className="text-center text-ar-xl" onSave={(lemma) => save({ lemma })} />
        {clip("word", "the word", note.lemma)}
        <InlineText label="English meaning" placeholder="Add the English meaning" value={note.english} className="mt-2 text-center text-2xl font-medium tracking-title" onSave={(english) => save({ english })} />
        <p className="mt-3 text-sm text-quiet">{POS_LABELS[note.pos]}</p>
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
                  <span className="ar text-ar-sm" lang="ar" dir="rtl">{f.arabic}</span>
                  <Clip readOnly note={note} slot={f.id} name={f.label} text={f.arabic} onNote={setNote} />
                </div>
              ))}
            </div>
          </section>
        ) : (
          <button className="action mt-4" aria-haspopup="dialog" onClick={() => setFormsOpen(true)}>Add forms</button>
        )}
        <InlineText label="Hint" placeholder="Add a hint" value={note.hint} className="mt-6 text-center text-quiet" onSave={(hint) => save({ hint })} />
      </div>

      <div className="mt-12 text-center">
        <ExampleText value={note.example} onSave={(example) => save({ example })} />
        {note.example.trim() !== "" && (
          <>
            <InlineText label="Example in English" placeholder="Add the English translation" value={note.exampleEn} className="mt-2 text-center text-lg" onSave={(exampleEn) => save({ exampleEn })} />
            {clip("example", "the example", plainExample(note.example))}
          </>
        )}
        <button className="action mt-6" disabled={exampleBusy} onClick={generateExample}>
          {exampleBusy ? "Asking Mistral…" : note.example ? "New example" : "Generate example"}
        </button>
        {(exampleProblem || note.exampleError) && <p className="mt-1 text-sm text-danger">{exampleProblem || note.exampleError}</p>}
      </div>

      <Details note={note} save={save} />
      <FormsDialog note={note} open={formsOpen} onClose={() => setFormsOpen(false)} save={save} onNote={setNote} />
      <ActionBar note={note} problems={problems} leaving={leaving} onApprove={approve} />
    </div>
  );
}
