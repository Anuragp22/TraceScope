"use client";

import * as React from "react";
import { AnimatePresence, motion } from "motion/react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Table, Th, Td, Bar } from "./table";
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
import { G } from "@/lib/walkthrough/data";

const LT = "<";

/* ── I · what it does ─────────────────────────────────────────────── */

export function MechRun() {
  const [step, setStep] = React.useState(0);
  const t = useT();
  React.useEffect(() => {
    setStep(0);
    const timers = [700, 1500, 2400].map((ms, i) => setTimeout(() => setStep(i + 1), ms));
    return () => timers.forEach(clearTimeout);
  }, []);

  return (
    <div>
      <Lbl>two commands, and that is the whole interface</Lbl>
      <Code>
        {"$ "}
        <b>tracescope index .</b>
        {"\n"}
        {step >= 1
          ? `  Found ${G.files} files across 2 languages\n  Parsed ${G.files} files (0 errors)\n  Built graph: ${G.nodes} nodes, ${G.edges.toLocaleString()} edges\n  Saved to .tracescope/graph.json\n  Done in ${G.coldIndexSec}s`
          : "  …"}
      </Code>

      <AnimatePresence>
        {step >= 2 && (
          <motion.div
            key="analyze"
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={t(0.4)}
            className="mt-3"
          >
            <Code>
              {"$ "}
              <b>git diff | tracescope analyze</b>
              {"\n"}
              {step >= 3 ? "  Changed Files (1):\n    internal/diff/parser.go\n  Blast Radius (14 affected):\n    " : "  …"}
              {step >= 3 && <s>HIGH RISK (1):</s>}
              {step >= 3 &&
                "\n      ParseUnifiedDiff (internal/diff/parser.go:24) [score 112, depth 1, exact]\n  Risk: 1 high, 1 medium, 12 low\n"}
            </Code>
            {step >= 3 && (
              <Row className="mt-2.5">
                <Node>$?</Node>
                <Node tone="heur">1</Node>
                <span className="font-mono text-[11px] text-muted-foreground">
                  HIGH risk found, so the CI job fails
                </span>
              </Row>
            )}
          </motion.div>
        )}
      </AnimatePresence>

      <Scale>
        Real output, trimmed. Indexing this repository cold takes <b>{G.coldIndexSec}s</b> and produces{" "}
        <b>{G.nodes} nodes</b>. Analysis then reads that file and finishes in milliseconds. It never
        touches your source again.
      </Scale>
    </div>
  );
}

/* ── I · the artifact ─────────────────────────────────────────────── */

export function MechArtifact() {
  const fields = [
    { k: "nodes[]", v: `${G.nodes}: functions, files, classes`, hot: true },
    { k: "edges[]", v: `${G.edges.toLocaleString()}: CALLS, CONTAINS, IMPORTS, EXTENDS, IMPLEMENTS`, hot: true },
    { k: "commit / repo_remote", v: `${G.commit} · ${G.remote}` },
    { k: "index_source", v: `"${G.source}": which backend built this` },
    { k: "indexer_statuses[]", v: "why each SCIP indexer ran, was cached, or was skipped" },
    { k: "file_metadata{}", v: `${G.metaEntries} entries: sha256 per file, for incremental re-indexing` },
    { k: "resolution_stats", v: "exact / heuristic / ambiguous / unresolved call counts" },
    { k: "resolution_issues[]", v: `${G.issuesStored} stored, capped at ${G.issueCap}`, warn: true },
  ];

  return (
    <div>
      <Lbl>
        .tracescope/graph.json · {G.artifactKB} KB of indented JSON
      </Lbl>
      <StaggerStack>
        {fields.map((f) => (
          <StaggerItem key={f.k} variants={itemV} className="flex flex-wrap items-baseline gap-2">
            <Node tone={f.hot ? "exact" : f.warn ? "heur" : "plain"} className="min-w-[168px]">
              {f.k}
            </Node>
            <span className="font-mono text-[11px] text-muted-foreground">{f.v}</span>
          </StaggerItem>
        ))}
      </StaggerStack>
      <Callout className="mt-4">
        Written to a temp file and renamed into place, so an interrupted index leaves the previous
        graph intact rather than a truncated one.
      </Callout>
      <Scale>
        One file, plain JSON, committed or not as you like. It is also the reason every other command
        is fast: <b>why</b>, <b>hotspots</b>, <b>report</b> and <b>serve</b> all read this and never
        re-parse anything.
      </Scale>
    </div>
  );
}

