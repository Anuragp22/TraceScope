import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "@/lib/providers";
import { Nav } from "@/components/nav";

export const metadata: Metadata = {
  title: "TraceScope",
  description: "Dependency graph & blast radius analyzer",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark">
      <body className="font-sans antialiased">
        <Providers>
          <div className="flex h-screen overflow-hidden">
            <Nav />
            <main className="flex-1 overflow-y-auto">{children}</main>
          </div>
        </Providers>
      </body>
    </html>
  );
}
