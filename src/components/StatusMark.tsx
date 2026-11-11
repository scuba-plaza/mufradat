import type { WordStatus } from "@/lib/types";

const LABEL: Record<Exclude<WordStatus, "todo">, string> = { progress: "in progress", ready: "ready", approved: "approved", suspended: "suspended" };

/** Where a card stands, each state in a shape of its own: a half-filled circle while it is being made, a ring with a tick when it lacks nothing, a filled circle once approved. Shape and word, never colour alone. */
export default function StatusMark({ status }: { status: Exclude<WordStatus, "todo"> }) {
  return (
    <span className="status" data-status={status} title={LABEL[status]}>
      <svg viewBox="0 0 10 10" width="12" height="12" aria-hidden="true">
        <circle cx="5" cy="5" r="4" fill={status === "approved" ? "currentColor" : "none"} stroke="currentColor" strokeWidth="1.5" />
        {status === "progress" && <path d="M5 1a4 4 0 0 0 0 8z" fill="currentColor" />}
        {status === "ready" && <path d="M3.2 5.2l1.4 1.4 2.3-2.8" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" />}
        {status === "suspended" && <path d="M3.6 2.5v5M6.4 2.5v5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />}
      </svg>
      <span className="max-sm:sr-only">{LABEL[status]}</span>
    </span>
  );
}
