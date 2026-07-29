"use client";

import * as React from "react";
import { AnimatePresence, motion } from "motion/react";
import { ArrowLeft, ArrowRight, BookOpen, Github } from "lucide-react";

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
} from "@/components/ui/sidebar";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";

import { ACTS, FLAT, type Stage } from "./stages";
import { SystemDiagram } from "./system-diagram";
import { MECHANISMS } from "./mechanisms";
import { Refs, useT } from "./primitives";
import { G, GLOSSARY, SCENARIOS } from "@/lib/walkthrough/data";

/* ── glossary ─────────────────────────────────────────────────────── */

function Glossary() {
  return (
    <Sheet>
      <SheetTrigger asChild>
        <Button variant="ghost" size="sm" className="w-full justify-start gap-2">
          <BookOpen className="size-4" />
          <span>Glossary</span>
        </Button>
      </SheetTrigger>
      <SheetContent side="right" className="w-full sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>Every term this page uses</SheetTitle>
        </SheetHeader>
        <ScrollArea className="h-[calc(100vh-6rem)] px-4 pb-8">
          <dl className="space-y-5">
            {GLOSSARY.map(([term, def]) => (
              <div key={term}>
                <dt className="font-mono text-[12px] text-[var(--ts-seed)]">{term}</dt>
                <dd className="mt-1 text-[13px] leading-relaxed text-muted-foreground">{def}</dd>
              </div>
            ))}
          </dl>
        </ScrollArea>
      </SheetContent>
    </Sheet>
  );
}

/* ── the landing page ─────────────────────────────────────────────── */

/**
 * Page one is the diagram and nothing else. Anyone landing cold needs the shape
 * of the system before any prose about it, so the reasoning for this stage sits
 * below the fold rather than beside the figure.
 */
function Landing({ onStart }: { onStart: () => void }) {
  const t = useT();
  return (
    <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={t(0.4)}>
      <div className="mx-auto flex min-h-[calc(100svh-3.5rem)] max-w-4xl flex-col justify-center px-5 py-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.16em] text-muted-foreground">
          TraceScope · how it works
        </p>
        <h1 className="mt-3 text-balance text-3xl font-semibold tracking-tight sm:text-4xl">
          Every part, on one diagram
        </h1>
        <p className="mt-3 max-w-2xl text-balance text-[15px] leading-relaxed text-muted-foreground">
          Nine boxes. Every other page in this walkthrough is a detail inside one of them.
        </p>

        <div className="mt-10">
          <SystemDiagram />
        </div>

        <div className="mt-10 flex flex-wrap items-center gap-3">
          <Button onClick={onStart} className="gap-2">
            Start the walkthrough
            <ArrowRight className="size-4" />
          </Button>
          <span className="font-mono text-[11px] text-muted-foreground">
            {FLAT.length} stages · every figure measured at {G.commit}
          </span>
        </div>
      </div>

      {/* Below the fold: the reasoning for the diagram above. Kept off the first
          screen so the figure lands on its own, but not dropped. */}
      <div className="mx-auto max-w-4xl px-5 pb-14">
        <Separator className="mb-10" />
        <div className="grid gap-8 sm:grid-cols-2">
          <div>
            <h3 className="font-mono text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
              why it is built this way
            </h3>
            <div className="mt-3 space-y-3 text-[13.5px] leading-relaxed text-muted-foreground [&_strong]:font-semibold [&_strong]:text-foreground">
              {FLAT[0].stage.why}
            </div>
          </div>
          <div className="space-y-4">
            <div
              className="rounded-md border-l-2 bg-muted/40 px-3.5 py-3"
              style={{ borderLeftColor: "var(--ts-heuristic)" }}
            >
              <p className="font-mono text-[10.5px] uppercase tracking-[0.14em] text-[var(--ts-heuristic)]">
                the honest limitation
              </p>
              <p className="mt-1.5 text-[13px] leading-relaxed text-muted-foreground">
                {FLAT[0].stage.limit}
              </p>
            </div>
            <Refs items={FLAT[0].stage.refs} />
          </div>
        </div>
      </div>
    </motion.div>
  );
}

/* ── a stage ──────────────────────────────────────────────────────── */