/* ── IV · report and serve ────────────────────────────────────────── */

export function MechVisualSurfaces() {
  return (
    <div>
      <Tabs defaultValue="report">
        <TabsList className="mb-3.5 h-8">
          <TabsTrigger value="report" className="text-[12px]">report</TabsTrigger>
          <TabsTrigger value="serve" className="text-[12px]">serve</TabsTrigger>
        </TabsList>

        <TabsContent value="report">
          <Code>
            {"$ "}
            <b>git diff | tracescope report --open</b>
            {"\n  Report saved to tracescope-report.html\n  Includes blast radius overlay (14 affected functions)"}
          </Code>
          <Callout className="mt-3">
            A single self-contained HTML file: D3 is <b>embedded in the binary</b> with go:embed, so
            the report opens with no network and no build step. The graph and analysis are injected as
            JSON with every <b>{LT}/</b> escaped, so a function named after a closing script tag
            cannot break out of the page.
          </Callout>
          <Callout className="mt-2.5" style={{ borderLeftColor: "var(--ts-alarm)" }}>
            It embeds the <em>entire</em> graph, so the file grows with the repository: every node and
            edge, whether or not the diff touched it.
          </Callout>
        </TabsContent>

        <TabsContent value="serve">
          <Code>
            {"$ "}
            <b>tracescope serve</b>
            {"\n  TraceScope API server running at http://127.0.0.1:4000\n\n  GET  /api/graph          GET  /api/stats      GET  /api/repo\n  GET  /api/hotspots       GET  /api/analyze/diff\n  POST /api/analyze        POST /api/reload"}
          </Code>
          <Callout className="mt-3">
            Binds to <b>loopback only</b> by default. It has no authentication and shells out to git
            against the working tree, so exposing it is opt-in via --host. The only git command it runs
            is <b>diff HEAD</b>. Nothing in the request picks the revision, so no query value ever
            reaches git&apos;s argument parser.
          </Callout>
          <Callout className="mt-2.5" style={{ borderLeftColor: "var(--ts-heuristic)" }}>
            /api/repo exists so the dashboard can check the served graph belongs to the repo you are
            looking at. Analysing one repo&apos;s diff against another&apos;s graph produces confident
            nonsense.
          </Callout>
        </TabsContent>
      </Tabs>
    </div>
  );
}

/* ── IV · configuration ───────────────────────────────────────────── */

export function MechConfig() {
  const rows = [
    { k: "max_depth", d: "5", f: "--depth" },
    { k: "format", d: "terminal", f: "--format" },
    { k: "top", d: "0 (all)", f: "--top" },
    { k: "ignore", d: "none", f: "--ignore", note: "union, not override" },
    { k: "graph_path", d: ".tracescope/graph.json", f: "none" },
    { k: "risk.high_callers", d: "10", f: "none" },
    { k: "risk.high_exported_callers", d: "5", f: "none" },
    { k: "risk.medium_callers", d: "3", f: "none" },
  ];

  return (
    <div>
      <Lbl>.tracescope.yaml: found by walking up from the working directory</Lbl>
      <Row className="mb-3.5">
        {["CLI flag", "config file", "built-in default"].map((s, i) => (
          <React.Fragment key={s}>
            {i > 0 && <span className="text-[11px] text-muted-foreground">beats</span>}
            <Node tone={i === 0 ? "exact" : "plain"}>{s}</Node>
          </React.Fragment>
        ))}
      </Row>

      <ScrollX>
        <Table>
          <thead>
            <tr>
              <Th>key</Th>
              <Th>default</Th>
              <Th>flag</Th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.k}>
                <Td>{r.k}</Td>
                <Td className="text-muted-foreground">{r.d}</Td>
                <Td style={{ color: r.f === "none" ? "var(--muted-foreground)" : "var(--ts-exact)" }}>
                  {r.f}
                  {r.note ? `  (${r.note})` : ""}
                </Td>
              </tr>
            ))}
          </tbody>
        </Table>
      </ScrollX>

      <Callout className="mt-3.5">
        A broken config falls back to the defaults and says so, rather than failing the run or, worse,
        silently leaving every setting at zero.
      </Callout>
      <Callout className="mt-2.5" style={{ borderLeftColor: "var(--ts-alarm)" }}>
        Config values only override when non-zero, which means <b>you cannot configure a zero</b>.
        Setting a risk threshold to 0 is indistinguishable from not setting it, so &ldquo;treat every
        function as highly connected&rdquo; is unreachable. The four risk thresholds also have no CLI
        flags at all, so they can only be set in the file.
      </Callout>
    </div>
  );
}

