import { NextResponse } from "next/server";

export function refuseCrossSite(req: Request): Response | null {
  const origin = req.headers.get("origin");
  if (!origin || req.method === "GET") return null;
  if (URL.canParse(origin) && new URL(origin).host === req.headers.get("host")) return null;
  return NextResponse.json({ error: "Cross-site request refused." }, { status: 403 });
}
