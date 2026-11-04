import { skipWord } from "@/app/actions";
import { missingParts } from "@/lib/missing";
import type { Note } from "@/lib/types";
import StatusMark from "./StatusMark";

/** The one place for what to do with a card, kept at the bottom of the window: where it stands, and how to go on. */
export default function ActionBar(props: {
  note: Note;
  problems: string[];
  leaving: boolean;
  onApprove: () => void;
}) {
  const { note } = props;
  const approved = note.status === "approved";
  const missing = missingParts(note);
  return (
    <div className="fixed inset-x-0 bottom-0 border-t border-rule bg-paper">
      <div className="mx-auto max-w-[44rem] px-5">
        {props.problems.length > 0 && (
          <ul role="alert" className="pt-2 text-sm text-danger">
            {props.problems.map((p) => <li key={p}>{p}</li>)}
          </ul>
        )}
        <div role="toolbar" aria-label="Card actions" className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 py-3">
          <p className="flex min-w-0 flex-1 items-center gap-2 text-sm text-quiet">
            <StatusMark status={note.status} />
            {!approved && <span>{missing.length > 0 ? `Still needs ${missing.join(", ")}` : "Ready to approve"}</span>}
          </p>
          {approved ? (
            <a href="/next" className="action-main">Next</a>
          ) : (
            <div className="flex items-center gap-3">
              {note.rank > 0 && (
                <form action={skipWord}>
                  <input type="hidden" name="rank" value={note.rank} />
                  <button className="action">Skip</button>
                </form>
              )}
              <button className="action-main" disabled={props.leaving} onClick={props.onApprove}>Approve and next</button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
