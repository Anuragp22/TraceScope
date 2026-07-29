"use client";

import * as React from "react";
import { motion, useReducedMotion } from "motion/react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

/* ── motion helpers ───────────────────────────────────────────────── */

export const EASE = [0.22, 0.61, 0.36, 1] as const;

/** Respect the OS reduced-motion setting everywhere, not just on big moves. */
export function useT() {
  const reduce = useReducedMotion();
  return React.useCallback(
    (d = 0.42, delay = 0) => (reduce ? { duration: 0 } : { duration: d, delay, ease: EASE }),
    [reduce],
  );
}

export const listV = { hidden: {}, show: { transition: { staggerChildren: 0.06 } } };
export const itemV = {
  hidden: { opacity: 0, y: 7 },
  show: { opacity: 1, y: 0, transition: { duration: 0.3, ease: EASE } },
};

/* ── text atoms ───────────────────────────────────────────────────── */

/** The caption above a figure: what you are looking at. */
export function Lbl({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <p
      className={cn(
        "mb-3 font-mono text-[10.5px] uppercase tracking-[0.14em] text-muted-foreground",
        className,
      )}
    >
      {children}
    </p>
  );
}

/** The note under a figure: what the numbers mean at real scale. */
export function Scale({ children }: { children: React.ReactNode }) {
  return (
    <p className="mt-4 border-t pt-3 text-[12.5px] leading-relaxed text-muted-foreground [&_b]:font-semibold [&_b]:text-foreground">
      {children}
    </p>
  );
}

/**
 * An aside that earns its own box: the one thing worth noticing about a figure.
 * The left border carries the meaning, so callers override it via `style` to
 * mark a caution (heuristic) or a real defect (alarm).
 */
export function Callout({
  children,
  className,
  ...rest
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "rounded-md border-l-2 bg-muted/40 px-3.5 py-2.5 text-[12.5px] leading-relaxed",
        "border-l-[var(--ts-seed)] [&_b]:font-semibold [&_b]:text-foreground",
        className,
      )}
      {...rest}
    >
      {children}
    </div>
  );
}

/* ── chips ────────────────────────────────────────────────────────── */

type NodeTone = "plain" | "exact" | "heur" | "seed" | "alarm";

const toneStyle: Record<NodeTone, React.CSSProperties> = {
  plain: {},
  exact: { color: "var(--ts-exact)", borderColor: "var(--ts-exact)", background: "var(--ts-exact-bg)" },
  heur: { color: "var(--ts-heuristic)", borderColor: "var(--ts-heuristic)" },
  seed: { color: "var(--ts-seed)", borderColor: "var(--ts-seed)", background: "var(--ts-seed-bg)" },
  alarm: { color: "var(--ts-alarm)", borderColor: "var(--ts-alarm)" },
};

/** A named thing in a diagram: a function, a field, a file. */
export function Node({
  children,
  tone = "plain",
  className,
  style,
  ...rest
}: React.HTMLAttributes<HTMLSpanElement> & { tone?: NodeTone }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded border px-1.5 py-[3px] font-mono text-[11px] leading-none",
        tone === "plain" && "border-border bg-card text-foreground",
        className,
      )}
      style={{ ...toneStyle[tone], ...style }}
      {...rest}
    >
      {children}
    </span>
  );
}

export const riskTone = (r: string): NodeTone =>
  r === "HIGH" ? "alarm" : r === "MEDIUM" ? "heur" : "exact";

/** Risk tier, rendered the same way everywhere it appears. */
export function RiskBadge({ risk }: { risk: string }) {
  return (
    <Node tone={riskTone(risk)} className="tracking-wide">
      {risk}
    </Node>
  );
}

/* ── blocks ───────────────────────────────────────────────────────── */

/** Terminal output. Pre-formatted, scrolls sideways rather than wrapping. */
export function Code({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <pre
      className={cn(
        "overflow-x-auto rounded-md border bg-muted/50 p-3.5",
        "font-mono text-[11.5px] leading-[1.65] text-foreground",
        "[&_b]:font-semibold [&_b]:text-[var(--ts-seed)]",
        "[&_s]:no-underline [&_s]:text-[var(--ts-alarm)]",
        className,
      )}
    >
      {children}
    </pre>
  );
}

export function Row({
  children,
  className,
  ...rest
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn("flex flex-wrap items-center gap-2", className)} {...rest}>
      {children}
    </div>
  );
}

export function Stack({ children, className }: { children: React.ReactNode; className?: string }) {
  return <div className={cn("flex flex-col gap-2", className)}>{children}</div>;
}

/** Wide figures scroll inside their own box; the page never scrolls sideways. */
export function ScrollX({ children, className }: { children: React.ReactNode; className?: string }) {
  return <div className={cn("w-full overflow-x-auto", className)}>{children}</div>;
}

/** Animated version of Stack, for lists that reveal in sequence. */
export function StaggerStack({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <motion.div
      className={cn("flex flex-col gap-2", className)}
      variants={listV}
      initial="hidden"
      animate="show"
    >
      {children}
    </motion.div>
  );
}

export const StaggerItem = motion.div;

/* ── traceability ─────────────────────────────────────────────────── */

/** The file:line citations under every stage. Nothing on this page is unsourced. */
export function Refs({ items }: { items: string[] }) {
  return (
    <div className="flex flex-wrap gap-1.5">
      {items.map((r) => (
        <Badge key={r} variant="outline" className="font-mono text-[10px] font-normal">
          {r}
        </Badge>
      ))}
    </div>
  );
}

/** seed → caller → caller, with a weaker arrow when the route was guessed. */
export function PathChain({ path, conf }: { path: string[]; conf?: string }) {
  return (
    <span className="inline-flex flex-wrap items-center gap-1.5">
      {path.map((p, i) => (
        <React.Fragment key={p + i}>
          {i > 0 && (
            <span
              className="font-mono text-[11px]"
              style={{ color: conf === "HEURISTIC" ? "var(--ts-heuristic)" : "var(--muted-foreground)" }}
            >
              →
            </span>
          )}
          <Node tone={i === 0 ? "seed" : "plain"} className="text-[10.5px]">
            {p}
          </Node>
        </React.Fragment>
      ))}
    </span>
  );
}