function StageView({
  stage,
  index,
  source,
  scenarioId,
}: {
  stage: Stage;
  index: number;
  source: string;
  scenarioId: string;
}) {
  const Mech = MECHANISMS[stage.mech];
  const sc = SCENARIOS[scenarioId];

  return (
    <div className="mx-auto max-w-6xl px-5 py-8">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
        stage {String(index + 1).padStart(2, "0")} · {stage.t}
      </p>
      <h2 className="mt-2 text-balance text-2xl font-semibold tracking-tight">{stage.h}</h2>
      <p className="mt-2 max-w-3xl text-balance text-[14.5px] leading-relaxed text-muted-foreground">
        {stage.k}
      </p>

      <div className="mt-8 grid gap-6 lg:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)]">
        <Card>
          <CardContent className="pt-6">
            {Mech ? (
              <Mech sc={sc} source={source} />
            ) : (
              <p className="font-mono text-[12px] text-muted-foreground">
                figure unavailable: {stage.mech}
              </p>
            )}
          </CardContent>
        </Card>

        <aside className="space-y-5">
          <div>
            <h3 className="font-mono text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
              why it is built this way
            </h3>
            <div className="mt-3 space-y-3 text-[13.5px] leading-relaxed text-muted-foreground [&_strong]:font-semibold [&_strong]:text-foreground">
              {stage.why}
            </div>
          </div>

          <div
            className="rounded-md border-l-2 bg-muted/40 px-3.5 py-3"
            style={{ borderLeftColor: "var(--ts-heuristic)" }}
          >
            <p className="font-mono text-[10.5px] uppercase tracking-[0.14em] text-[var(--ts-heuristic)]">
              the honest limitation
            </p>
            <p className="mt-1.5 text-[13px] leading-relaxed text-muted-foreground">{stage.limit}</p>
          </div>

          <Refs items={stage.refs} />
        </aside>
      </div>
    </div>
  );
}

/* ── the page-level provenance note ───────────────────────────────── */

/**
 * Rendered once, at the end of the walkthrough. This is a statement about the
 * whole page, so repeating it under every stage was noise.
 */
function Provenance() {
  return (
    <div className="mx-auto max-w-3xl space-y-4 px-5 pb-16 text-[12.5px] leading-relaxed text-muted-foreground">
      <Separator className="mb-8" />
      <p>
        <b className="font-semibold text-foreground">Provenance.</b> Graph counts come from{" "}
        <code className="font-mono text-[11.5px]">.tracescope/graph.json</code>, built by the parser
        backend at commit {G.commit}. Scenario results were produced by replaying the logic in{" "}
        <code className="font-mono text-[11.5px]">graph/query.go</code>,{" "}
        <code className="font-mono text-[11.5px]">analyzer/risk_scorer.go</code> and{" "}
        <code className="font-mono text-[11.5px]">analyzer/blast_radius.go</code> against that
        artifact. Evaluation figures come from{" "}
        <code className="font-mono text-[11.5px]">docs/EVALUATION.md</code> (gin-gonic/gin, n=300).
      </p>
      <p>
        <b className="font-semibold text-foreground">What this page does not claim.</b> No SCIP-built
        graph of this repository exists on disk, so no node or edge counts are quoted for the SCIP
        path, because inventing them would be a made-up number. Timings come from one machine and are
        indicative, not a benchmark.
      </p>
      <p>
        <b className="font-semibold text-foreground">On stated limitations.</b> Every stage carries
        one. They are scope boundaries, not disclaimers: the tool measures impact, and the evaluation
        in Act IV establishes that impact alone is a weak predictor of defects. Documenting where a
        measurement stops being valid is what makes the rest of it usable.
      </p>
    </div>
  );
}

/* ── shell ────────────────────────────────────────────────────────── */

