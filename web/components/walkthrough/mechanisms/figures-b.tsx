"use client";

import * as React from "react";
import { AnimatePresence, motion } from "motion/react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Table, Th, Td } from "./table";
import {
  Callout,
  Code,
  Lbl,
  Node,
  PathChain,
  Row,
  Scale,
  ScrollX,
  StaggerItem,
  StaggerStack,
  itemV,
  riskTone,
  useT,
} from "../primitives";
import { EVAL, G, type Scenario } from "@/lib/walkthrough/data";

const LT = "<";

type P = { sc: Scenario };

/* ── III · read the diff ──────────────────────────────────────────── */

export function MechDiff({ sc }: P) {
  const [cut, setCut] = React.useState(false);
  const t = useT();
  React.useEffect(() => {
    setCut(false);
    const timer = setTimeout(() => setCut(true), 950);
    return () => clearTimeout(timer);
  }, [sc.id]);

  const visible = cut ? sc.hunk.filter((l) => l.t !== "del") : sc.hunk;

  return (
    <div>
      <Lbl>ParseUnifiedDiff → hunk body, line by line</Lbl>
      <div className="flex flex-col gap-0.5 font-mono text-[11.5px]">
        <AnimatePresence mode="popLayout">
          {visible.map((l, i) => (
            <motion.div
              key={sc.id + l.s + i}
              layout
              initial={{ opacity: 0, x: -8 }}
              animate={{ opacity: 1, x: 0 }}
              exit={{ opacity: 0, x: 34, filter: "blur(2px)" }}
              transition={t(0.36)}
              className="overflow-x-auto whitespace-pre rounded-[2px] px-2 py-[3px]"
              style={{
                background:
                  l.t === "add"
                    ? "var(--ts-exact-bg)"
                    : l.t === "del"
                      ? "color-mix(in oklab, var(--ts-alarm) 10%, transparent)"
                      : "transparent",
                color:
                  l.t === "add"
                    ? "var(--ts-exact)"
                    : l.t === "del"
                      ? "var(--ts-alarm)"
                      : "var(--muted-foreground)",
              }}
            >
              {(l.t === "add" ? "+" : l.t === "del" ? "-" : " ") + l.s}
            </motion.div>
          ))}
        </AnimatePresence>
      </div>

      <Lbl className="mt-5">changed line ranges, in new-file coordinates</Lbl>
      <Row>
        <Node tone="seed">{sc.file}</Node>
        <Node>{sc.ranges}</Node>
      </Row>

      {sc.kind === "delete" && (
        <Callout className="mt-3.5" style={{ borderLeftColor: "var(--ts-heuristic)" }}>
          Deleted lines do not exist in the new file, so the counter does not advance and a pure
          deletion produces <b>no range at all</b>. The fix: anchor a zero-width range at the deletion
          point, so it still overlaps the enclosing function. An addition at the same spot supersedes
          the anchor.
        </Callout>
      )}
    </div>
  );
}

/* ── III · map to functions ───────────────────────────────────────── */

export function MechMap({ sc }: P) {
  return (
    <div>
      <Lbl>MapDiffToFunctions: overlap, not name matching</Lbl>
      <Code>
        {`func linesOverlap(aStart, aEnd, bStart, bEnd int) bool {\n    return aStart `}
        <b>{LT}</b>
        {`= bEnd && bStart `}
        <b>{LT}</b>
        {`= aEnd\n}`}
      </Code>

      <Lbl className="mt-5">path reconciliation first</Lbl>
      <div className="flex flex-col gap-2">
        <Callout>
          diff says <b>{sc.file}</b> · the graph stores absolute OS paths
        </Callout>
        <Callout>
          matchGraphPath: exact hit, else longest <b>path-segment</b> suffix, ties broken
          alphabetically so the answer never depends on map iteration order
        </Callout>
        <Callout style={{ borderLeftColor: "var(--ts-exact)" }}>
          src/utils/helper.go ⊃ utils/helper.go ✓ · src/myutils/helper.go ⊅ utils/helper.go ✗
          (segments, not text)
        </Callout>
      </div>

      <Lbl className="mt-5">result</Lbl>
      <Row>
        <Node tone="seed">{sc.seed}</Node>
        <span className="font-mono text-[11px] text-muted-foreground">
          {sc.file}:{sc.line} · {sc.seedProd} production caller{sc.seedProd === 1 ? "" : "s"} ·{" "}
          {sc.seedExport ? "exported" : "unexported"}
        </span>
      </Row>

      <Scale>
        A whole-file add or delete short-circuits the overlap test: every function in that file
        becomes a changed function, because a deleted file&apos;s callers still need traversing.
      </Scale>
    </div>
  );
}

