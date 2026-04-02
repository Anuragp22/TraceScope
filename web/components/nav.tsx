"use client";

import Link from "next/link";
import Image from "next/image";
import { usePathname } from "next/navigation";
import { BarChart3, GitBranch, Network, Flame, Github, LogOut } from "lucide-react";
import { signIn, signOut, useSession } from "@/lib/auth-client";

const navItems = [
  { href: "/", label: "Dashboard", icon: BarChart3 },
  { href: "/explore", label: "Explore", icon: Network },
  { href: "/hotspots", label: "Hotspots", icon: Flame },
  { href: "/analyze", label: "Analyze", icon: GitBranch },
];

export function Nav() {
  const pathname = usePathname();
  const { data: session, isPending } = useSession();

  return (
    <nav className="w-56 border-r border-border bg-card flex flex-col h-screen shrink-0">
      <div className="p-4 border-b border-border">
        <Link href="/" className="flex items-center gap-2">
          <div className="h-5 w-5 rounded bg-primary flex items-center justify-center">
            <span className="text-primary-foreground text-xs font-bold">T</span>
          </div>
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
      <div className="p-3 border-t border-border">
        {isPending ? (
          <div className="text-xs text-muted-foreground px-2">Loading...</div>
        ) : session?.user ? (
          <div className="space-y-2">
            <div className="flex items-center gap-2 px-2">
              {session.user.image && (
                <Image
                  src={session.user.image}
                  alt=""
                  width={24}
                  height={24}
                  className="h-6 w-6 rounded-full"
                />
              )}
              <span className="text-sm truncate">{session.user.name}</span>
            </div>
            <button
              onClick={() => signOut()}
              className="flex items-center gap-2 px-2 py-1.5 text-xs text-muted-foreground hover:text-foreground w-full rounded-md hover:bg-accent/50"
            >
              <LogOut className="h-3 w-3" />
              Sign out
            </button>
          </div>
        ) : (
          <button
            onClick={() => signIn.social({ provider: "github" })}
            className="flex items-center justify-center gap-2 w-full px-3 py-2 bg-primary text-primary-foreground rounded-md text-sm font-medium hover:bg-primary/90"
          >
            <Github className="h-4 w-4" />
            Sign in with GitHub
          </button>
        )}
      </div>
    </nav>
  );
}