/* ── II · the walk ────────────────────────────────────────────────── */

export function MechWalk() {
  const [swept, setSwept] = React.useState(false);
  const t = useT();
  React.useEffect(() => {
    const timer = setTimeout(() => setSwept(true), 900);
    return () => clearTimeout(timer);
  }, []);

  const entries = [
    { n: "cmd/tracescope/", keep: true },
    { n: "internal/", keep: true },
    { n: ".github/", keep: true, note: "allow-listed dot dir" },
    { n: "web/app/", keep: true },
    { n: "node_modules/", keep: false },
    { n: "vendor/", keep: false },
    { n: ".git/", keep: false },
    { n: ".next/", keep: false },
    { n: "dist/", keep: false },
    { n: "__pycache__/", keep: false },
  ];
  const shown = swept ? entries.filter((e) => e.keep) : entries;

  return (
    <div>
      <Lbl>filepath.WalkDir: one pass, skip on the way down</Lbl>
      <motion.div layout className="flex flex-col items-start gap-2">
        <AnimatePresence mode="popLayout">
          {shown.map((e) => (
            <motion.div
              key={e.n}
              layout
              initial={{ opacity: 0, x: -6 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 26, filter: "blur(2px)" }}
              transition={t(0.34)}
            >
              <Node
                tone="plain"
                className={e.keep ? "gap-2" : "gap-2 opacity-45 line-through decoration-1"}
              >
                {e.n}
                {e.note && <span className="text-[9.5px] text-[var(--ts-exact)]">{e.note}</span>}
                {!e.keep && <span className="text-[9.5px] text-[var(--ts-alarm)]">SkipDir</span>}
              </Node>
            </motion.div>
          ))}
        </AnimatePresence>
      </motion.div>

      <Lbl className="mt-5">then, by extension</Lbl>
      <Row>
        {[".go", ".ts", ".tsx", ".js", ".jsx", ".py"].map((x) => (
          <Node key={x} tone="exact">
            {x}
          </Node>
        ))}
        <Node className="opacity-45 line-through decoration-1">.min.js</Node>
      </Row>

      <Scale>
        Drawing 10 of the directory entries this repo actually has. The walk resolved to{" "}
        <b>{G.files} file nodes</b> in the committed graph ({G.goNodes} Go nodes, {G.tsNodes}{" "}
        TypeScript nodes).
      </Scale>
    </div>
  );
}

/* ── II · which backend ───────────────────────────────────────────── */

export function MechScip({ source }: { source?: string }) {
  const scip = source === "scip";
  const t = useT();
  const states = [
    { n: "scip-go", st: scip ? "generated" : "missing_binary", ok: scip, why: scip ? "go.mod found" : "binary not on PATH" },
    { n: "scip-typescript", st: scip ? "generated" : "missing_binary", ok: scip, why: scip ? "package.json found" : "binary not on PATH" },
    { n: "scip-python", st: "skipped", ok: false, why: "disabled on native Windows" },
  ];

  return (
    <div>
      <Lbl>collectSCIPIndexes: the branch, in order</Lbl>
      <div className="mb-4 flex flex-col gap-2">
        <Callout>
          1 · does index.scip already sit at the repo root? → use it, state <b>used_existing</b>, stop
        </Callout>
        <Callout>2 · otherwise build a candidate per language and run each one</Callout>
      </div>

      <StaggerStack key={source}>
        {states.map((s) => (
          <StaggerItem key={s.n} variants={itemV} className="flex flex-wrap items-center gap-2">
            <Node tone={s.ok ? "exact" : "plain"} className="min-w-[168px]">
              {s.n}
            </Node>
            <span
              className="font-mono text-[10px]"
              style={{ color: s.ok ? "var(--ts-exact)" : "var(--ts-heuristic)" }}
            >
              {s.st}
            </span>
            <span className="font-mono text-[11px] text-muted-foreground">{s.why}</span>
          </StaggerItem>
        ))}
      </StaggerStack>

      <AnimatePresence mode="wait">
        <motion.div
          key={String(source)}
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -6 }}
          transition={t(0.3)}
          className="mt-4"
        >
          {scip ? (
            <Callout style={{ borderLeftColor: "var(--ts-exact)" }}>
              Every candidate produced an index → BuildFromSCIPFiles merges them. The parser backend
              never runs; graph.IndexSource stays unset.
            </Callout>
          ) : (
            <Callout style={{ borderLeftColor: "var(--ts-heuristic)" }}>
              No indexer produced anything → fall through to the parser backend. graph.IndexSource =
              &ldquo;parser&rdquo;. This is the path the committed artifact took.
            </Callout>
          )}
        </motion.div>
      </AnimatePresence>

      <Scale>
        The committed <b>.tracescope/graph.json</b> in this repo was built by the <b>parser</b> backend
        (index_source: &ldquo;parser&rdquo;), so this page can only quote measured numbers for that
        path. No SCIP-built graph of this repo is checked in, and inventing its node count would be a
        made-up number.
      </Scale>
    </div>
  );
}

