/**
 * The architecture, read out of the repository rather than drawn by hand.
 * Layers come from the production imports of every .go file; a cycle check over
 * those imports finds none.
 */

export const ARCH_BOXES = [
  { id: "cmd", label: "cmd", x: 210, y: 8, w: 120, sub: "11 files" },
  { id: "ownership", label: "ownership", x: 16, y: 78, w: 108, sub: "3" },
  { id: "output", label: "output", x: 136, y: 78, w: 96, sub: "6" },
  { id: "server", label: "server", x: 244, y: 78, w: 88, sub: "1" },
  { id: "eval", label: "eval", x: 344, y: 78, w: 84, sub: "1" },
  { id: "analyzer", label: "analyzer", x: 168, y: 152, w: 116, sub: "4 files" },
  { id: "graph", label: "graph", x: 168, y: 222, w: 116, sub: "7 files" },
  { id: "parser", label: "parser", x: 40, y: 292, w: 104, sub: "9" },
  { id: "diff", label: "diff", x: 172, y: 292, w: 84, sub: "1" },
  { id: "config", label: "config", x: 284, y: 292, w: 92, sub: "1" },
];

export const ARCH_ARROWS: [string, string][] = [
  ["ownership", "analyzer"],
  ["output", "analyzer"],
  ["server", "analyzer"],
  ["eval", "analyzer"],
  ["analyzer", "graph"],
  ["analyzer", "diff"],
  ["graph", "parser"],
];

export const ARCH = {
  layers: [
    {
      n: 0,
      label: "leaves: depend on nothing internal",
      pkgs: [
        { p: "parser", c: 9, t: 8, ext: "go-tree-sitter" },
        { p: "diff", c: 1, t: 1, ext: "go-diff" },
        { p: "config", c: 1, t: 1, ext: "yaml.v3" },
      ],
    },
    { n: 1, label: "the model", pkgs: [{ p: "graph", c: 7, t: 8, deps: ["parser"], ext: "scip" }] },
    { n: 2, label: "the decision", pkgs: [{ p: "analyzer", c: 4, t: 6, deps: ["diff", "graph"] }] },
    {
      n: 3,
      label: "consumers: read the decision, never make one",
      pkgs: [
        { p: "ownership", c: 3, t: 2, deps: ["analyzer", "diff"] },
        { p: "output", c: 6, t: 2, deps: ["analyzer", "graph", "ownership"] },
        { p: "server", c: 1, t: 1, deps: ["analyzer", "diff", "graph", "ownership"] },
        { p: "eval", c: 1, t: 1, deps: ["analyzer", "diff", "graph", "parser"] },
      ],
    },
    { n: 4, label: "wiring", pkgs: [{ p: "cmd", c: 11, t: 4, deps: ["everything above"] }] },
  ],
  seams: [
    {
      t: "parser.FileResult",
      from: "parser",
      to: "graph",
      d: "functions, classes, imports, call sites, in the same shape whether go/ast or tree-sitter produced it",
    },
    {
      t: "graph.GraphData",
      from: "graph",
      to: "analyzer · output · server · eval",
      d: "nodes and edges, plus a record of where they came from. This is also exactly what gets written to disk",
    },
    {
      t: "analyzer.AnalysisResult",
      from: "analyzer",
      to: "output · server",
      d: "one struct every surface renders. Nothing is recomputed per surface, so they cannot disagree",
    },
  ],
  fanIn: [
    { p: "analyzer", by: 5 },
    { p: "diff", by: 5 },
    { p: "graph", by: 5 },
    { p: "ownership", by: 3 },
    { p: "parser", by: 3 },
  ],
};

export const SEQUENCE = [
  { pkg: "cmd/tracescope", call: "main()", note: "one job: turn an error into an exit code" },
  { pkg: "internal/cmd", call: "runAnalyze", note: "merge config with flags, read the diff from stdin" },
  { pkg: "internal/diff", call: "ParseUnifiedDiff", note: "hunks → changed line ranges" },
  { pkg: "internal/graph", call: "Store.Load", note: "the saved graph; no parsing happens here" },
  { pkg: "internal/analyzer", call: "MapDiffToFunctions", note: "line ranges → seed functions" },
  { pkg: "internal/graph", call: "ComputeBlastRadius", note: "reverse BFS, bounded at depth 5" },
  { pkg: "internal/analyzer", call: "Score + computeReviewScore", note: "tier, then ranking key" },
  { pkg: "internal/ownership", call: "ResolveOwnership", note: "only with --owners; shells out to git" },
  { pkg: "internal/output", call: "PrintAnalysis", note: "render to terminal, JSON, markdown or HTML" },
  { pkg: "internal/cmd", call: "ResolveExit", note: "0 · 1 · 2 · 3, and the process ends" },
];

/**
 * The rules the graph and the ranking have to hold. Each states the rule, why it
 * is necessary, and the failure mode if it lapses.
 */
export const INVARIANTS = [
  {
    rule: "One CALLS edge per (caller, callee) pair",
    holds:
      "Caller counts are computed by counting edges, so the edge set has to be a set. Eleven calls from one function are one dependency, not eleven.",
    breaks:
      "A single caller is reported as many, and a function can cross the HIGH threshold on its own call volume.",
    where: "graph/builder.go:316",
  },
  {
    rule: "A call with a receiver never resolves by bare name alone",
    holds:
      "x.Foo() and Foo() are different references. Matching some definition of Foo without knowing what x is has not identified a target.",
    breaks: "A chained call binds to an unrelated local function of the same name, and is reported as EXACT.",
    where: "graph/builder.go:759",
  },
  {
    rule: "A test with no production callers is LOW risk",
    holds:
      "Every Go TestXxx is an exported identifier, so the exported-function rung would otherwise classify all of them as MEDIUM.",
    breaks: "Test functions occupy the top of the list a reviewer is directed to read first.",
    where: "analyzer/risk_scorer.go:69",
  },
  {
    rule: "Each piece of evidence enters the score exactly once",
    holds:
      "The risk tier already encodes caller count and export status. Anything added afterwards has to be something the tier does not already know about: how far away it is, whether it is a test, how trustworthy the route was.",
    breaks:
      "Caller count is counted twice, and the caller term stops growing, so 6 and 600 callers end up ranked the same.",
    where: "analyzer/blast_radius.go:246",
  },
  {
    rule: "CALLS edges are followed only into function nodes",
    holds:
      "A SCIP index emits a CALLS edge from a function to each type it references. A call, by definition, targets a function.",
    breaks: "Shared types become hubs and the blast radius floods with every user of a common struct.",
    where: "graph/query.go:47",
  },
  {
    rule: "A deletion-only hunk anchors a zero-width range",
    holds:
      "Deleted lines have no position in the new file, so the line counter does not advance and the hunk yields no range at all.",
    breaks: "The enclosing function is never seeded, and a pure deletion reports a clean blast radius.",
    where: "diff/parser.go:66",
  },
  {
    rule: "A graph records the revision it was built from",
    holds:
      "Every line number in the graph is relative to one commit. Analysis compares that commit against HEAD and warns on drift.",
    breaks: "A diff maps onto whatever now occupies those line numbers, and the report is confidently wrong.",
    where: "cmd/index.go:507",
  },
  {
    rule: "The exit code is the contract; everything else is a side effect",
    holds:
      "Posting a PR comment can fail for reasons unrelated to the code under review: a token, a rate limit, a detached checkout.",
    breaks: "A transient API failure blocks a merge and the risk signal is lost behind an unrelated error.",
    where: "cmd/analyze.go:182",
  },
];

export { HOTSPOTS } from "./data";
