import type { CardStatus } from "@/lib/types";

const LABEL: Record<CardStatus, string> = { draft: "in progress", incomplete: "in progress", approved: "approved" };

/** Where a card stands: a half-filled circle while it is being made, a filled one once approved. Shape and word, never colour alone. */
export default function StatusMark({ status }: { status: CardStatus }) {
  const approved = status === "approved";
  return (
    <span className="status" data-status={status} title={LABEL[status]}>
      <svg viewBox="0 0 10 10" width="10" height="10" aria-hidden="true">
        <circle cx="5" cy="5" r="4" fill={approved ? "currentColor" : "none"} stroke="currentColor" strokeWidth="1.5" />
        {!approved && <path d="M5 1a4 4 0 0 0 0 8z" fill="currentColor" />}
      </svg>
      <span className="max-sm:sr-only">{LABEL[status]}</span>
    </span>
  );
}