/* ── III · seed selection ─────────────────────────────────────────── */

export function MechSeeds({ sc }: P) {
  return (
    <div>
      <Lbl>seed selection: function nodes win</Lbl>
      <StaggerStack key={sc.id}>
        <StaggerItem variants={itemV} className="flex flex-wrap items-center gap-2">
          <span className="w-4 font-mono text-[10px] text-muted-foreground">01</span>
          <Node tone="seed">{sc.seed}</Node>
          <span className="font-mono text-[10px] text-[var(--ts-exact)]">function seed</span>
        </StaggerItem>
        <StaggerItem variants={itemV} className="flex flex-wrap items-center gap-2">
          <span className="w-4 font-mono text-[10px] text-muted-foreground">02</span>
          <Node className="opacity-45 line-through decoration-1">file node: {sc.file}</Node>
          <span className="font-mono text-[10px] text-[var(--ts-heuristic)]">dropped</span>
        </StaggerItem>
      </StaggerStack>

      <Callout className="mt-4">
        A file node is added <b>only</b> when no function in that file matched. Otherwise the file
        node and its functions would both seed the BFS, and everything reachable from either would be
        counted twice, inflating both the affected set and the caller counts the score is built on.
      </Callout>
      <Callout className="mt-2.5">
        Seeds are then excluded from their own results: the report answers{" "}
        <b>what does my change reach</b>, not <b>what did I change</b>, which you already know.
      </Callout>
    </div>
  );
}

/* ── III · the traversal ──────────────────────────────────────────── */

export function MechTraverse({ sc }: P) {
  const [wave, setWave] = React.useState(0);
  const t = useT();
  React.useEffect(() => {
    setWave(0);
    const timers = [900, 1700, 2500].map((ms, i) => setTimeout(() => setWave(i + 1), ms));
    return () => timers.forEach(clearTimeout);
  }, [sc.id]);

  const levels = [
    { d: 0, items: [{ n: sc.seed, seed: true } as const] },
    { d: 1, items: sc.top.filter((x) => x.d === 1) },
    { d: 2, items: sc.top.filter((x) => x.d === 2) },
    { d: 3, items: sc.top.filter((x) => x.d >= 3) },
  ].filter((l) => l.items.length);

  return (
    <div>
      <Lbl>reverse BFS: who calls this?</Lbl>
      <div className="flex flex-col gap-3.5">
        {levels.map((lv, li) => (
          <motion.div
            key={sc.id + "-" + lv.d}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: li <= wave ? 1 : 0.16, y: 0 }}
            transition={t(0.4)}
          >
            <div className="flex items-start gap-2">
              <span className="w-[52px] shrink-0 pt-1.5 font-mono text-[10px] text-muted-foreground">
                depth {lv.d}
              </span>
              <Row>
                {lv.items.map((it) => {
                  const item = it as { n: string; seed?: boolean; conf?: string; test?: boolean };
                  return (
                    <Node
                      key={item.n}
                      tone={item.seed ? "seed" : item.conf === "HEURISTIC" ? "heur" : "exact"}
                      className="gap-1.5"
                    >
                      {item.n}
                      {item.test && (
                        <span className="text-[9.5px] text-[var(--ts-heuristic)]">test</span>
                      )}
                    </Node>
                  );
                })}
              </Row>
            </div>
          </motion.div>
        ))}
      </div>

      <Lbl className="mt-5">the loop, in code order</Lbl>
      <Code>
        {`for head `}
        <b>{LT}</b>
        {` len(queue) {\n    curr := queue[head]; head++\n    `}
        <s>if curr.depth &gt;= maxDepth {"{ continue }"}</s>
        {`   `}
        <i className="text-muted-foreground">{"// bound checked BEFORE expanding"}</i>
        {`\n    for _, nb := range reverseAdj[curr.id] {\n        if visited[nb.source] { continue }      `}
        <i className="text-muted-foreground">{"// marked on enqueue, not on dequeue"}</i>
        {`\n        visited[nb.source] = true\n        ...\n    }\n}`}
      </Code>

      <Callout className="mt-3.5">
        Two details that are easy to animate backwards: the depth bound is checked <b>before</b> a
        node expands, so an over-deep frontier is never built and then trimmed; and <b>visited</b> is
        set when a node is enqueued, so a node reached twice keeps its first path found, which is the
        shortest one.
      </Callout>

      <Scale>
        Showing the top-scoring nodes per level. The full traversal for this change reached{" "}
        <b>{sc.affected} functions</b> across the {G.nodes}-node graph. IMPORTS edges are excluded
        from the reverse adjacency, and CALLS edges are followed only into function nodes. Otherwise a
        shared type becomes a hub and the radius floods.
      </Scale>
    </div>
  );
}

