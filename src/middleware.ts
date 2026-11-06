import { NextResponse, type NextRequest } from "next/server";
import { refuseCrossSite } from "@/lib/origin";

/** The app only answers pages it served itself: other sites cannot make the browser call it. */
export function middleware(req: NextRequest) {
  return refuseCrossSite(req) ?? NextResponse.next();
}

export const config = { matcher: "/api/((?!backups/import$).*)" };
