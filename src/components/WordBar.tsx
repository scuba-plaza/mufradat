export default function WordBar({ q, error }: { q: string; error: string | null }) {
  return (
    <>
      <form action="/" role="search" className="mt-4 flex items-center gap-2">
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
          placeholder="Search, or filter: status:progress pos:noun rank:<500 sort:-edited"
        />
        <button type="submit" className="action min-h-control">Search</button>
      </form>
      {error && <p id="query-error" role="alert" className="px-1 pt-2 text-sm text-danger">{error}</p>}
    </>
  );
}