/* ── III · the ladder ─────────────────────────────────────────────── */

export function MechScore({ sc }: P) {
  const ladder: [string, string][] = [
    ["prod ≥ 10", "HIGH"],
    ["exported && prod ≥ 5", "HIGH"],
    ["depth ≤ 1 && exported && prod ≥ 1", "MEDIUM"],
    ["prod ≥ 3", "MEDIUM"],
    ["is a test && prod == 0", "LOW"],
    ["exported", "MEDIUM"],
    ["otherwise", "LOW"],
  ];

  return (
    <div>
      <Lbl>the ladder: first rule that fires wins</Lbl>
      <div className="mb-4 flex flex-col gap-2">
        {ladder.map(([c, r]) => (
          <Row key={c}>
            <Node className="min-w-[232px] whitespace-normal text-left">{c}</Node>
            <Node tone={riskTone(r)}>{r}</Node>
          </Row>
        ))}
      </div>

      <ScrollX>
        <Table>
          <thead>
            <tr>
              <Th>affected</Th>
              <Th num>score</Th>
              <Th>tier</Th>
              <Th num>prod</Th>
              <Th num>raw</Th>
              <Th num>d</Th>
              <Th>conf</Th>
            </tr>
          </thead>
          <tbody>
            {sc.top.map((x) => (
              <tr key={x.n}>
                <Td>
                  {x.n}
                  {x.test && (
                    <span className="ml-1.5 text-[9.5px] text-[var(--ts-heuristic)]">test</span>
                  )}
                </Td>
                <Td num>
                  <b>{x.rs}</b>
                </Td>
                <Td>
                  <Node tone={riskTone(x.risk)}>{x.risk}</Node>
                </Td>
                <Td num>{x.prod}</Td>
                <Td num style={{ color: x.raw !== x.prod ? "var(--ts-heuristic)" : undefined }}>
                  {x.raw}
                </Td>
                <Td num>{x.d}</Td>
                <Td>
                  <span
                    className="font-mono text-[10px]"
                    style={{
                      color: x.conf === "HEURISTIC" ? "var(--ts-heuristic)" : "var(--ts-exact)",
                    }}
                  >
                    {x.conf.toLowerCase()}
                  </span>
                </Td>
              </tr>
            ))}
          </tbody>
        </Table>
      </ScrollX>

      {sc.id === "hub" && (
        <Callout className="mt-3.5">
          Look at <b>Build</b>: raw fan-in 25, production fan-in 3. Twenty-two of its callers are
          tests. Counting raw callers would have called it HIGH; counting production callers puts it
          at MEDIUM. That gap is the whole argument for excluding test callers.
        </Callout>
      )}
      {sc.top.some((x) => x.test) && (
        <Callout className="mt-3.5" style={{ borderLeftColor: "var(--ts-exact)" }}>
          The <b>is a test &amp;&amp; prod == 0</b> rung is why that test row sits at LOW / 26 rather
          than MEDIUM / 56. It was added after this walkthrough measured the problem: every Go TestXxx
          is an <b>exported</b> name, so the export fallback below it used to tag every test in the
          blast radius MEDIUM, and the 6-point test penalty in the review score was far too small to
          sink them. Tests crowded the top of the list a reviewer is told to read first.
        </Callout>
      )}
    </div>
  );
}

