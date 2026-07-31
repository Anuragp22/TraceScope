"use client";

import * as React from "react";
import { cn } from "@/lib/utils";

/** The one table style the walkthrough uses. Dense, monospaced, no zebra. */
export function Table({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <table className={cn("w-full border-collapse text-[11.5px]", className)}>{children}</table>
  );
}

export function Th({
  children,
  num,
  className,
}: {
  children?: React.ReactNode;
  num?: boolean;
  className?: string;
}) {
  return (
    <th
      className={cn(
        "border-b px-2 py-1.5 font-mono text-[10px] font-normal uppercase tracking-[0.1em] text-muted-foreground",
        num ? "text-right" : "text-left",
        className,
      )}
    >
      {children}
    </th>
  );
}

export function Td({
  children,
  num,
  className,
  style,
}: {
  children?: React.ReactNode;
  num?: boolean;
  className?: string;
  style?: React.CSSProperties;
}) {
  return (
    <td
      className={cn(
        "border-b border-border/50 px-2 py-1.5 font-mono align-top",
        num ? "text-right tabular-nums" : "text-left",
        className,
      )}
      style={style}
    >
      {children}
    </td>
  );
}

/** A proportional bar inside a table cell, sized in pixels by the caller. */
export function Bar({ width, colour }: { width: number; colour: string }) {
  return (
    <span
      className="inline-block h-2 rounded-[1px] align-middle"
      style={{ width: `${Math.max(width, 1)}px`, background: colour }}
    />
  );
}
