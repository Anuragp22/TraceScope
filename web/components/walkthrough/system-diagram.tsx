"use client";

import { useState } from "react";
import { motion, useReducedMotion } from "motion/react";
import { SYS, SYS_LEGEND, type SysBox } from "@/lib/walkthrough/data";
import { cn } from "@/lib/utils";

const BOX_H = 30;

/** Where an arrow attaches to a box, by side. */
function anchor(b: SysBox, side: string): [number, number] {
  switch (side) {
    case "t":
      return [b.x + b.w / 2, b.y];
    case "b":
      return [b.x + b.w / 2, b.y + BOX_H];
    case "l":
      return [b.x, b.y + BOX_H / 2];
    default:
      return [b.x + b.w, b.y + BOX_H / 2];
  }
}

const strokeFor = (kind: SysBox["kind"]) =>
  kind === "in"
    ? "var(--muted-foreground)"
    : kind === "cmd"
      ? "var(--ts-seed)"
      : kind === "file"
        ? "var(--ts-exact)"
        : kind === "out"
          ? "var(--ts-heuristic)"
          : "var(--border)";

const legendColour = (kind: string) =>
  kind === "in"
    ? "var(--muted-foreground)"
    : kind === "cmd"
      ? "var(--ts-seed)"
      : kind === "file"
        ? "var(--ts-exact)"
        : "var(--ts-heuristic)";

/**
 * The whole tool on one diagram. Hovering or focusing a box dims the rest, so
 * a reader can isolate one path without losing where it sits.
 */
export function SystemDiagram({ className }: { className?: string }) {
  const [lit, setLit] = useState<string | null>(null);
  const reduce = useReducedMotion();
  const t = (d: number, delay = 0) =>
    reduce ? { duration: 0 } : { duration: d, delay, ease: [0.22, 0.61, 0.36, 1] as const };

  return (
    <div className={cn("w-full", className)}>
      <div className="w-full overflow-x-auto">
        <svg
          viewBox="0 0 470 340"
          className="h-auto w-full min-w-[440px]"
          role="img"
          aria-label="How the parts of TraceScope connect"
        >
          <defs>
            <marker id="sysArrow" markerWidth="7" markerHeight="7" refX="5.6" refY="3" orient="auto">
              <path d="M0,0 L6,3 L0,6 Z" fill="currentColor" className="text-muted-foreground" />
            </marker>
          </defs>

          {SYS.arrows.map(([from, to, fs, ts], i) => {
            const A = SYS.boxes.find((b) => b.id === from)!;
            const B = SYS.boxes.find((b) => b.id === to)!;
            const [x1, y1] = anchor(A, fs);
            const [x2, y2] = anchor(B, ts);
            const mid = y1 + (y2 - y1) / 2;
            const d =
              fs === "r"
                ? `M ${x1} ${y1} L ${x1 + 16} ${y1} L ${x1 + 16} ${y2} L ${x2} ${y2}`
                : `M ${x1} ${y1} L ${x1} ${mid} L ${x2} ${mid} L ${x2} ${y2}`;
            // An arrow stays lit only while one of its own endpoints is lit.
            const dim = lit !== null && lit !== from && lit !== to;
            return (
              <motion.path
                key={from + to}
                d={d}
                fill="none"
                stroke="currentColor"
                className="text-muted-foreground"
                strokeWidth={lit === from || lit === to ? 1.6 : 1}
                markerEnd="url(#sysArrow)"
                initial={{ pathLength: 0, opacity: 0 }}
                animate={{ pathLength: 1, opacity: dim ? 0.15 : 0.55 }}
                transition={t(0.5, 0.2 + i * 0.09)}
              />
            );
          })}

          {SYS.boxes.map((b, i) => (
            <motion.g
              key={b.id}
              initial={{ opacity: 0 }}
              animate={{ opacity: !lit || lit === b.id ? 1 : 0.28 }}
              transition={t(0.4, i * 0.05)}
              tabIndex={0}
              role="button"
              aria-label={`${b.t}${b.s ? `, ${b.s}` : ""}`}
              className="cursor-pointer outline-none focus-visible:opacity-100"
              onMouseEnter={() => setLit(b.id)}
              onMouseLeave={() => setLit(null)}
              onFocus={() => setLit(b.id)}
              onBlur={() => setLit(null)}
            >
              <rect
                x={b.x}
                y={b.y}
                width={b.w}
                height={BOX_H}
                rx={4}
                fill={b.kind === "file" ? "var(--ts-exact-bg)" : "var(--card)"}
                stroke={strokeFor(b.kind)}
                strokeWidth={lit === b.id ? 1.8 : 1}
              />
              <text
                x={b.x + b.w / 2}
                y={b.s ? b.y + 14 : b.y + 19}
                textAnchor="middle"
                fill={b.kind === "file" ? "var(--ts-exact)" : "var(--foreground)"}
                style={{ font: "10.5px var(--font-geist-mono)" }}
              >
                {b.t}
              </text>
              {b.s && (
                <text
                  x={b.x + b.w / 2}
                  y={b.y + 24}
                  textAnchor="middle"
                  fill="var(--muted-foreground)"
                  style={{ font: "8px var(--font-geist-mono)" }}
                >
                  {b.s}
                </text>
              )}
            </motion.g>
          ))}
        </svg>
      </div>

      <div className="mt-6 flex flex-wrap items-center gap-x-5 gap-y-2">
        {SYS_LEGEND.map((x) => (
          <span key={x.kind} className="flex items-center gap-2">
            <span
              aria-hidden
              className="size-2.5 rounded-[3px] border"
              style={{ borderColor: legendColour(x.kind) }}
            />
            <span className="font-mono text-[11px] text-muted-foreground">{x.label}</span>
          </span>
        ))}
      </div>
    </div>
  );
}