/* ── III · why-paths ──────────────────────────────────────────────── */

export function MechWhy({ sc }: P) {
  return (
    <div>
      <Lbl>buildImpactPath: walk the BFS parent map back to the seed</Lbl>
      <StaggerStack key={sc.id} className="gap-3.5">
        {sc.top.map((x) => (
          <StaggerItem key={x.n} variants={itemV}>
            <PathChain path={x.path} conf={x.conf} />
            <div className="mt-1 font-mono text-[10.5px] text-muted-foreground">
              {x.f}:{x.l} · {x.why}
            </div>
          </StaggerItem>
        ))}
      </StaggerStack>

      <Callout className="mt-4">
        The path is reconstructed, not searched for. BFS already recorded a parent per node, so the
        chain is free. And because the search works outward in order, it is the shortest chain rather
        than just a chain.
      </Callout>

      {sc.top.some((x) => x.conf === "HEURISTIC") && (
        <Callout className="mt-2.5" style={{ borderLeftColor: "var(--ts-heuristic)" }}>
          One path here is heuristic. Some hop along it was resolved without knowing the
          receiver&apos;s type, so it is a name match the tool could not confirm. A path takes the{" "}
          <b>weakest</b> confidence along it, so the whole chain is marked heuristic and the row loses
          8 points. Note it is four hops deep: the further a claim travels, the more chances it has to
          pass through a link like that.
        </Callout>
      )}
    </div>
  );
}

/* ── III · ownership ──────────────────────────────────────────────── */

export function MechOwners({ sc }: P) {
  return (
    <div>
      <Lbl>two independent lookups, both best-effort</Lbl>
      <div className="flex flex-col gap-2">
        <Callout>
          <b>CODEOWNERS</b>, searched at ./CODEOWNERS, .github/CODEOWNERS and docs/CODEOWNERS. Globs
          matched with doublestar; <b>last matching rule wins</b>, which is GitHub&apos;s semantics,
          not first-match.
        </Callout>
        <Callout>
          <b>git log -1</b> per file for the last author, email and date.
        </Callout>
      </div>

      <Lbl className="mt-5">reviewers are suggested for both sets of files</Lbl>
      <Row>
        <Node tone="seed">changed: {sc.file}</Node>
        <Node tone="exact">
          affected: {sc.top.length} of {sc.affected} shown
        </Node>
      </Row>

      <Callout className="mt-4" style={{ borderLeftColor: "var(--ts-alarm)" }}>
        This repository has <b>no CODEOWNERS file</b>, so on this project the reviewer suggestion is
        empty and only the git-log author survives. A missing CODEOWNERS is not an error. It returns
        nil, and CodeownersFound stays false.
      </Callout>
      <Callout className="mt-2.5">
        Ownership is opt-in behind <b>--owners</b> because it shells out to git once per file, which
        is the slowest thing in the pipeline.
      </Callout>
    </div>
  );
}

/* ── III · the four renderings ────────────────────────────────────── */

