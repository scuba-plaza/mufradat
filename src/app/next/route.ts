import { context } from "@/lib/server";
import { nextCardPath } from "@/lib/words";

export const dynamic = "force-dynamic";

/** Goes to the card of the next word that is not approved, or to the words page when none is left. */
export function GET() {
  return new Response(null, { status: 303, headers: { Location: nextCardPath(context().store) } });
}
