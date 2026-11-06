import ExportButton from "@/components/ExportButton";
import Pager from "@/components/Pager";
import StatusMark from "@/components/StatusMark";
import { rankingSize } from "@/lib/lexicon";
import { paginate, requestedPage } from "@/lib/paging";
import { context } from "@/lib/server";
import type { WordStatus } from "@/lib/types";
import { continueFrom, defaultOrder, wordRows } from "@/lib/words";

const PAGE_SIZE = 100;

export default async function Home({ searchParams }: { searchParams: Promise<{ page?: string }> }) {
  const query = await searchParams;
  const rows = defaultOrder(wordRows(context().store));
  const next = continueFrom(rows);
  const count = (status: WordStatus) => rows.filter((w) => w.status === status).length;
  const listed = rankingSize();
  const approvedListed = rows.filter((w) => w.status === "approved" && w.rank !== null).length;
  const paging = paginate(rows.length, requestedPage(query), PAGE_SIZE);
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
      <Pager paging={paging} />
      {/* Plain anchors: opening a word creates its card, which a prefetching <Link> would do for every row on screen. */}
      <div className="table-box">
        <div aria-hidden="true" className="table-head grid-cols-[2.5rem_5rem_1fr_auto] sm:grid-cols-[2.5rem_6rem_1fr_7rem]">
          <span className="text-right">#</span>
          <span className="justify-self-end">Arabic</span>
          <span>English</span>
          <span className="justify-self-end">Status</span>
        </div>
        <ol aria-label="Word list">
          {rows.slice(paging.start, paging.end).map((w) => (
            <li key={w.id ?? w.rank}>
              <a href={w.id ? `/cards/${w.id}` : `/open?rank=${w.rank}`} className="entry grid-cols-[2.5rem_5rem_1fr_auto] sm:grid-cols-[2.5rem_6rem_1fr_7rem]">
                <span className="num text-right text-xs text-quiet">{w.rank ?? "—"}</span>
                <span className="ar justify-self-end text-ar-sm" lang="ar" dir="rtl">{w.lemma}</span>
                <span className="truncate">{w.english}</span>
                <span className="justify-self-end">{w.status !== "todo" && <StatusMark status={w.status} />}</span>
              </a>
            </li>
          ))}
        </ol>
      </div>
      <Pager paging={paging} label="More pages" />
      <ExportButton approved={count("approved")} />
    </>
  );
}
