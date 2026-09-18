import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";

import { AppProviders } from "@/components/app/app-providers";

import "./globals.css";

const hubCallbackScript = `
  (() => {
    try {
      if (typeof sessionStorage === "undefined") return;
      const params = new URLSearchParams(window.location.hash.substring(1));
      const id = params.get("id");
      const status = params.get("status");
      const result = params.get("result");
      if (!id || !status || result == null) return;
      if (sessionStorage.getItem("nimnear.auth.hub.rpcCallback")) {
        const next = window.location.pathname + window.location.search;
        try { history.replaceState(history.state, "", next); }
        catch (e) {
          try { history.replaceState(null, "", next); }
          catch (e2) { window.location.hash = ""; }
        }
        return;
      }
      sessionStorage.setItem("nimnear.auth.hub.rpcCallback", JSON.stringify({
        v: 1,
        id: id,
        status: status,
        result: result,
        pathname: window.location.pathname,
        search: window.location.search
      }));
      const next = window.location.pathname + window.location.search;
      try { history.replaceState(history.state, "", next); }
      catch (e) {
        try { history.replaceState(null, "", next); }
        catch (e2) { window.location.hash = ""; }
      }
    } catch (e) {}
  })();
`;

const themeScript = `
  (() => {
    try {
      const storedTheme = window.localStorage.getItem("nimnear.theme");
      const theme = storedTheme === "light" || storedTheme === "dark"
        ? storedTheme
        : window.matchMedia("(prefers-color-scheme: light)").matches
          ? "light"
          : "dark";
      document.documentElement.dataset.theme = theme;
    } catch {
      document.documentElement.dataset.theme = "dark";
    }
  })();
`;

const publicOrigin = process.env.NEXT_PUBLIC_NIMNEAR_PUBLIC_ORIGIN?.trim();

export const viewport: Viewport = {
  themeColor: "#0f0e10",
};

export const metadata: Metadata = {
  applicationName: "NIMNear",
  title: {
    default: "NIMNear",
    template: "%s · NIMNear",
  },
  description: "Discover nearby places and events, and pay with NIM.",
  ...(publicOrigin ? { metadataBase: new URL(publicOrigin) } : {}),
  appleWebApp: {
    capable: true,
    title: "NIMNear",
    statusBarStyle: "black-translucent",
  },
  openGraph: {
    title: "NIMNear",
    description: "Discover nearby places and events, and pay with NIM.",
    type: "website",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: hubCallbackScript }} />
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body className="overflow-x-clip">
        <a
          href="#main"
          className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-[100] focus:rounded-lg focus:bg-primary focus:px-3 focus:py-2 focus:text-sm focus:font-medium focus:text-white"
        >
          Skip to content
        </a>
        <AppProviders>
          <div id="main">{children}</div>
        </AppProviders>
      </body>
    </html>
  );
}