export function Walkthrough() {
  // One flat index over every stage, so the sidebar, the prev/next buttons and
  // the keyboard shortcuts can never disagree about where the reader is.
  const [pos, setPos] = React.useState(0);
  const [source, setSource] = React.useState("parser");
  const [scenarioId, setScenarioId] = React.useState("leaf");

  const current = FLAT[pos];
  const { act, stage, si } = current;
  const onLanding = pos === 0;

  const showSource = act.id === "index";
  const showScenario = act.id === "analyse";

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = document.activeElement;
      if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement) return;
      if (e.key === "ArrowRight") setPos((p) => Math.min(FLAT.length - 1, p + 1));
      if (e.key === "ArrowLeft") setPos((p) => Math.max(0, p - 1));
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  // Scrolling is per-pane, so a new stage has to reset the pane, not the window.
  const paneRef = React.useRef<HTMLDivElement>(null);
  React.useEffect(() => {
    paneRef.current?.scrollTo({ top: 0 });
  }, [pos]);

  return (
    <SidebarProvider>
      <Sidebar collapsible="offcanvas">
        <SidebarHeader className="px-3 py-3">
          <div className="font-mono text-[13px] font-semibold tracking-tight">
            tracescope
            <span className="text-muted-foreground"> / how it works</span>
          </div>
        </SidebarHeader>

        <SidebarContent>
          {ACTS.map((a) => (
            <SidebarGroup key={a.id}>
              <SidebarGroupLabel className="font-mono text-[10px] uppercase tracking-[0.14em]">
                {a.label}
              </SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  {a.stages.map((s) => {
                    const flatIdx = FLAT.findIndex((f) => f.act.id === a.id && f.stage.id === s.id);
                    const active = flatIdx === pos;
                    return (
                      <SidebarMenuItem key={a.id + s.id}>
                        <SidebarMenuButton
                          isActive={active}
                          onClick={() => setPos(flatIdx)}
                          className="gap-2.5"
                        >
                          <span className="font-mono text-[10px] tabular-nums text-muted-foreground">
                            {String(flatIdx + 1).padStart(2, "0")}
                          </span>
                          <span className="truncate text-[13px]">{s.t}</span>
                        </SidebarMenuButton>
                      </SidebarMenuItem>
                    );
                  })}
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          ))}
        </SidebarContent>

        <SidebarFooter className="gap-1 p-2">
          <Glossary />
          <Button variant="ghost" size="sm" className="w-full justify-start gap-2" asChild>
            <a href={`https://${G.remote}`} target="_blank" rel="noreferrer">
              <Github className="size-4" />
              <span>Repository</span>
            </a>
          </Button>
        </SidebarFooter>

        <SidebarRail />
      </Sidebar>

      <SidebarInset className="flex h-svh min-w-0 flex-col overflow-hidden">
        <header className="flex h-14 shrink-0 items-center gap-3 border-b px-4">
          <SidebarTrigger className="-ml-1" />
          <Separator orientation="vertical" className="h-5" />
          <div className="flex min-w-0 items-center gap-2">
            <span className="truncate font-mono text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
              {act.label}
            </span>
            {!onLanding && (
              <>
                <span className="text-muted-foreground">·</span>
                <span className="truncate text-[13px]">{stage.t}</span>
              </>
            )}
          </div>

          <div className="ml-auto flex items-center gap-3">
            <span className="hidden font-mono text-[11px] tabular-nums text-muted-foreground sm:inline">
              {String(pos + 1).padStart(2, "0")} / {FLAT.length}
            </span>
            <div className="hidden items-center gap-1 md:flex">
              <Button
                variant="ghost"
                size="icon"
                aria-label="Previous stage"
                disabled={pos === 0}
                onClick={() => setPos((p) => Math.max(0, p - 1))}
              >
                <ArrowLeft className="size-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                aria-label="Next stage"
                disabled={pos === FLAT.length - 1}
                onClick={() => setPos((p) => Math.min(FLAT.length - 1, p + 1))}
              >
                <ArrowRight className="size-4" />
              </Button>
            </div>
          </div>
        </header>

        {/* The controls only appear on the acts whose figures actually read them. */}
        {(showSource || showScenario) && (
          <div className="flex shrink-0 flex-wrap items-center gap-4 border-b px-4 py-2.5">
            {showSource && (
              <Tabs value={source} onValueChange={setSource}>
                <TabsList className="h-8">
                  <TabsTrigger value="parser" className="text-[12px]">
                    parser backend
                  </TabsTrigger>
                  <TabsTrigger value="scip" className="text-[12px]">
                    SCIP index
                  </TabsTrigger>
                </TabsList>
              </Tabs>
            )}
            {showScenario && (
              <Tabs value={scenarioId} onValueChange={setScenarioId}>
                <TabsList className="h-8">
                  {Object.values(SCENARIOS).map((s) => (
                    <TabsTrigger key={s.id} value={s.id} className="text-[12px]">
                      {s.label}
                    </TabsTrigger>
                  ))}
                </TabsList>
              </Tabs>
            )}
            <p className="text-[11.5px] text-muted-foreground">
              {showScenario
                ? `${SCENARIOS[scenarioId].seed}: ${SCENARIOS[scenarioId].blurb}. Replayed against the real graph; ${SCENARIOS[scenarioId].affected} affected, exit ${SCENARIOS[scenarioId].exit}.`
                : source === "scip"
                  ? "SCIP path: compiler-grade symbols, and no way for the tool to express doubt."
                  : "Parser path: this is what the committed graph artifact actually used."}
            </p>
          </div>
        )}

        <div ref={paneRef} className="min-h-0 flex-1 overflow-y-auto">
          <AnimatePresence mode="wait">
            <motion.div
              key={act.id + "/" + stage.id}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -6 }}
              transition={{ duration: 0.28, ease: [0.22, 0.61, 0.36, 1] }}
            >
              {onLanding ? (
                <Landing onStart={() => setPos(1)} />
              ) : (
                <StageView stage={stage} index={si} source={source} scenarioId={scenarioId} />
              )}
            </motion.div>
          </AnimatePresence>

          {!onLanding && (
            <div className="mx-auto flex max-w-6xl flex-wrap items-center gap-2 px-5 pb-10">
              <Button variant="outline" size="sm" onClick={() => setPos((p) => Math.max(0, p - 1))}>
                <ArrowLeft className="size-4" /> Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={pos === FLAT.length - 1}
                onClick={() => setPos((p) => Math.min(FLAT.length - 1, p + 1))}
              >
                Next <ArrowRight className="size-4" />
              </Button>
              <Badge variant="secondary" className="ml-auto font-mono text-[10px] font-normal">
                {stage.refs[0]}
              </Badge>
            </div>
          )}

          {pos === FLAT.length - 1 && <Provenance />}
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
