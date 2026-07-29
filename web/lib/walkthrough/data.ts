/**
 * MEASURED FACTS
 *
 * Read out of this repository at commit ad6c837. Graph counts come from
 * .tracescope/graph.json, rebuilt from a cold cache; scenario results come from
 * replaying graph/query.go, analyzer/risk_scorer.go and analyzer/blast_radius.go
 * against it. Eval figures come from docs/EVALUATION.md.
 *
 * Nothing here is illustrative. If a number changes in the repository it has to
 * be re-derived here rather than adjusted by hand.
 */

const base = {
  nodes: 662,
  edges: 1483,
  fn: 494,
  files: 93,
  cls: 75,
  calls: 771,
  contains: 632,
  imports: 80,
  exact: 1378,
  heuristic: 105,
  rExact: 853,
  rHeur: 150,
  rAmb: 46,
  rUnres: 3234,
  goNodes: 599,
  tsNodes: 63,
  exported: 246,
  tests: 186,
  nonTestFns: 308,
  ratedHigh: 6,
  connectedFns: 465,
  issuesStored: 200,
  issueCap: 200,
  metaEntries: 93,
  artifactKB: 508,
  commit: "ad6c837",
  // The graph was rebuilt at HEAD, so these match. The stale-graph stage shows
  // what the check does when they do not.
  head: "ad6c837",
  remote: "github.com/Anuragp22/TraceScope",
  source: "parser",
  coldIndexSec: 9.6,
};

const callSites = base.rExact + base.rHeur + base.rAmb + base.rUnres;

export const G = {
  ...base,
  callSites,
  // Resolved call SITES over total call sites. Not G.calls, which is a deduped
  // edge count: dividing an edge count by a site count mixes units.
  resolvedPct: Math.round(((base.rExact + base.rHeur) / callSites) * 1000) / 10,
};

export const EVAL = {
  repo: "gin-gonic/gin",
  n: 300,
  positives: 59,
  base: 19.7,
  window: 30,
  rankers: [
    { name: "ladder", auc: 0.613, p5: 0.0, p10: 0.2, ifa: 6.0, mine: true },
    { name: "churn", auc: 0.595, p5: 0.0, p10: 0.0, ifa: 13.0 },
    { name: "random", auc: 0.494, p5: 0.22, p10: 0.2, ifa: 5.7 },
  ],
  zero: 201,
  zeroRisky: 29,
  subset: {
    n: 99,
    base: 30.3,
    rows: [
      { name: "max fan-in", auc: 0.541 },
      { name: "review-score sum", auc: 0.528 },
      { name: "churn", auc: 0.526 },
      { name: "review score (the ladder)", auc: 0.516, mine: true },
    ],
  },
};

/** Top hotspots, computed by analyzer/hotspots.go as inbound*2 + outbound. */
export const HOTSPOTS = [
  { n: "Error", f: "internal/analyzer/blast_radius.go", l: 56, i: 47, o: 0, c: 94, suspect: true },
  { n: "Build", f: "internal/graph/builder.go", l: 34, i: 25, o: 13, c: 63 },
  { n: "NewBuilder", f: "internal/graph/builder.go", l: 26, i: 25, o: 0, c: 50 },
  { n: "Analyze", f: "internal/analyzer/blast_radius.go", l: 95, i: 12, o: 9, c: 33 },
  { n: "NewGoParser", f: "internal/parser/golang.go", l: 15, i: 16, o: 0, c: 32 },
  { n: "BuildFromSCIP", f: "internal/graph/scip.go", l: 17, i: 15, o: 1, c: 31 },
];

/** The six functions the ladder rates HIGH, and why. */
export const HIGH_FNS = [
  { n: "canonicalPath", f: "internal/graph/builder.go", l: 915, prod: 15, raw: 15, rule: "prod ≥ 10" },
  { n: "New", f: "internal/server/server.go", l: 44, prod: 8, raw: 11, rule: "exported && prod ≥ 5" },
  { n: "Error", f: "internal/analyzer/blast_radius.go", l: 56, prod: 6, raw: 47, rule: "exported && prod ≥ 5", suspect: true },
  { n: "Analyze", f: "internal/analyzer/blast_radius.go", l: 95, prod: 5, raw: 12, rule: "exported && prod ≥ 5" },
  { n: "NewBlastRadiusAnalyzer", f: "internal/analyzer/blast_radius.go", l: 84, prod: 5, raw: 12, rule: "exported && prod ≥ 5" },
  { n: "ParseUnifiedDiff", f: "internal/diff/parser.go", l: 24, prod: 5, raw: 10, rule: "exported && prod ≥ 5" },
];

export type HunkLine = { t: "ctx" | "del" | "add"; s: string };

