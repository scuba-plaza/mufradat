import { NextResponse } from "next/server";

export function refuseCrossSite(req: Request): Response | null {
  const origin = req.headers.get("origin");
  if (!origin || req.method === "GET") return null;
  let host: string | null = null;
  try {
    host = new URL(origin).host;
  } catch {
    host = null;
  }
  if (host !== null && host === req.headers.get("host")) return null;
  return NextResponse.json({ error: "Cross-site request refused." }, { status: 403 });
}
