import type { Metadata } from "next";
import StatusMark from "@/components/StatusMark";
import { lookup } from "@/lib/lexicon";
import { context } from "@/lib/server";

export async function generateMetadata({ searchParams }: { searchParams: Promise<{ q?: string }> }): Promise<Metadata> {
  return { title: `Search: ${((await searchParams).q ?? "").trim()}` };
}

const row = "entry grid-cols-[6rem_1fr_auto]";

export default async function Search({ searchParams }: { searchParams: Promise<{ q?: string }> }) {
  const q = ((await searchParams).q ?? "").trim();
  const { store } = context();
  const cards = store.search(q);
  const entries = lookup(q);
  return (
    <>
      <form action="/search" role="search" className="py-6">
        <input
          name="q"
          defaultValue={q}
          aria-label="Search"
          autoFocus
          className="w-full border-b border-rule bg-transparent py-2 font-serif text-xl placeholder:text-quiet"
          placeholder="Search your cards and Wiktionary, in Arabic or English"
        />
      </form>
      <div className="space-y-8">
        <section>
          <h2 className="mb-1 px-2 text-sm text-quiet">Your cards ({cards.length})</h2>
          {cards.length === 0 ? (
            <p className="px-2">Nothing matches.</p>
          ) : (
            <div>
              {cards.map((n) => (
                <a key={n.id} href={`/cards/${n.id}`} className={row}>
                  <span className="ar justify-self-end text-ar-sm" lang="ar" dir="rtl">{n.lemma}</span>
                  <span className="truncate font-serif">{n.english}</span>
                  <span className="flex items-center gap-3 text-xs text-quiet">{n.rank > 0 && n.rank}<StatusMark status={n.status} /></span>
                </a>
              ))}
            </div>
          )}
        </section>
        <section>
          <h2 className="mb-1 px-2 text-sm text-quiet">Wiktionary ({entries.length})</h2>
          {entries.length === 0 ? (
            <p className="px-2">Nothing matches.</p>
          ) : (
            <div>
              {entries.map((e) => (
                <a key={e.lemma + e.pos} href={`/open?lemma=${encodeURIComponent(e.lemma)}&pos=${e.pos}`} className={row}>
                  <span className="ar justify-self-end text-ar-sm" lang="ar" dir="rtl">{e.lemma}</span>
                  <span className="truncate font-serif">{e.english}</span>
                  <span className="text-xs text-quiet">{e.pos}{e.rank ? `, rank ${e.rank}` : ""}</span>
                </a>
              ))}
            </div>
          )}
        </section>
      </div>
    </>
  );
}
