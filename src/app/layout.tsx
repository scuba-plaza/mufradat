import type { Metadata, Viewport } from "next";
import { headers } from "next/headers";
import localFont from "next/font/local";
import Menu from "@/components/Menu";
import { context } from "@/lib/server";
import "./globals.css";

export async function generateMetadata(): Promise<Metadata> {
  const origin = `http://${(await headers()).get("host") ?? "localhost:3000"}`;
  return {
    metadataBase: URL.canParse(origin) ? new URL(origin) : undefined,
    title: { default: "Mufradat", template: "%s · Mufradat" },
    description: "Build one Modern Standard Arabic Anki deck, one card at a time.",
  };
}
export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#ffffff" },
    { media: "(prefers-color-scheme: dark)", color: "#191919" },
  ],
};

export const dynamic = "force-dynamic";

// The Anki export reads the Arabic font from public/ too, so it stays there.
const arabic = localFont({ src: "../../public/ScheherazadeNew-Regular.ttf", variable: "--font-arabic" });

export default function RootLayout({ children }: { children: React.ReactNode }) {
  const { approved } = context().store.counts();
  return (
    <html lang="en" className={arabic.variable}>
      <body className="min-h-screen">
        <header className="topbar">
          <div className="topbar-inner">
            <a href="/" className="wordmark">Mufradat</a>
            <Menu approved={approved} />
          </div>
        </header>
        <main className="mx-auto max-w-column px-5">{children}</main>
      </body>
    </html>
  );
}