export type TopRow = {
  n: string;
  f: string;
  l: number;
  d: number;
  prod: number;
  raw: number;
  risk: "HIGH" | "MEDIUM" | "LOW";
  rs: number;
  conf: "EXACT" | "HEURISTIC";
  why: string;
  path: string[];
  test?: boolean;
};

export type Scenario = {
  id: string;
  label: string;
  blurb: string;
  seed: string;
  file: string;
  line: number;
  seedProd: number;
  seedExport: boolean;
  kind: "modify" | "delete";
  hunk: HunkLine[];
  ranges: string;
  affected: number;
  high: number;
  med: number;
  low: number;
  exit: number;
  lesson: string;
  top: TopRow[];
};

/** Replayed against the indexed graph using the real logic. */
export const SCENARIOS: Record<string, Scenario> = {
  leaf: {
    id: "leaf",
    label: "Leaf helper",
    blurb: "a one-line predicate with a single caller",
    seed: "linesOverlap",
    file: "internal/analyzer/diff_mapper.go",
    line: 144,
    seedProd: 1,
    seedExport: false,
    kind: "modify",
    hunk: [
      { t: "ctx", s: "func linesOverlap(aStart, aEnd, bStart, bEnd int) bool {" },
      { t: "del", s: "\treturn aStart <= bEnd && bStart <= aEnd" },
      { t: "add", s: "\treturn aStart <= bEnd && bStart <= aEnd && aEnd >= bStart" },
      { t: "ctx", s: "}" },
    ],
    ranges: "144–147",
    affected: 24,
    high: 1,
    med: 2,
    low: 21,
    exit: 1,
    lesson: "A one-line change to a private helper fails the build. Diff size predicts nothing about reach.",
    top: [
      { n: "Analyze", f: "internal/analyzer/blast_radius.go", l: 95, d: 2, prod: 5, raw: 12, risk: "HIGH", rs: 106, conf: "EXACT", why: "exported function with 5 production callers", path: ["linesOverlap", "MapDiffToFunctions", "Analyze"] },
      { n: "MapDiffToFunctions", f: "internal/analyzer/diff_mapper.go", l: 19, d: 1, prod: 1, raw: 8, risk: "MEDIUM", rs: 74, conf: "EXACT", why: "direct exported dependency (1 production callers)", path: ["linesOverlap", "MapDiffToFunctions"] },
      { n: "Run", f: "internal/eval/eval.go", l: 86, d: 4, prod: 2, raw: 3, risk: "MEDIUM", rs: 53, conf: "HEURISTIC", why: "exported/public function", path: ["linesOverlap", "MapDiffToFunctions", "Analyze", "replayCommit", "Run"] },
      { n: "TestMapDiffToFunctions_BasicOverlap", f: "internal/analyzer/diff_mapper_test.go", l: 10, d: 2, prod: 0, raw: 0, risk: "LOW", rs: 26, conf: "EXACT", test: true, why: "test function with no production callers", path: ["linesOverlap", "MapDiffToFunctions", "TestMapDiffToFunctions_BasicOverlap"] },
    ],
  },
  hub: {
    id: "hub",
    label: "Hub helper",
    blurb: "15 production callers, every one in the same file",
    seed: "canonicalPath",
    file: "internal/graph/builder.go",
    line: 915,
    seedProd: 15,
    seedExport: false,
    kind: "modify",
    hunk: [
      { t: "ctx", s: "func canonicalPath(path string) string {" },
      { t: "del", s: '\tpath = strings.ReplaceAll(path, "\\\\", "/")' },
      { t: "add", s: '\tpath = strings.TrimSpace(strings.ReplaceAll(path, "\\\\", "/"))' },
      { t: "ctx", s: "\treturn filepath.ToSlash(filepath.Clean(path))" },
    ],
    ranges: "915–918",
    affected: 73,
    high: 0,
    med: 5,
    low: 68,
    exit: 2,
    lesson: "The widest radius on the page, 73 functions, and it still passes the gate. Breadth and severity are different things.",
    top: [
      { n: "Build", f: "internal/graph/builder.go", l: 34, d: 1, prod: 3, raw: 25, risk: "MEDIUM", rs: 79, conf: "EXACT", why: "direct exported dependency (3 production callers)", path: ["canonicalPath", "Build"] },
      { n: "BuildFromSCIPFiles", f: "internal/graph/scip.go", l: 23, d: 2, prod: 3, raw: 4, risk: "MEDIUM", rs: 73, conf: "EXACT", why: "moderately connected (3 production callers)", path: ["canonicalPath", "loadSourceLines", "BuildFromSCIPFiles"] },
      { n: "Run", f: "internal/eval/eval.go", l: 86, d: 4, prod: 2, raw: 3, risk: "MEDIUM", rs: 53, conf: "HEURISTIC", why: "exported/public function", path: ["canonicalPath", "Build", "buildGraphAt", "replayCommit", "Run"] },
    ],
  },
  deletion: {
    id: "deletion",
    label: "Deletion only",
    blurb: "a hunk that removes lines and adds none",
    seed: "mergeConfidence",
    file: "internal/graph/query.go",
    line: 125,
    seedProd: 1,
    seedExport: false,
    kind: "delete",
    hunk: [
      { t: "ctx", s: "func mergeConfidence(pathConfidence, edgeConfidence EdgeConfidence) EdgeConfidence {" },
      { t: "del", s: "\tif pathConfidence == EdgeConfidenceHeuristic || edgeConfidence == EdgeConfidenceHeuristic {" },
      { t: "del", s: "\t\treturn EdgeConfidenceHeuristic" },
      { t: "del", s: "\t}" },
      { t: "ctx", s: "\treturn EdgeConfidenceExact" },
    ],
    ranges: "126 (zero-width anchor)",
    affected: 23,
    high: 1,
    med: 2,
    low: 20,
    exit: 1,
    lesson: "Deleted lines do not exist in the new file. Without a zero-width anchor this diff produces no ranges, no seeds, and a clean bill of health.",
    top: [
      { n: "Analyze", f: "internal/analyzer/blast_radius.go", l: 95, d: 2, prod: 5, raw: 12, risk: "HIGH", rs: 106, conf: "EXACT", why: "exported function with 5 production callers", path: ["mergeConfidence", "ComputeBlastRadius", "Analyze"] },
      { n: "ComputeBlastRadius", f: "internal/graph/query.go", l: 23, d: 1, prod: 1, raw: 7, risk: "MEDIUM", rs: 74, conf: "EXACT", why: "direct exported dependency (1 production callers)", path: ["mergeConfidence", "ComputeBlastRadius"] },
      { n: "TestComputeBlastRadius_BoundedDepth", f: "internal/graph/query_test.go", l: 43, d: 2, prod: 0, raw: 0, risk: "LOW", rs: 26, conf: "EXACT", test: true, why: "test function with no production callers", path: ["mergeConfidence", "ComputeBlastRadius", "TestComputeBlastRadius_BoundedDepth"] },
    ],
  },
  gate: {
    id: "gate",
    label: "Diff parser",
    blurb: "a path helper one hop from the tool's front door",
    seed: "cleanDiffPath",
    file: "internal/diff/parser.go",
    line: 162,
    seedProd: 1,
    seedExport: false,
    kind: "modify",
    hunk: [
      { t: "ctx", s: "func cleanDiffPath(path string) string {" },
      { t: "del", s: '\tif len(path) > 2 && (path[:2] == "a/" || path[:2] == "b/") {' },
      { t: "add", s: '\tif len(path) > 2 && (path[:2] == "a/" || path[:2] == "b/" || path[:2] == "i/") {' },
      { t: "ctx", s: "\t\treturn path[2:]" },
    ],
    ranges: "162–165",
    affected: 14,
    high: 1,
    med: 1,
    low: 12,
    exit: 1,
    lesson: "The tightest radius on the page, 14 functions, and the highest single score. One hop to a public entry point beats breadth every time.",
    top: [
      { n: "ParseUnifiedDiff", f: "internal/diff/parser.go", l: 24, d: 1, prod: 5, raw: 10, risk: "HIGH", rs: 112, conf: "EXACT", why: "exported function with 5 production callers", path: ["cleanDiffPath", "ParseUnifiedDiff"] },
      { n: "Run", f: "internal/eval/eval.go", l: 86, d: 3, prod: 2, raw: 3, risk: "MEDIUM", rs: 65, conf: "EXACT", why: "exported/public function", path: ["cleanDiffPath", "ParseUnifiedDiff", "replayCommit", "Run"] },
      { n: "replayCommit", f: "internal/eval/eval.go", l: 226, d: 2, prod: 1, raw: 1, risk: "LOW", rs: 38, conf: "EXACT", why: "internal function with few callers", path: ["cleanDiffPath", "ParseUnifiedDiff", "replayCommit"] },
      { n: "handleAnalyze", f: "internal/server/server.go", l: 163, d: 2, prod: 0, raw: 0, risk: "LOW", rs: 32, conf: "EXACT", why: "internal function with few callers", path: ["cleanDiffPath", "ParseUnifiedDiff", "handleAnalyze"] },
    ],
  },
};

