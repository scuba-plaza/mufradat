import { listPath, type Paging } from "@/lib/paging";

export default function Pager({ paging, q, total, label = "Pages" }: { paging: Paging; q: string; total?: number; label?: string }) {
  const href = (page: number) => listPath(q, page);
  return (
    <nav aria-label={label} className="flex flex-wrap items-center gap-x-4 gap-y-2 py-3 text-sm text-quiet">
      {total !== undefined && <span>{total} {total === 1 ? "word" : "words"}</span>}
      {paging.prev !== null && (
        <>
          <a className="action-quiet" href={href(1)}>First</a>
          <a className="action-quiet" href={href(paging.prev)}>Previous</a>
        </>
      )}
      <span>Page {paging.page} of {paging.pages}</span>
      {paging.next !== null && (
        <>
          <a className="action-quiet" href={href(paging.next)}>Next</a>
          <a className="action-quiet" href={href(paging.pages)}>Last</a>
        </>
      )}
    </nav>
  );
}
