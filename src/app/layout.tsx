import type { Metadata } from "next";
import localFont from "next/font/local";
import Shortcuts from "@/components/Shortcuts";
import "./globals.css";

export const metadata: Metadata = { title: { default: "Mufradat", template: "%s · Mufradat" } };
export const dynamic = "force-dynamic";

// The Anki export reads the Arabic font from public/ too, so it stays there.
const arabic = localFont({ src: "../../public/ScheherazadeNew-Regular.ttf", variable: "--font-arabic" });

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={arabic.variable}>
      <body className="min-h-screen">
        <header className="topbar">
          <div className="topbar-inner">
            <a href="/" className="wordmark">Mufradat</a>
            <nav className="flex items-center gap-2">
              <a href="/search" className="action-quiet">Search</a>
              <Shortcuts />
            </nav>
          </div>
        </header>
        <main className="mx-auto max-w-column px-5">{children}</main>
      </body>
    </html>
  );
}