export const GLOSSARY: [string, string][] = [
  ["call graph", "Functions as nodes, one function calling another as a directed edge. TraceScope also keeps file nodes (CONTAINS) and type nodes (EXTENDS / IMPLEMENTS)."],
  ["blast radius", "Everything that could be affected by a change. Here: a reverse breadth-first search from the functions your diff touched, following callers upward to a bounded depth."],
  ["seed", "A node the diff actually changed. The search starts from the seed set; seeds are excluded from their own results."],
  ["depth", "Hops from a seed. Depth 1 is a direct caller. The default bound is 5."],
  ["fan-in", "How many call edges point at a function. High fan-in means changing it reaches a lot of code."],
  ["production caller", "A caller that is not a test function. The scorer counts only these, so a helper called by forty tests and one handler is not treated as critical."],
  ["risk tier", "HIGH / MEDIUM / LOW, from a fixed ladder of rules over production caller count, export status and depth."],
  ["review score", "An integer used only for ordering. Tier base (80/50/20) plus a log1p caller term plus adjustments for depth, test-ness and confidence."],
  ["confidence", "EXACT or HEURISTIC, per edge, recording how the edge was resolved. A path takes the weakest link along it."],
  ["why-path", "The chain of edges from the seed to an affected function, rebuilt from the search's parent map. It is the shortest such chain, because breadth-first search finds nodes in depth order."],
  ["SCIP", "Sourcegraph Code Intelligence Protocol. A compiler-grade index: a real toolchain (scip-go, scip-typescript) emits every symbol with a globally unique ID, so a reference points at exactly one definition."],
  ["tree-sitter", "An incremental parser that produces a syntax tree without a compiler. Fast and works for any language, but it only sees shape, so it cannot tell you which Foo a name refers to."],
  ["go/types", "Go's standard-library type checker. TraceScope runs it to learn the real type of a method receiver, which is what makes Go call edges exact rather than guessed."],
  ["hotspot", "A function ranked by static coupling alone, with no diff involved: inbound×2 + outbound."],
  ["CODEOWNERS", "GitHub's file mapping globs to owners. The last matching rule wins. TraceScope reads it to suggest reviewers."],
  ["unified diff", "The standard patch format git emits. TraceScope reads it on stdin and needs nothing else about your change."],
  ["exit code", "The number the process returns. 0 clean, 1 HIGH risk, 2 MEDIUM risk, 3 the tool itself failed. This is the whole CI contract."],
  ["AUC", "Probability a random risky commit outranks a random clean one. 0.5 is chance."],
  ["Precision@k", "Of the top k ranked commits, the fraction that actually went wrong."],
  ["IFA", "Initial False Alarms: how far down the list the first genuinely risky change appears. Lower is better."],
  ["churn", "Added plus deleted lines. The baseline the defect-prediction literature keeps warning is hard to beat."],
];

