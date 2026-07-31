"use client";

import * as React from "react";
import { AnimatePresence, motion } from "motion/react";
import { ChevronRight } from "lucide-react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Table, Th, Td } from "./table";
import {
  Callout,
  Code,
  Lbl,
  Node,
  Row,
  Scale,
  ScrollX,
  StaggerItem,
  StaggerStack,
  itemV,
  useT,
} from "../primitives";
import { ARCH, ARCH_ARROWS, ARCH_BOXES, HOTSPOTS, INVARIANTS } from "@/lib/walkthrough/arch";
import { cn } from "@/lib/utils";

/* ── the package dependency diagram ───────────────────────────────── */

const BOX_H = 26;

function ArchDiagram() {
  const t = useT();
  const box = (id: string) => ARCH_BOXES.find((b) => b.id === id)!;
  const path = (a: string, b: string) => {
    const A = box(a);
    const B = box(b);
    const x1 = A.x + A.w / 2;
    const y1 = A.y + BOX_H;
    const x2 = B.x + B.w / 2;
    const y2 = B.y;
    const mid = y1 + (y2 - y1) / 2;
    return `M ${x1} ${y1} L ${x1} ${mid} L ${x2} ${mid} L ${x2} ${y2}`;
  };

  return (
    <ScrollX>
      <svg
        viewBox="0 0 444 330"
        className="h-auto w-full min-w-[420px]"
        role="img"
        aria-label="Package dependency diagram"
      >
        <defs>
          <marker id="archArrow" markerWidth="7" markerHeight="7" refX="5.6" refY="3" orient="auto">
            <path d="M0,0 L6,3 L0,6 Z" fill="currentColor" className="text-muted-foreground" />
          </marker>
        </defs>

        {ARCH_ARROWS.map(([a, b]) => (
          <path
            key={a + b}
            d={path(a, b)}
            fill="none"
            stroke="currentColor"
            className="text-muted-foreground"
            strokeWidth="1"
            markerEnd="url(#archArrow)"
            opacity="0.55"
          />
        ))}

        <text
          x="222"
          y="64"
          textAnchor="middle"
          fill="var(--muted-foreground)"
          style={{ font: "9.5px var(--font-geist-mono)", letterSpacing: "0.1em" }}
        >
          cmd imports every package below it
        </text>

        {ARCH_BOXES.map((b, i) => (
          <motion.g key={b.id} initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={t(0.4, i * 0.04)}>
            <rect
              x={b.x}
              y={b.y}
              width={b.w}
              height={BOX_H}
              rx="3"
              fill={b.id === "analyzer" ? "var(--ts-seed-bg)" : "var(--card)"}
              stroke={
                b.id === "analyzer"
                  ? "var(--ts-seed)"
                  : b.y === 292
                    ? "var(--ts-exact)"
                    : "var(--border)"
              }
              strokeWidth="1"
            />
            <text
              x={b.x + b.w / 2}
              y={b.y + 17}
              textAnchor="middle"
              fill={b.id === "analyzer" ? "var(--ts-seed)" : "var(--foreground)"}
              style={{ font: "11.5px var(--font-geist-mono)" }}
            >
              {b.label}
            </text>
            <text
              x={b.x + b.w / 2}
              y={b.y + 38}
              textAnchor="middle"
              fill="var(--muted-foreground)"
              style={{ font: "8.5px var(--font-geist-mono)" }}
            >
              {b.sub}
            </text>
          </motion.g>
        ))}
      </svg>
    </ScrollX>
  );
}

/* ── the data-flow diagram ────────────────────────────────────────── */