export function MechSurfaces({ sc }: P) {
  const top = sc.top[0];
  return (
    <div>
      <Tabs defaultValue="github">
        <TabsList className="mb-3.5 h-8">
          <TabsTrigger value="terminal" className="text-[12px]">terminal</TabsTrigger>
          <TabsTrigger value="json" className="text-[12px]">json</TabsTrigger>
          <TabsTrigger value="github" className="text-[12px]">github markdown</TabsTrigger>
          <TabsTrigger value="report" className="text-[12px]">html report</TabsTrigger>
        </TabsList>

        <TabsContent value="terminal">
          <Code>
            {`TraceScope — blast radius\n\n  changed files      1\n  changed functions  1\n  affected functions ${sc.affected}\n\n  `}
            <s>{sc.high} HIGH</s>
            {`   `}
            <i className="text-[var(--ts-heuristic)]">{sc.med} MEDIUM</i>
            {`   ${sc.low} LOW`}
          </Code>
        </TabsContent>

        <TabsContent value="json">
          <Code>
            {`{\n  "affected_functions": [\n    {\n      "node": { "name": "${top.n}", "file_path": "${top.f}" },\n      "depth": ${top.d},\n      "risk": "${top.risk}",\n      "review_score": ${top.rs},\n      "confidence": "${top.conf}",\n      "caller_count": ${top.raw},\n      "reason": "${top.why}"\n    }\n  ],\n  "total_nodes": ${G.nodes}, "total_edges": ${G.edges}\n}`}
          </Code>
        </TabsContent>

        <TabsContent value="github">
          <Code>
            {LT + "!-- tracescope-blast-radius -->\n"}
            <b>## TraceScope — Blast Radius</b>
            {`\n\n| Metric | Value |\n|--------|-------|\n| Changed functions | 1 |\n| Affected functions | ${sc.affected} |\n| Risk | ${sc.high} high, ${sc.med} medium, ${sc.low} low |\n| Resolution confidence | ${G.rExact} exact, ${G.rHeur} heuristic, ${G.rAmb} ambiguous skipped, ${G.rUnres} unresolved |`}
          </Code>
          <Callout className="mt-3">
            The marker comment is how the tool finds its own previous comment and{" "}
            <b>edits it in place</b> instead of appending a new one on every push.
          </Callout>
        </TabsContent>

        <TabsContent value="report">
          <Callout>
            A standalone HTML file from <b>tracescope report</b>, holding the same AnalysisResult
            rendered for someone who was not at the terminal.
          </Callout>
        </TabsContent>
      </Tabs>

      <Callout className="mt-4">
        All four render the same <b>AnalysisResult</b> struct. Nothing is recomputed per surface, so
        the terminal and the PR comment cannot disagree.
      </Callout>
    </div>
  );
}

/* ── III · the exit code ──────────────────────────────────────────── */

export function MechExit({ sc }: P) {
  const t = useT();
  const codes = [
    { c: 0, l: "no significant risk", on: sc.exit === 0 },
    { c: 1, l: "HIGH risk found", on: sc.exit === 1 },
    { c: 2, l: "MEDIUM risk found", on: sc.exit === 2 },
    { c: 3, l: "TraceScope itself failed", on: false },
  ];

  return (
    <div>
      <Lbl>ResolveExit: the CI contract</Lbl>
      <div className="flex flex-col gap-2">
        {codes.map((x) => (
          <motion.div
            key={x.c}
            animate={{ opacity: x.on ? 1 : 0.4 }}
            transition={t(0.3)}
            className="flex flex-wrap items-center gap-2"
          >
            <Node
              tone={x.on ? (x.c === 1 ? "heur" : "exact") : "plain"}
              className="min-w-[42px] justify-center"
            >
              {x.c}
            </Node>
            <span
              className="font-mono text-[11.5px]"
              style={{ color: x.on ? "var(--foreground)" : "var(--muted-foreground)" }}
            >
              {x.l}
            </span>
            {x.on && <span className="font-mono text-[10px] text-[var(--ts-exact)]">this run</span>}
          </motion.div>
        ))}
      </div>

      <Lbl className="mt-5">what the workflow does with it</Lbl>
      <Code>
        {`if [ "$code" = "1" ] || [ "$code" = "3" ]; then exit "$code"; fi\nexit 0                       `}
        <i className="text-muted-foreground">{"// MEDIUM is advisory; the comment still posts"}</i>
      </Code>

      <Callout className="mt-3.5">
        Code 3 exists so a broken invocation cannot be mistaken for a high-risk PR. Without it, a
        missing graph and a dangerous change would both be non-zero, and the gate would block for the
        wrong reason.
      </Callout>
      <Callout className="mt-2.5">
        Posting the PR comment is <b>allowed to fail</b>. The exit code is the contract; the comment
        is a side effect. A transient gh or API hiccup logs a warning and the risk signal still
        stands.
      </Callout>

      {sc.exit === 2 && (
        <Callout className="mt-3.5" style={{ borderLeftColor: "var(--ts-alarm)" }}>
          This change exits <b>2</b>, which the workflow lets through. Seventy-three affected
          functions and none of them HIGH. Breadth and severity are separate axes, and only severity
          is wired to the gate.
        </Callout>
      )}
      {sc.exit === 1 && (
        <Callout className="mt-3.5" style={{ borderLeftColor: "var(--ts-alarm)" }}>
          The gate is blunt in the other direction too. Just{" "}
          <b>
            {G.ratedHigh} of {G.fn}
          </b>{" "}
          functions in this repo are rated HIGH, but they sit close to the middle of the call graph,
          so a great many small changes reach one and fail the build. A single HIGH anywhere in the
          radius fails it, however weak the path.
        </Callout>
      )}
    </div>
  );
}