/* ── the system diagram ───────────────────────────────────────────── */

export type SysBox = {
  id: string;
  t: string;
  s: string;
  x: number;
  y: number;
  w: number;
  kind: "in" | "cmd" | "file" | "data" | "out";
};

export const SYS: { boxes: SysBox[]; arrows: [string, string, string, string][] } = {
  boxes: [
    { id: "src", t: "source files", s: "93 in this repo", x: 14, y: 8, w: 118, kind: "in" },
    { id: "diff", t: "git diff", s: "on stdin", x: 300, y: 8, w: 104, kind: "in" },

    { id: "index", t: "tracescope index", s: "walk · parse · resolve", x: 14, y: 74, w: 152, kind: "cmd" },
    { id: "analyze", t: "tracescope analyze", s: "map · walk · rank", x: 262, y: 74, w: 160, kind: "cmd" },

    { id: "graph", t: "graph.json", s: "662 nodes · 1,483 edges", x: 76, y: 148, w: 168, kind: "file" },
    { id: "cache", t: "parse_cache.json", s: "sha256 per file", x: 14, y: 214, w: 142, kind: "file" },

    { id: "result", t: "AnalysisResult", s: "one struct, four renderings", x: 262, y: 148, w: 168, kind: "data" },

    { id: "out", t: "terminal · JSON · PR comment · HTML", s: "", x: 236, y: 224, w: 220, kind: "out" },
    { id: "exit", t: "exit code", s: "0 · 1 · 2 · 3", x: 236, y: 278, w: 104, kind: "out" },

    { id: "read", t: "why · hotspots · report · serve", s: "all read the same graph", x: 14, y: 296, w: 200, kind: "cmd" },
  ],
  arrows: [
    ["src", "index", "b", "t"],
    ["index", "graph", "b", "t"],
    ["index", "cache", "b", "t"],
    ["graph", "analyze", "r", "b"],
    ["diff", "analyze", "b", "t"],
    ["analyze", "result", "b", "t"],
    ["result", "out", "b", "t"],
    ["out", "exit", "b", "t"],
    ["graph", "read", "b", "t"],
  ],
};

/** What each box means, shown beside the diagram on the landing page. */
export const SYS_LEGEND = [
  { kind: "in", label: "what you give it" },
  { kind: "cmd", label: "commands" },
  { kind: "file", label: "files on disk" },
  { kind: "out", label: "what you get back" },
] as const;