/* ── II · name to edge ────────────────────────────────────────────── */

export function MechResolve({ source }: { source?: string }) {
  const scip = source === "scip";
  const rungs = [
    { r: "receiver + static receiver type (Go, from go/types)", c: "EXACT" },
    { r: "receiver resolved through the file's import map", c: "EXACT" },
    { r: "bare name, same file", c: "EXACT" },
    { r: "bare name, same package (Go)", c: "EXACT" },
    { r: "receiver.name, no static type, a guess", c: "HEURISTIC" },
    { r: "bare name, but the call had a receiver, matched without knowing what it is", c: "HEURISTIC" },
    { r: "more than one candidate → ambiguous, edge dropped", c: "AMBIGUOUS" },
    { r: "nothing matched → unresolved, edge dropped", c: "UNRESOLVED" },
  ];

  const outcomes: [string, number, string][] = [
    ["exact", G.rExact, "var(--ts-exact)"],
    ["heuristic", G.rHeur, "var(--ts-heuristic)"],
    ["ambiguous, dropped", G.rAmb, "var(--muted-foreground)"],
    ["unresolved, dropped", G.rUnres, "var(--ts-alarm)"],
  ];

  return (
    <div>
      {scip ? (
        <div>
          <Lbl>SCIP path: no ladder at all</Lbl>
          <Callout className="mb-3.5" style={{ borderLeftColor: "var(--ts-exact)" }}>
            A SCIP occurrence already carries a globally unique symbol string. The builder looks it up
            and emits the edge. Every call to addEdge in scip.go passes EdgeConfidenceExact.
          </Callout>
          <Callout style={{ borderLeftColor: "var(--ts-alarm)" }}>
            Which means: in SCIP mode the tool has <b>no way to express doubt</b>. HEURISTIC is its
            only way to record a doubt about an edge, and it is never used. Absence of a heuristic tag
            is not evidence of a correct graph.
          </Callout>
        </div>
      ) : (
        <div>
          <Lbl>resolveCall: first rung that matches wins</Lbl>
          <StaggerStack>
            {rungs.map((x, i) => (
              <StaggerItem key={x.r} variants={itemV} className="flex items-start gap-2">
                <span className="w-4 shrink-0 pt-1 font-mono text-[10px] text-muted-foreground">
                  {String(i + 1).padStart(2, "0")}
                </span>
                <Node
                  tone={x.c === "EXACT" ? "exact" : x.c === "HEURISTIC" ? "heur" : "plain"}
                  className={
                    x.c === "AMBIGUOUS" || x.c === "UNRESOLVED"
                      ? "whitespace-normal text-left opacity-50"
                      : "whitespace-normal text-left"
                  }
                >
                  {x.r}
                </Node>
              </StaggerItem>
            ))}
          </StaggerStack>
        </div>
      )}

      <Lbl className="mt-5">measured on this repo (parser backend)</Lbl>
      <ScrollX>
        <Table>
          <thead>
            <tr>
              <Th>outcome</Th>
              <Th num>call sites</Th>
              <Th>share</Th>
            </tr>
          </thead>
          <tbody>
            {outcomes.map(([n, v, c]) => (
              <tr key={n}>
                <Td style={{ color: c }}>{n}</Td>
                <Td num>{v.toLocaleString()}</Td>
                <Td>
                  <Bar width={(v / G.callSites) * 150} colour={c} />
                </Td>
              </tr>
            ))}
          </tbody>
        </Table>
      </ScrollX>

      <Lbl className="mt-5">what the label actually claims</Lbl>
      <ScrollX>
        <Table>
          <thead>
            <tr>
              <Th>backend</Th>
              <Th>exact means</Th>
              <Th>heuristic means</Th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <Td>parser</Td>
              <Td style={{ color: "var(--ts-exact)" }}>
                matched by type, by import, or by being the only candidate in the file or package
              </Td>
              <Td style={{ color: "var(--ts-heuristic)" }}>
                matched on the name alone, with no idea what the receiver was
              </Td>
            </tr>
            <tr>
              <Td>SCIP</Td>
              <Td style={{ color: "var(--ts-exact)" }}>a compiler symbol matched</Td>
              <Td className="text-muted-foreground">never used: every edge is marked exact</Td>
            </tr>
          </tbody>
        </Table>
      </ScrollX>

      <Callout className="mt-3" style={{ borderLeftColor: "var(--ts-alarm)" }}>
        So the two backends do not mean the same thing by the word. On the SCIP path there is no way
        for the tool to record a doubt, and two graphs of the same repository can both say all exact
        while disagreeing about which edges exist.
      </Callout>

      <Scale>
        {G.callSites.toLocaleString()} call sites seen · {G.calls.toLocaleString()} CALLS edges
        emitted · <b>{G.resolvedPct}% resolved</b>. The graph you analyse is roughly a quarter of the
        call sites in the source.
      </Scale>
    </div>
  );
}