function FlowDiagram() {
  const t = useT();
  const cols = ["parser", "graph", "analyzer", "output"];
  const seams = [
    { from: 0, to: 1, label: "FileResult" },
    { from: 1, to: 2, label: "GraphData" },
    { from: 2, to: 3, label: "AnalysisResult" },
  ];
  const W = 96;
  const GAP = 44;
  const H = 30;
  const x = (i: number) => 8 + i * (W + GAP);

  return (
    <ScrollX>
      <svg
        viewBox={`0 0 ${8 + cols.length * (W + GAP)} 96`}
        className="h-auto w-full min-w-[460px]"
        role="img"
        aria-label="Data flow between packages"
      >
        <defs>
          <marker id="flowArrow" markerWidth="7" markerHeight="7" refX="5.6" refY="3" orient="auto">
            <path d="M0,0 L6,3 L0,6 Z" fill="var(--ts-seed)" />
          </marker>
        </defs>

        {seams.map((s, i) => (
          <React.Fragment key={s.label}>
            <motion.line
              x1={x(s.from) + W}
              y1={10 + H / 2}
              x2={x(s.to) - 6}
              y2={10 + H / 2}
              stroke="var(--ts-seed)"
              strokeWidth="1.2"
              markerEnd="url(#flowArrow)"
              initial={{ pathLength: 0 }}
              animate={{ pathLength: 1 }}
              transition={t(0.6, 0.25 + i * 0.2)}
            />
            <text
              x={x(s.from) + W + GAP / 2 - 3}
              y={10 + H + 18}
              textAnchor="middle"
              fill="var(--ts-seed)"
              style={{ font: "9.5px var(--font-geist-mono)" }}
            >
              {s.label}
            </text>
          </React.Fragment>
        ))}

        {cols.map((c, i) => (
          <React.Fragment key={c}>
            <rect x={x(i)} y={10} width={W} height={H} rx="3" fill="var(--card)" stroke="var(--border)" strokeWidth="1" />
            <text
              x={x(i) + W / 2}
              y={29}
              textAnchor="middle"
              fill="var(--foreground)"
              style={{ font: "11.5px var(--font-geist-mono)" }}
            >
              {c}
            </text>
          </React.Fragment>
        ))}
      </svg>
    </ScrollX>
  );
}

/* ── I · the packages ─────────────────────────────────────────────── */

export function MechPackages() {
  return (
    <div>
      <Lbl>11 packages · arrows point from dependent to dependency</Lbl>
      <ArchDiagram />
      <Callout className="mt-3.5">
        Dependencies point one way only, downward on this diagram. <b>parser</b>, <b>diff</b> and{" "}
        <b>config</b> import nothing internal at all, which is why they are the easiest packages to
        test and the ones with the densest test files.
      </Callout>
      <Scale>
        Counts are code and test files per package, read from the repository. The layering is derived
        from actual production imports, not drawn by hand, and a cycle check over those imports finds{" "}
        <b>none</b>.
      </Scale>
    </div>
  );
}

/* ── I · the seams ────────────────────────────────────────────────── */

export function MechSeams() {
  return (
    <div>
      <Lbl>three plain structs carry everything across a boundary</Lbl>
      <FlowDiagram />
      <StaggerStack className="mt-4 gap-3.5">
        {ARCH.seams.map((s) => (
          <StaggerItem key={s.t} variants={itemV}>
            <Row>
              <Node>{s.from}</Node>
              <span className="font-mono text-[11px] text-[var(--ts-seed)]">→</span>
              <Node tone="seed">{s.t}</Node>
              <span className="font-mono text-[11px] text-[var(--ts-seed)]">→</span>
              <Node>{s.to}</Node>
            </Row>
            <div className="mt-1 pl-0.5 text-[12px] text-muted-foreground">{s.d}</div>
          </StaggerItem>
        ))}
      </StaggerStack>

      <Callout className="mt-4">
        The pattern is the same each time: a <b>plain data struct</b>, no interfaces, no behaviour. A
        producer fills it, consumers read it. That is why a second parser front end could be added
        without the graph builder changing, and why four output surfaces exist without the analyzer
        knowing any of them are there.
      </Callout>
      <Scale>
        The middle one doubles as the on-disk format. <b>GraphData</b> is what <b>graph.json</b>{" "}
        contains. The file format and the in-memory model are the same type, which is why the artifact
        is inspectable and why loading it needs no translation layer.
      </Scale>
    </div>
  );
}

/* ── I · the one cycle ────────────────────────────────────────────── */