/* ── IV · the evaluation ──────────────────────────────────────────── */

export function MechEval() {
  return (
    <div>
      <Lbl>replay: index each commit&apos;s tree in a throwaway worktree, analyse its own diff</Lbl>
      <Code>
        {`git worktree add --detach --force $tmp $sha\nWalkDirectory → ParseFiles → Build          `}
        <i className="text-muted-foreground">{"// parser backend, no SCIP, for replay speed"}</i>
        {`\ndiff C^1 C → MapDiffToFunctions → blast radius\nrecord: max review score, churn, affected count, max fan-in\n\nlabel: reverted   ("This reverts commit ...")\n       hot-fixed  (fix-shaped subject, overlapping files, within the window)`}
      </Code>

      <Tabs defaultValue="full" className="mt-4">
        <TabsList className="mb-3.5 h-8">
          <TabsTrigger value="full" className="text-[12px]">full corpus (n=300)</TabsTrigger>
          <TabsTrigger value="sub" className="text-[12px]">scored subset (n=99)</TabsTrigger>
        </TabsList>

        <TabsContent value="full">
          <ScrollX>
            <Table>
              <thead>
                <tr>
                  <Th>ranker</Th>
                  <Th num>AUC</Th>
                  <Th num>P@5</Th>
                  <Th num>P@10</Th>
                  <Th num>IFA</Th>
                </tr>
              </thead>
              <tbody>
                {EVAL.rankers.map((r) => (
                  <tr key={r.name}>
                    <Td style={{ color: r.mine ? "var(--ts-seed)" : undefined }}>
                      {r.name}
                      {r.mine ? " ←" : ""}
                    </Td>
                    <Td num>{r.auc.toFixed(3)}</Td>
                    <Td num>{r.p5.toFixed(3)}</Td>
                    <Td num>{r.p10.toFixed(3)}</Td>
                    <Td num>{r.ifa.toFixed(1)}</Td>
                  </tr>
                ))}
              </tbody>
            </Table>
          </ScrollX>
        </TabsContent>

        <TabsContent value="sub">
          <ScrollX>
            <Table>
              <thead>
                <tr>
                  <Th>ranker, restricted to commits the tool actually scored</Th>
                  <Th num>AUC</Th>
                </tr>
              </thead>
              <tbody>
                {EVAL.subset.rows.map((r) => (
                  <tr key={r.name}>
                    <Td style={{ color: r.mine ? "var(--ts-seed)" : undefined }}>
                      {r.name}
                      {r.mine ? " ←" : ""}
                    </Td>
                    <Td num>{r.auc.toFixed(3)}</Td>
                  </tr>
                ))}
              </tbody>
            </Table>
          </ScrollX>
        </TabsContent>
      </Tabs>

      <Callout className="mt-4" style={{ borderLeftColor: "var(--ts-alarm)" }}>
        <b>The honest reading.</b> On the full corpus the ladder (0.613) beats churn (0.595) and
        random (0.494). But{" "}
        <b>
          {EVAL.zero} of {EVAL.n} commits score zero
        </b>
        , including{" "}
        <b>
          {EVAL.zeroRisky} of the {EVAL.positives} risky ones
        </b>
        . Restrict to the {EVAL.subset.n} commits it actually scored and AUC collapses to{" "}
        <b>0.516</b>, behind raw fan-in at 0.541. Nearly all the signal is the rough test &ldquo;does
        this change touch resolved functions at all&rdquo;, not the weighting.
      </Callout>
      <Callout className="mt-2.5">
        <b>P@5 = 0.000 is a finding, not a bug.</b> The five widest-blast-radius commits were large
        refactors and features, not the ones that got hot-fixed. Impact is not probability of defect.
        The score models impact only, and big changes also attract more review.
      </Callout>
    </div>
  );
}
