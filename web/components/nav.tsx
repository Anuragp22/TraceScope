"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { BarChart3, GitBranch, Network, Flame, Search } from "lucide-react";

const navItems = [
  { href: "/", label: "Dashboard", icon: BarChart3 },
  { href: "/explore", label: "Explore", icon: Network },
  { href: "/hotspots", label: "Hotspots", icon: Flame },
  { href: "/analyze", label: "Analyze", icon: GitBranch },
];

export function Nav() {
  const pathname = usePathname();

  return (
    <nav className="w-56 border-r border-border bg-card flex flex-col h-screen shrink-0">
      <div className="p-4 border-b border-border">
        <Link href="/" className="flex items-center gap-2">
          <Search className="h-5 w-5 text-primary" />
          <span className="font-semibold text-lg">TraceScope</span>
        </Link>
      </div>
      <div className="flex-1 p-2 space-y-1">
        {navItems.map((item) => {
          const isActive = pathname === item.href;
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors ${
                isActive
                  ? "bg-accent text-accent-foreground font-medium"
                  : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
              }`}
            >
              <item.icon className="h-4 w-4" />
              {item.label}
            </Link>
          );
        })}
      </div>
      <div className="p-4 border-t border-border text-xs text-muted-foreground">
        TraceScope v1.0
      </div>
    </nav>
  );
}