/* ── II · stamp and save ──────────────────────────────────────────── */

export function MechBind() {
  return (
    <div>
      <Lbl>storeGraph: stamp the revision, then rename into place</Lbl>
      <Code>
        {`graphData.Commit     = gitHead(rootPath)\ngraphData.RepoRemote = gitOriginRemote(rootPath)\n\n`}
        <i className="text-muted-foreground">
          {"// Store.Save writes .graph-*.tmp then os.Rename,\n// a killed index leaves the old graph intact, never a half-written one"}
        </i>
      </Code>

      <Lbl className="mt-5">right now, in this repository</Lbl>
      <StaggerStack>
        <StaggerItem variants={itemV} className="flex flex-wrap items-center gap-2">
          <Node>graph.commit</Node>
          <Node tone="seed">{G.commit}</Node>
        </StaggerItem>
        <StaggerItem variants={itemV} className="flex flex-wrap items-center gap-2">
          <Node>git HEAD</Node>
          <Node tone="seed">{G.head}</Node>
        </StaggerItem>
        <StaggerItem variants={itemV}>
          <Callout style={{ borderLeftColor: "var(--ts-exact)" }}>in sync, so analysis runs</Callout>
        </StaggerItem>
      </StaggerStack>

      <Scale>
        These match because the graph was rebuilt at HEAD. When this page was first written they did
        not: the artifact was bound to <b>2c2d5e0</b> while HEAD was <b>4dc8ac0</b>, and every line
        number quoted from it had drifted. That is the failure the stamp exists to catch, and the
        reason every figure on this page was re-measured after the graph moved.
      </Scale>
    </div>
  );
}

/* ── II · the two frontends ───────────────────────────────────────── */

