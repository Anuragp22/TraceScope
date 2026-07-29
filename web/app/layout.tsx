import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "@/lib/providers";

export const metadata: Metadata = {
  title: "TraceScope",
  description: "Dependency graph & blast radius analyzer",
};

// The dashboard chrome lives in app/(dashboard)/layout.tsx, so routes with their
// own navigation (the walkthrough) are not nested inside it.
export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark">
      <body className="font-sans antialiased">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
