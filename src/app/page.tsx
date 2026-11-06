import Backups from "@/components/Backups";
import ExportButton from "@/components/ExportButton";
import Pager from "@/components/Pager";
import StatusMark from "@/components/StatusMark";
import UnusedAudio from "@/components/UnusedAudio";
import WordBar from "@/components/WordBar";
import { rankingSize } from "@/lib/lexicon";
import { firstParam, listPath, paginate, requestedPage } from "@/lib/paging";
import type { SortKey } from "@/lib/query/fields";
import { nextSort, withSort } from "@/lib/query/parse";
import { context } from "@/lib/server";
import type { WordStatus } from "@/lib/types";
import { continueFrom, rowHref, view, wordRows } from "@/lib/words";

const PAGE_SIZE = 100;

const TABS = [
  { label: "All", query: "", status: null },
  { label: "Not started", query: "status:todo", status: "todo" },
  { label: "In progress", query: "status:progress sort:-edited", status: "progress" },
  { label: "Ready", query: "status:ready sort:-edited", status: "ready" },
  { label: "Approved", query: "status:approved", status: "approved" },
] as const;

const COLUMNS: { key: SortKey; label: string; name: string; className: string }[] = [
  { key: "rank", label: "#", name: "Rank (#)", className: "justify-self-end" },
  { key: "arabic", label: "Arabic", name: "Arabic", className: "justify-self-end" },
  { key: "english", label: "English", name: "English", className: "" },
];

export default async function Home({ searchParams }: { searchParams: Promise<{ q?: string | string[]; page?: string | string[] }> }) {
  const params = await searchParams;
  const q = firstParam(params.q).trim();
  const { store } = context();
  const everything = wordRows(store);
  const next = continueFrom(everything);
  const { rows, error, sort } = view(store, q);
  const count = (status: WordStatus) => everything.filter((w) => w.status === status).length;
  const listed = rankingSize();
  const approvedListed = everything.filter((w) => w.status === "approved" && w.rank !== null).length;
  const paging = paginate(rows.length, requestedPage(params), PAGE_SIZE);
  return (
    <>
      <h1 className="sr-only">Words</h1>
      <section aria-label="Next word" className="panel mt-6 px-6 py-10 text-center">
        {next ? (
          <>
            <p className="ar text-ar-md" lang="ar" dir="rtl">{next.lemma}</p>
            <p className="text-lg">{next.english}</p>
            <a className="action-main mt-6" href="/next">Continue</a>
          </>
        ) : (
          <p className="text-lg">Every word is approved.</p>
        )}
        <p className="mt-8 text-sm text-quiet">{count("approved")} approved · {count("progress")} in progress · {count("ready")} ready</p>
        <div role="progressbar" aria-label="Approved among the most frequent words" aria-valuemin={0} aria-valuemax={listed} aria-valuenow={approvedListed} className="mx-auto mt-2 h-1 w-48 overflow-hidden rounded-full bg-rule-strong/30">
          <div className="h-full rounded-full bg-progress" style={{ width: `${(approvedListed / listed) * 100}%` }} />
        </div>
      </section>
      <WordBar q={q} error={error} />
      <nav aria-label="Status" className="mt-2 flex flex-wrap gap-1">
        {TABS.map((tab) => (
          <a
            key={tab.label}
            href={listPath(tab.query)}
            aria-current={q === tab.query ? "page" : undefined}
            className="action-quiet aria-[current=page]:bg-hover aria-[current=page]:text-ink"
          >
            {tab.label} <span className="num">{tab.status ? count(tab.status) : everything.length}</span>
          </a>
        ))}
      </nav>
      {rows.length > 0 && <Pager paging={paging} q={q} total={rows.length} />}
      {rows.length === 0 && !error && <p className="px-2 py-4">Nothing matches.</p>}
      {rows.length > 0 && (
        <div className="table-box">
          <div role="group" aria-label="Sort by" className="table-head grid-cols-[2.5rem_5rem_1fr_auto] sm:grid-cols-[2.5rem_6rem_1fr_7rem]">
            {COLUMNS.map((column) => (
              <a
                key={column.key}
                href={listPath(withSort(q, nextSort(sort, column.key)))}
                aria-label={sort?.key === column.key ? `${column.name}, sorted ${sort.descending ? "descending" : "ascending"}` : column.name}
                aria-current={sort?.key === column.key ? "true" : undefined}
                className={`${column.className} hover:text-ink`}
              >
                {column.label}
                {sort?.key === column.key && <span aria-hidden="true"> {sort.descending ? "↓" : "↑"}</span>}
              </a>
            ))}
            <span className="justify-self-end">Status</span>
          </div>
          {/* Plain anchors: opening a word creates its card, which a prefetching <Link> would do for every row on screen. */}
          <ol aria-label="Word list">
            {rows.slice(paging.start, paging.end).map((w) => (
              <li key={rowHref(w)}>
                <a href={rowHref(w, q)} className="entry grid-cols-[2.5rem_5rem_1fr_auto] sm:grid-cols-[2.5rem_6rem_1fr_7rem]">
                  <span className="num text-right text-xs text-quiet">{w.rank ?? "—"}</span>
                  <span className="ar justify-self-end text-ar-sm" lang="ar" dir="rtl">{w.lemma}</span>
                  <span className="truncate">{w.english}</span>
                  <span className="justify-self-end">{w.status !== "todo" && <StatusMark status={w.status} />}</span>
                </a>
              </li>
            ))}
          </ol>
        </div>
      )}
      {rows.length > 0 && <Pager paging={paging} q={q} label="More pages" />}
      <div className="flex flex-wrap items-start gap-x-4 gap-y-2 py-6">
        <ExportButton approved={count("approved")} />
        <UnusedAudio />
        <Backups />
      </div>
    </>
  );
}