export function MechFrontends() {
  return (
    <div>
      <Tabs defaultValue="go">
        <TabsList className="mb-3.5 h-8">
          <TabsTrigger value="go" className="text-[12px]">go/ast + go/types</TabsTrigger>
          <TabsTrigger value="ts" className="text-[12px]">tree-sitter</TabsTrigger>
        </TabsList>

        <TabsContent value="go">
          <Code>
            {`ast.Inspect(file, func(n ast.Node) bool {\n    call, ok := n.(*ast.CallExpr)\n    ...\n    case *ast.SelectorExpr:\n        if sel := selections[fn]; sel != nil {\n            if sel.Kind() == types.MethodVal { `}
            <u>{"// a real method"}</u>
            {`\n                call.ReceiverType, call.ReceiverPackage =\n                    goReceiverTypeInfo(sel.Recv())     `}
            <u>{"// EXACT"}</u>
            {`\n            }\n        }\n})`}
          </Code>
          <Callout className="mt-3" style={{ borderLeftColor: "var(--ts-exact)" }}>
            Go gets a type checker. <b>types.Config.Check</b> fills a Selections map, so <b>x.Foo()</b>{" "}
            resolves to the method on x&apos;s real type, giving a call edge with EXACT confidence
            rather than a name guess.
          </Callout>
          <Callout className="mt-2.5" style={{ borderLeftColor: "var(--ts-heuristic)" }}>
            But the checker runs on <b>one file at a time</b> with importer.Default(), and its errors
            are swallowed. Cross-package inference is partial, which is precisely why{" "}
            {G.rUnres.toLocaleString()} call sites still end up unresolved.
          </Callout>
        </TabsContent>

        <TabsContent value="ts">
          <Code>
            {`parser := sitter.NewParser()\nparser.SetLanguage(lang)\nctx, cancel := context.WithTimeout(ctx, 30*time.Second)\ntree, err := parser.ParseCtx(ctx, nil, source)\n\n`}
            <i className="text-muted-foreground">
              {"// then a hand-written walk: call_expression → name + receiver\n// plus inferTSVariableType / findTSConstructorType to guess a type"}
            </i>
          </Code>
          <Callout className="mt-3" style={{ borderLeftColor: "var(--ts-heuristic)" }}>
            Tree-sitter returns syntax, not meaning. There is no type checker, so TypeScript receiver
            types are <b>inferred from local declarations and constructor calls</b>, which is good
            enough to be useful and weak enough that its edges are the ones tagged HEURISTIC.
          </Callout>
          <Callout className="mt-2.5" style={{ borderLeftColor: "var(--ts-alarm)" }}>
            The README scopes the tool to Go for a reason. TypeScript and Python are marked
            experimental, and Python SCIP indexing is skipped outright on native Windows.
          </Callout>
        </TabsContent>
      </Tabs>

      <Scale>
        Measured split in the committed graph: <b>{G.goNodes} Go nodes</b> against{" "}
        <b>{G.tsNodes} TypeScript nodes</b>. The Go path is the one that has been exercised.
      </Scale>
    </div>
  );
}

/* ── II · incremental ─────────────────────────────────────────────── */

export function MechIncremental() {
  const [pass, setPass] = React.useState(0);
  React.useEffect(() => {
    setPass(0);
    const timers = [900, 1900].map((ms, i) => setTimeout(() => setPass(i + 1), ms));
    return () => timers.forEach(clearTimeout);
  }, []);

  const files = [
    { n: "internal/graph/builder.go", changed: true },
    { n: "internal/graph/query.go", changed: false },
    { n: "internal/parser/golang.go", changed: false },
    { n: "internal/diff/parser.go", changed: false },
  ];

  return (
    <div>
      <Lbl>second index onward: hash first, parse only what moved</Lbl>
      <StaggerStack>
        {files.map((f) => (
          <StaggerItem key={f.n} variants={itemV} className="flex flex-wrap items-center gap-2">
            <Node className="min-w-[216px]">{f.n}</Node>
            {pass === 0 && <span className="font-mono text-[10px] text-muted-foreground">hashing</span>}
            {pass >= 1 && (
              <span
                className="font-mono text-[10px]"
                style={{ color: f.changed ? "var(--ts-heuristic)" : "var(--ts-exact)" }}
              >
                {f.changed ? "sha256 differs" : "sha256 matches"}
              </span>
            )}
            {pass >= 2 && (
              <span
                className="font-mono text-[11px]"
                style={{ color: f.changed ? "var(--ts-heuristic)" : "var(--muted-foreground)" }}
              >
                {f.changed ? "re-parsed" : "restored from parse_cache.json"}
              </span>
            )}
          </StaggerItem>
        ))}
      </StaggerStack>

      <Callout className="mt-4">
        The hash lives in <b>file_metadata</b> inside the graph, and the parsed result in a separate{" "}
        <b>parse_cache.json</b>. Both must be present for incremental mode; if either is missing the
        index falls back to a full parse rather than trusting half a cache.
      </Callout>

      <Lbl className="mt-5">what is not incremental</Lbl>
      <Callout style={{ borderLeftColor: "var(--ts-alarm)" }}>
        Only <b>parsing</b> is cached. The graph is rebuilt from scratch on every index, all three
        passes, every edge re-resolved, because one changed file can move where a call in an untouched
        file resolves to. Caching the graph itself would be wrong, not merely harder.
      </Callout>
      <Callout className="mt-2.5" style={{ borderLeftColor: "var(--ts-alarm)" }}>
        The SCIP path ignores this entirely: it has its own freshness check based on file{" "}
        <b>modification time</b>, not content hash. Touching a file with no edits invalidates a SCIP
        index that did not actually change.
      </Callout>

      <Scale>
        Measured here: a cold index of {G.files} files takes <b>{G.coldIndexSec}s</b>; a warm one with
        a single edited file re-parses 1 and restores {G.files - 1}.
      </Scale>
    </div>
  );
}
