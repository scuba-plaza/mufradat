import type { ReactNode } from "react";

export default function WordBar({ q, error, children }: { q: string; error: string | null; children?: ReactNode }) {
  return (
    <>
      <div className="mt-4 flex flex-wrap items-center gap-2">
        <form action="/" role="search" className="flex min-w-0 flex-1 items-center gap-2">
          <input
            name="q"
            dir="auto"
            defaultValue={q}
            aria-label="Search words"
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? "query-error" : undefined}
            autoComplete="off"
            spellCheck={false}
            className="field"
            placeholder="Search, or filter: status:progress pos:noun rank:1-500 sort:-edited"
          />
          <button type="submit" className="action min-h-control">Search</button>
        </form>
        {children}
      </div>
      {error && <p id="query-error" role="alert" className="px-1 pt-2 text-sm text-danger">{error}</p>}
    </>
  );
}
