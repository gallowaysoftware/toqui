import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getLocale, getMessages } from "next-intl/server";
import "./globals.css";
import { Providers } from "@/components/providers/Providers";
import { ServiceWorkerRegistrar } from "@/components/pwa/ServiceWorkerRegistrar";

const inter = Inter({ subsets: ["latin"] });

import type { Viewport } from "next";

export const metadata: Metadata = {
  title: "Toqui",
  description: "Your AI-powered travel companion",
  manifest: "/manifest.json",
  appleWebApp: {
    capable: true,
    statusBarStyle: "default",
    title: "Toqui",
  },
};

export const viewport: Viewport = {
  themeColor: "#E8654A",
};

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const locale = await getLocale();
  const messages = await getMessages();

  return (
    <html lang={locale} suppressHydrationWarning>
      <head>
        {/* Prevent flash of wrong theme by applying dark class before paint */}
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem("toqui_theme");var d=t==="dark"||(t!=="light"&&window.matchMedia("(prefers-color-scheme:dark)").matches);if(d)document.documentElement.classList.add("dark")}catch(e){}})()`,
          }}
        />
      </head>
      <body
        className={`${inter.className} antialiased bg-[var(--color-surface-secondary)] text-[var(--color-text-primary)]`}
      >
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:bg-[var(--color-surface)] focus:px-4 focus:py-2 focus:rounded-lg focus:z-50 focus:text-[var(--color-text-primary)] focus:shadow-lg focus:ring-2 focus:ring-[var(--color-accent)]"
        >
          Skip to content
        </a>
        <NextIntlClientProvider messages={messages}>
          <Providers>{children}</Providers>
        </NextIntlClientProvider>
        <ServiceWorkerRegistrar />
      </body>
    </html>
  );
}
