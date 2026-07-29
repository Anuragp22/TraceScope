import { Nav } from "@/components/nav";

/**
 * The dashboard shell. Split out of the root layout so /how-it-works can bring
 * its own sidebar instead of nesting one navigation inside another.
 */
export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-screen overflow-hidden">
      <Nav />
      <main className="flex-1 overflow-y-auto">{children}</main>
    </div>
  );
}