export function MechCycle() {
  const [broken, setBroken] = React.useState(false);
  const t = useT();
  React.useEffect(() => {
    const timer = setTimeout(() => setBroken(true), 1400);
    return () => clearTimeout(timer);
  }, []);

  return (
    <div>
      <Lbl>the cycle this codebase would otherwise have</Lbl>
      <Row className="mb-1.5">
        <Node>ownership</Node>
        <span className="font-mono text-[11px] text-[var(--ts-exact)]">imports →</span>
        <Node>analyzer</Node>
      </Row>
      <Row>
        <Node>analyzer</Node>
        <span
          className="font-mono text-[11px]"
          style={{ color: broken ? "var(--muted-foreground)" : "var(--ts-alarm)" }}
        >
          {broken ? "does NOT import" : "wants to import →"}
        </span>
        <motion.span animate={{ opacity: broken ? 0.4 : 1 }} transition={t(0.4)}>
          <Node tone={broken ? "plain" : "heur"} className={broken ? "line-through decoration-1" : ""}>
            ownership
          </Node>
        </motion.span>
      </Row>

      <AnimatePresence>
        {broken && (
          <motion.div
            key="fix"
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={t(0.4)}
            className="mt-4"
          >
            <Code>
              {`type AnalysisResult struct {\n    ...\n    Ownership `}
              <s>interface{"{}"}</s>
              {` \`json:"ownership,omitempty"\`\n}\n\n`}
              <i className="text-muted-foreground">{"// and every consumer pays for it:"}</i>
              {`\nif oi, ok := result.Ownership.(*ownership.OwnershipInfo); ok {`}
            </Code>
          </motion.div>
        )}
      </AnimatePresence>

      <Callout className="mt-4">
        <b>ResolveOwnership</b> takes a slice of the analyzer&apos;s own AffectedFunction, so ownership
        depends on analyzer. For the analyzer to hold the result, it would have to depend on
        ownership. That is a cycle, and Go rejects it at compile time.
      </Callout>
      <Callout className="mt-2.5" style={{ borderLeftColor: "var(--ts-alarm)" }}>
        The way out is an <b>interface{"{}"}</b> field and a cast wherever it is read. It compiles, and
        it costs the one thing every other boundary here gives you: the compiler no longer knows what
        is in that field. Passing plain file paths into ownership instead of the analyzer&apos;s
        structs would have removed the dependency entirely and kept the field typed.
      </Callout>
    </div>
  );
}

/* ── I · the invariants ───────────────────────────────────────────── */

export function MechInvariants() {
  const [open, setOpen] = React.useState(0);
  const t = useT();

  return (
    <div>
      <Lbl>properties the graph and the ranking must hold</Lbl>
      <div className="flex flex-col gap-[3px]">
        {INVARIANTS.map((v, i) => (
          <div key={v.rule}>
            <button
              type="button"
              onClick={() => setOpen(open === i ? -1 : i)}
              aria-expanded={open === i}
              className={cn(
                "flex w-full items-baseline gap-2.5 border-l-2 py-1.5 pl-2.5 text-left font-mono text-[11.5px]",
                "cursor-pointer transition-colors hover:text-foreground",
                open === i
                  ? "border-l-[var(--ts-seed)] text-foreground"
                  : "border-l-transparent text-muted-foreground",
              )}
            >
              <span className="shrink-0 text-[10px] text-muted-foreground">
                {String(i + 1).padStart(2, "0")}
              </span>
              <ChevronRight
                className={cn("size-3 shrink-0 transition-transform", open === i && "rotate-90")}
              />
              <span>{v.rule}</span>
            </button>

            <AnimatePresence initial={false}>
              {open === i && (
                <motion.div
                  key="body"
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: "auto" }}
                  exit={{ opacity: 0, height: 0 }}
                  transition={t(0.26)}
                  className="overflow-hidden"
                >
                  <div className="py-1 pl-[21px] pr-1">
                    <Callout className="mb-1.5" style={{ borderLeftColor: "var(--ts-exact)" }}>
                      <b style={{ color: "var(--ts-exact)" }}>Why it holds.</b> {v.holds}
                    </Callout>
                    <Callout style={{ borderLeftColor: "var(--ts-heuristic)" }}>
                      <b>Failure mode without it.</b> {v.breaks}
                    </Callout>
                    <div className="mt-2 font-mono text-[10px] text-muted-foreground">
                      enforced at {v.where}
                    </div>
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        ))}
      </div>

      <Callout className="mt-3.5">
        Each of these is documented at the line that enforces it, not only here. A rule with its
        reasoning attached survives the next refactor; a bare condition gets simplified away by
        someone who cannot see what it was for.
      </Callout>
      <Scale>
        Select a rule for its rationale and failure mode. All eight are covered by regression tests, so
        a change that violates one fails the build rather than degrading the output quietly.
      </Scale>
    </div>
  );
}

