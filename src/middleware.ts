import { NextResponse, type NextRequest } from "next/server";

/** The app only answers pages it served itself: other sites cannot make the browser call it. */
export function middleware(req: NextRequest) {
  const origin = req.headers.get("origin");
  if (origin && req.method !== "GET" && new URL(origin).host !== req.headers.get("host")) {
    return NextResponse.json({ error: "Cross-site request refused." }, { status: 403 });
  }
  return NextResponse.next();
}

export const config = { matcher: "/api/:path*" };
