import type { Metadata } from "next";
import localFont from "next/font/local";
import Shortcuts from "@/components/Shortcuts";
import "./globals.css";

export const metadata: Metadata = { title: { default: "Mufradat", template: "%s · Mufradat" } };
export const dynamic = "force-dynamic";

// The Anki export reads the Arabic font from public/ too, so it stays there.
const arabic = localFont({ src: "../../public/ScheherazadeNew-Regular.ttf", variable: "--font-arabic" });
const serif = localFont({ src: "./fonts/SourceSerif4-Regular.otf", variable: "--font-serif" });

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${arabic.variable} ${serif.variable}`}>
      <body className="min-h-screen">
        <header className="mx-auto flex max-w-[44rem] items-center justify-between px-5 py-3">
          <a href="/" className="font-semibold">Mufradat</a>
          <nav className="flex items-center gap-3">
            <a href="/search" className="action">Search</a>
            <Shortcuts />
          </nav>
        </header>
        <main className="mx-auto max-w-[44rem] px-5 pb-24">{children}</main>
      </body>
    </html>
  );
}