/* ── IV · the two query commands ──────────────────────────────────── */

export function MechQueries() {
  return (
    <div>
      <Tabs defaultValue="why">
        <TabsList className="mb-3.5 h-8">
          <TabsTrigger value="why" className="text-[12px]">why</TabsTrigger>
          <TabsTrigger value="hot" className="text-[12px]">hotspots</TabsTrigger>
        </TabsList>

        <TabsContent value="why">
          <Code>
            {"$ "}
            <b>tracescope why ComputeBlastRadius Analyze --reverse</b>
            {`\n\n  TraceScope, Call Path (1 hop)\n\n  ComputeBlastRadius             internal/graph/query.go:23 [graph]\n    │\n    │ CALLS\n    ▼\n  Analyze                        internal/analyzer/blast_radius.go:95 [analyzer]`}
          </Code>

          <Lbl className="mt-5">how it reads the names you typed</Lbl>
          <Row className="mb-2.5">
            {["exact", "qualified", "prefix", "substring"].map((m, i) => (
              <React.Fragment key={m}>
                {i > 0 && <span className="text-[11px] text-muted-foreground">then</span>}
                <Node tone={i === 0 ? "exact" : "plain"}>{m}</Node>
              </React.Fragment>
            ))}
          </Row>

          <Callout>
            Most specific first. A bare name matching several packages is refused rather than picked at
            random, and the candidates are printed so you can say which one you meant.
          </Callout>
          <Callout className="mt-2.5">
            <b>--reverse</b> asks the opposite question. Not does A call B, but does anything lead from
            B back to A. Same search, adjacency built the other way round.
          </Callout>
        </TabsContent>

        <TabsContent value="hot">
          <ScrollX>
            <Table>
              <thead>
                <tr>
                  <Th>function</Th>
                  <Th num>in</Th>
                  <Th num>out</Th>
                  <Th num>score</Th>
                  <Th>file</Th>
                </tr>
              </thead>
              <tbody>
                {HOTSPOTS.map((h) => (
                  <tr key={h.n}>
                    <Td style={{ color: h.suspect ? "var(--ts-heuristic)" : undefined }}>
                      {h.n}
                      {h.suspect ? " ⚠" : ""}
                    </Td>
                    <Td num>{h.i}</Td>
                    <Td num>{h.o}</Td>
                    <Td num>
                      <b>{h.c}</b>
                    </Td>
                    <Td className="text-muted-foreground">
                      {h.f}:{h.l}
                    </Td>
                  </tr>
                ))}
              </tbody>
            </Table>
          </ScrollX>

          <Callout className="mt-3">
            The score is <b>callers in, doubled, plus calls out</b>. Callers count double because they
            are the blast radius if that function changes. It is a sum and not a product so that
            something called everywhere but calling nothing itself does not score zero.
          </Callout>
          <Callout className="mt-2.5" style={{ borderLeftColor: "var(--ts-alarm)" }}>
            The top row is wrong, and usefully so. <b>Error</b> shows 47 callers, but those are every
            err.Error() in the repository landing on one definition. This ranking counts a guessed call
            exactly like a verified one, so a known mistake sits at the top of it.
          </Callout>
        </TabsContent>
      </Tabs>

      <Scale>
        Neither command needs a diff. Both read the graph that <b>index</b> already wrote, which is why
        they answer instantly.
      </Scale>
    </div>
  );
}
