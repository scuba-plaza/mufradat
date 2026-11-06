import type { Metadata } from "next";
import { notFound } from "next/navigation";
import CardScreen from "@/components/CardScreen";
import { rankingSize } from "@/lib/lexicon";
import { context } from "@/lib/server";

export async function generateMetadata({ params }: { params: Promise<{ id: string }> }): Promise<Metadata> {
  const note = context().store.getNote((await params).id);
  if (!note) notFound();
  return { title: note.rank > 0 ? `Word ${note.rank}` : note.lemma };
}

export default async function CardPage({ params, searchParams }: { params: Promise<{ id: string }>; searchParams: Promise<{ q?: string }> }) {
  const note = context().store.getNote((await params).id);
  if (!note) notFound();
  const q = ((await searchParams).q ?? "").trim();
  return <CardScreen key={note.id} initial={note} total={rankingSize()} q={q} />;
}
