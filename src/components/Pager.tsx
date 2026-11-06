import type { Paging } from "@/lib/paging";

/** Links between pages of the list. Plain links: no client code. */
export default function Pager({ paging, label = "Pages" }: { paging: Paging; label?: string }) {
  const href = (page: number) => `/?page=${page}`;
  return (
    <nav aria-label={label} className="flex flex-wrap items-center gap-x-4 gap-y-2 py-3 text-sm text-quiet">
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
