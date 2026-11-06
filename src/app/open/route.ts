import { context } from "@/lib/server";
import { cardPath, openCard } from "@/lib/words";

export const dynamic = "force-dynamic";

/** Goes to the card for `?rank=N` or `?lemma=…&pos=…`, creating the bare card on first use, and hands `?q=…` on. */
export function GET(req: Request) {
  const q = new URL(req.url).searchParams;
  const rank = Number(q.get("rank"));
  const id = openCard(context().store, q.has("rank") ? { rank } : { lemma: q.get("lemma") ?? "", pos: q.get("pos") ?? "" });
  if (!id) return new Response("Wiktionary has no such entry.", { status: 404 });
  return new Response(null, { status: 303, headers: { Location: cardPath(id, (q.get("q") ?? "").trim()) } });
}
