import ExportButton from "@/components/ExportButton";
import Pager from "@/components/Pager";
import StatusMark from "@/components/StatusMark";
import { paginate, requestedPage } from "@/lib/paging";
import { context } from "@/lib/server";
import { frequencyList, nextWord } from "@/lib/words";

const PAGE_SIZE = 100;

export default async function Home({ searchParams }: { searchParams: Promise<{ page?: string; rank?: string }> }) {
  const query = await searchParams;
  const { store } = context();
  const next = nextWord(store);
  const words = frequencyList(store);
  const approved = words.filter((w) => w.status === "approved").length;
  const paging = paginate(words.length, requestedPage(query, next?.rank ?? null, PAGE_SIZE), PAGE_SIZE);
  return (
    <>
      <section aria-label="Next word" className="py-8 text-center">
        {next ? (
          <>
            <p className="ar text-ar-md" lang="ar" dir="rtl">{next.lemma}</p>
            <p className="font-serif text-lg">{next.english}</p>
            <a className="action-main mt-4" href="/next">Continue</a>
          </>
        ) : (
          <p className="font-serif text-lg">Every word is approved.</p>
        )}
        <p className="mt-6 text-sm text-quiet">{approved} of {words.length} approved</p>
        <div role="progressbar" aria-label="Approved words" aria-valuemin={0} aria-valuemax={words.length} aria-valuenow={approved} className="mx-auto mt-2 h-0.5 w-48 bg-rule">
          <div className="h-full bg-lapis" style={{ width: `${(approved / words.length) * 100}%` }} />
        </div>
      </section>
      <Pager paging={paging} total={words.length} />
      {/* Plain anchors: opening a word creates its card, which a prefetching <Link> would do for every row on screen. */}
      <ol aria-label="Frequency list">
        {words.slice(paging.start, paging.end).map((w) => (
          <li key={w.rank} aria-current={Number(query.rank) === w.rank ? "location" : undefined} className="aria-[current=location]:bg-lapis/10">
            <a href={`/open?rank=${w.rank}`} className="entry grid-cols-[2.5rem_5rem_1fr_auto] sm:grid-cols-[2.5rem_6rem_1fr_7rem]">
              <span className="text-right text-xs tabular-nums text-quiet">{w.rank}</span>
              <span className="ar justify-self-end text-ar-sm" lang="ar" dir="rtl">{w.lemma}</span>
              <span className="truncate font-serif">{w.english}</span>
              <span className="justify-self-end">{w.status && <StatusMark status={w.status} />}</span>
            </a>
          </li>
        ))}
      </ol>
      <Pager paging={paging} total={words.length} />
      <ExportButton approved={approved} />
    </>
  );
}
