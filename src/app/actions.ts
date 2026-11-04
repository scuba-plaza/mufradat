"use server";
import { redirect } from "next/navigation";
import { rankingSize } from "@/lib/lexicon";
import { context } from "@/lib/server";
import { nextCardPath } from "@/lib/words";

/** Puts a word aside until every other word is approved, and goes on to the next one. */
export async function skipWord(form: FormData) {
  const rank = Number(form.get("rank"));
  if (!Number.isInteger(rank) || rank < 1 || rank > rankingSize()) return;
  const { store } = context();
  store.skipRank(rank);
  redirect(nextCardPath(store));
}
