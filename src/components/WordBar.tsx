export default function WordBar({ q }: { q: string }) {
  return (
    <form action="/" role="search" className="mt-4 flex items-center gap-2">
      <input
        name="q"
        defaultValue={q}
        aria-label="Search words"
        autoComplete="off"
        spellCheck={false}
        className="field"
        placeholder="Search, or filter: status:progress pos:noun rank:<500 sort:-edited"
      />
      <button type="submit" className="action min-h-control">Search</button>
    </form>
  );
}
