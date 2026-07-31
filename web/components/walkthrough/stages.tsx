import type { ReactNode } from "react";

/**
 * The walkthrough, as four acts of numbered stages.
 *
 * Every stage carries the same four things: what it does (`h`), a one-line
 * orientation (`k`), why it is built that way (`why`), and the one place that
 * design stops working (`limit`). `refs` points at the lines in this repository
 * that back the claim, so nothing here is unfalsifiable.
 *
 * `mech` names the figure to render; the registry in ./mechanisms resolves it.
 */
export type Stage = {
  id: string;
  spine: string;
  /** Short label, used in the sidebar. */
  t: string;
  /** Headline, used on the stage itself. */
  h: string;
  /** One-line orientation under the headline. */
  k: string;
  mech: string;
  why: ReactNode;
  limit: string;
  refs: string[];
};

export type Act = {
  id: string;
  heroTitle: string;
  heroThesis: string;
  label: string;
  title: string;
  stages: Stage[];
};

export const ACTS: Act[] = [
  {
    id: "system",
    heroTitle: "What does this pull request actually touch?",
    heroThesis:
      "TraceScope reads your repository into a call graph, then uses it to tell you what a change reaches. Start here: the parts, and how they fit together.",
    label: "I · The system",
    title: "The parts, and how they connect",
    stages: [
      {
        id: "map",
        spine: "graph",
        t: "The whole system",
        h: "Every part, on one diagram",
        k: "Start here. Nine boxes, and every other page in this walkthrough is a detail inside one of them.",
        mech: "System",
        why: (
          <>
            <p>
              Two commands, one file between them. <strong>index</strong> writes graph.json,{" "}
              <strong>analyze</strong> reads it. The four smaller commands read the same file and
              write nothing, which is why they are cheap and why they can never disagree with the
              analysis.
            </p>
            <p>
              Data only moves along the arrows. There is no database, no daemon, and no shared state
              between runs beyond that one file.
            </p>
          </>
        ),
        limit:
          "Everything depends on one artifact, so everything inherits its faults. If the graph is missing an edge, the blast radius is missing a result, the hotspot ranking is missing a caller, and nothing anywhere reports a gap.",
        refs: ["internal/graph/store.go:20", "internal/cmd/analyze.go:112"],
      },
      {
        id: "run",
        spine: "files",
        t: "What it does",
        h: "Two commands, and that is the interface",
        k: "Those two commands in practice. You index once, then pipe a diff in as often as you like.",
        mech: "Run",
        why: (
          <>
            <p>
              Indexing is the expensive half. It opens every file in the repository. Analysis is the
              cheap half: it reads the saved graph and walks it. Keeping them apart means a pull
              request does not pay for re-reading code that has not changed.
            </p>
            <p>
              Analysis takes a diff on standard input and nothing else. No branch name, no remote, no
              token. Those are only needed if you also want a comment posted.
            </p>
          </>
        ),
        limit:
          "The graph is a snapshot, so it can describe a revision you are no longer on. Analysis refuses to run in that case instead of reporting against shifted line numbers. Outside a git checkout there is no commit to compare, so the check cannot fire at all.",
        refs: ["internal/cmd/index.go:36", "internal/cmd/analyze.go:49"],
      },
      {
        id: "artifact",
        spine: "graph",
        t: "The file it writes",
        h: "Everything it knows, in one JSON file",
        k: "Not a database and not a running service. A file you can open, diff and delete.",
        mech: "Artifact",
        why: (
          <>
            <p>
              Keeping the whole index in one readable file is what makes the rest of the tool
              inspectable. You can answer why did it think that by opening the file, and check a
              surprising result by searching it.
            </p>
            <p>
              It also records where it came from, so anything reading it can tell whether it
              describes the code in front of you.
            </p>
          </>
        ),
        limit:
          "Paths inside it are absolute, so the file is tied to the machine that wrote it. Move it and author lookups and path matching start to fail. The list of resolution problems stops at 200 entries, so it is a sample rather than a full account.",
        refs: ["internal/graph/types.go:98", "internal/graph/store.go:20"],
      },
      {
        id: "packages",
        spine: "graph",
        t: "Inside the box",
        h: "Eleven packages, all pointing one way",
        k: "The same system again, now as code: which package holds what, and who is allowed to call whom.",
        mech: "Packages",
        why: (
          <>
            <p>
              Dependencies only point downward on this diagram. Go refuses to compile an import
              cycle, so this is checked by the compiler rather than by convention.
            </p>
            <p>
              That is what lets the pieces move independently. A second parser can be added without
              the graph builder knowing, and a fourth output format without the analyzer knowing.
            </p>
          </>
        ),
        limit:
          "internal/cmd sits above everything and holds eleven files of flag parsing, config merging and ordering. It is the one place the layering does no work, and the first thing to split if the command set grows.",
        refs: ["cmd/tracescope/main.go:10", "internal/cmd/root.go:17"],
      },
      {
        id: "seams",
        spine: "graph",
        t: "What crosses",
        h: "Three structs carry everything",
        k: "Packages only talk to each other through three plain data types.",
        mech: "Seams",
        why: (
          <>
            <p>
              Every boundary is data, not behaviour. No interfaces, no callbacks. One package fills a
              struct, another reads it. That makes each side testable on its own: hand it a struct,
              check the struct that comes back.
            </p>
            <p>
              GraphData does double duty as the file format, so what is in memory and what is on disk
              cannot drift apart.
            </p>
          </>
        ),
        limit:
          "Because GraphData is also the saved file, any field added for internal convenience becomes part of a format other tools may read. Removing one is a breaking change, so the struct has to be treated as an API.",
        refs: ["internal/graph/types.go:98", "internal/analyzer/blast_radius.go:37"],
      },
      {
        id: "cycle",
        spine: "graph",
        t: "The one exception",
        h: "Where the layering had to be paid for",
        k: "The dependency graph has no cycles. One untyped field is the reason it can.",
        mech: "Cycle",
        why: (
          <>
            <p>
              Ownership needs the analyzer&apos;s types to do its job, so it imports the analyzer. The
              result then has to travel back inside AnalysisResult, which would make the analyzer
              import ownership and close a loop that Go will not compile.
            </p>
            <p>
              The way out was an untyped field and a cast at every read. It works, and it costs the
              one thing the other boundaries give you: the compiler no longer knows what is in there.
            </p>
          </>
        ),
        limit:
          "Every reader repeats the same cast, and a wrong type fails when the program runs rather than when it builds. Passing plain file paths into ownership instead of analyzer structs would have removed the dependency and kept the field typed.",
        refs: ["internal/analyzer/blast_radius.go:48", "internal/ownership/ownership.go:16"],
      },
      {
        id: "invariants",
        spine: "score",
        t: "The rules",
        h: "What the output is allowed to assume",
        k: "Eight properties the graph and the ranking have to hold. Each is enforced at one line and covered by a test.",
        mech: "Invariants",
        why: (
          <>
            <p>
              A rule written down with its failure mode tells the next person why the obvious
              simplification is wrong. The condition in the code cannot do that on its own.
            </p>
            <p>
              Several of them are not obvious. Counting call edges to get caller counts looks right
              until one function calls another inside a loop.
            </p>
          </>
        ),
        limit:
          "The package layering is checked by the compiler. These are checked only by tests and comments, so nothing structurally stops a later change from breaking one. The test would fail, but a test can be deleted as easily as it can be read.",
        refs: ["internal/graph/builder.go:316", "internal/analyzer/risk_scorer.go:69"],
      },
    ],
  },

  {
    id: "index",
    heroTitle: "First it has to read your code",
    heroThesis:
      "Indexing turns a directory of files into one graph of functions and the calls between them. This is where accuracy is won or lost, because everything later just reads what this produced.",
    label: "II · Indexing",
    title: "From files on disk to a graph you can query",
    stages: [
      {
        id: "walk",
        spine: "files",
        t: "Find the files",
        h: "What the tool can see",
        k: "One pass over the directory decides what will ever be in the graph.",
        mech: "Walk",
        why: (
          <>
            <p>
              Skipping happens on the way down. Turning back at node_modules means nothing inside it
              is ever opened. Dot directories are skipped too, with .github allowed through so
              workflow files stay visible.
            </p>
            <p>
              Symbolic links are not followed. On Windows, following them can send a directory walk
              into an endless loop.
            </p>
          </>
        ),
        limit:
          "Files are picked by extension only. A Go file excluded by a build tag is still read, generated code looks the same as hand-written code, and a minified file is spotted by its name rather than its contents.",
        refs: ["internal/parser/walker.go:46", "walker.go:20"],
      },
      {
        id: "parse",
        spine: "parse",
        t: "Read them",
        h: "A type checker for Go, a syntax tree for the rest",
        k: "The difference between knowing what x.Foo() refers to and guessing.",
        mech: "Frontends",
        why: (
          <>
            <p>
              Go gets the standard library&apos;s own parser and type checker, so a method call can be
              matched to the real type of the thing it was called on. Everything else gets
              tree-sitter, which is fast and works for any language but only sees shape, not meaning.
            </p>
            <p>
              Both produce the same FileResult, which is why the next stage does not care which one
              ran. A file with a syntax error keeps whatever was readable rather than being dropped.
            </p>
          </>
        ),
        limit:
          "The Go type checker runs on one file at a time and its errors are thrown away, so even the strong path is only partly type-aware. This is the main reason so many calls end up unresolved. Files over 10 MB are skipped without being reported as gaps.",
        refs: ["internal/parser/golang.go:129", "internal/parser/registry.go:35"],
      },
      {
        id: "source",
        spine: "parse",
        t: "Two backends",
        h: "Compiler-grade index, or best effort",
        k: "The most important branch in the tool: which backend builds the graph.",
        mech: "Scip",
        why: (
          <>
            <p>
              If a SCIP index already exists it wins. Otherwise the tool tries to generate one per
              language, each gated on a marker file like go.mod or package.json, and records what
              happened for every attempt.
            </p>
            <p>
              Recording the outcome matters more than it looks. An indexer that quietly failed would
              leave a thinner graph with nothing to say so.
            </p>
          </>
        ),
        limit:
          "SCIP indexes are considered fresh by file timestamp rather than content, unlike the parser cache which hashes. Touching a file with no edits throws away a still-valid index. The Go correction that widens a function to its body is Go-only, so the same class of mistake is still possible elsewhere.",
        refs: ["internal/cmd/index.go:226", "internal/graph/scip.go:257"],
      },
      {
        id: "resolve",
        spine: "graph",
        t: "Name to edge",
        h: "Deciding what a call points at",
        k: "A call site is a piece of text. Turning it into an edge is where a call graph is made or ruined.",
        mech: "Resolve",
        why: (
          <>
            <p>
              The parser backend tries a list of strategies in order and stops at the first that
              works, recording how it matched. If more than one definition fits, the edge is thrown
              away rather than guessed at.
            </p>
            <p>
              That is the central trade. A missing edge is something you can count. A wrong edge is
              something you cannot see, and it quietly spoils every answer built on top of it.
            </p>
          </>
        ),
        limit:
          "Fewer than a quarter of call sites resolve on this repository. The right way to read any blast radius is therefore at least this much, never exactly this much. Confidence also means different things per backend: SCIP marks every edge exact, so on that path the label carries no information.",
        refs: ["internal/graph/builder.go:592", "internal/graph/scip.go:395"],
      },
      {
        id: "incremental",
        spine: "parse",
        t: "Only what changed",
        h: "Why the second index is fast",
        k: "Files are hashed first. Unchanged ones are never read again.",
        mech: "Incremental",
        why: (
          <p>
            Each file&apos;s hash is stored with the graph. Next time, a file whose hash still matches
            is restored from the cache instead of being parsed.
          </p>
        ),
        limit:
          "Only reading is cached. The graph is rebuilt from scratch every time, because a change in one file can move where a call in an untouched file points. Caching the graph would give wrong answers, not just be harder.",
        refs: ["internal/parser/cache.go:19", "internal/cmd/index.go:110"],
      },
      {
        id: "bind",
        spine: "graph",
        t: "Stamp and save",
        h: "A graph that knows which commit it describes",
        k: "Line numbers mean nothing without knowing which version they came from.",
        mech: "Bind",
        why: (
          <>
            <p>
              The commit and the remote are written into the graph when it is saved, and analysis
              checks them before using it. A graph from another revision does not give a slightly
              worse answer. It gives a confident wrong one.
            </p>
            <p>
              The file is written to a temporary name and then renamed, so an interrupted index
              leaves the previous graph intact rather than half of a new one.
            </p>
          </>
        ),
        limit:
          "The remote is recorded but never checked, so a graph built from a different repository would be used without complaint as long as the commit happened to match.",
        refs: ["internal/cmd/index.go:507", "internal/graph/store.go:20"],
      },
    ],
  },

  {
    id: "analyse",
    heroTitle: "Then a diff arrives",
    heroThesis:
      "Analysis maps your change onto the graph, walks outward to find what depends on it, ranks what is worth reading, and returns a number CI can act on.",
    label: "III · Analysing",
    title: "From a patch to a ranked list and an exit code",
    stages: [
      {
        id: "diff",
        spine: "diff",
        t: "Read the diff",
        h: "Patch text into line numbers",
        k: "The input is a unified diff on standard input. Nothing else is needed.",
        mech: "Diff",
        why: (
          <>
            <p>
              Each hunk is walked line by line, tracking a position in the new version of the file.
              Added lines open a range and unchanged lines close it.
            </p>
            <p>Deletions are the awkward case, which is why one of the scenarios is built around them.</p>
          </>
        ),
        limit:
          "A binary file is spotted by looking for the word binary in the patch header, so a text file whose header happens to contain that word is skipped.",
        refs: ["internal/diff/parser.go:24", "parser.go:60"],
      },
      {
        id: "find",
        spine: "diff",
        t: "Find the functions",
        h: "Which functions those lines sit in",
        k: "Changed line ranges meet function boundaries. Where they overlap is your change.",
        mech: "Map",
        why: (
          <>
            <p>
              This is geometry, not name matching. A function counts as changed if any changed line
              falls inside it.
            </p>
            <p>
              The fiddly part is paths. The diff says internal/x.go and the graph stores a full path
              from the machine that built it, so they are compared segment by segment.
            </p>
          </>
        ),
        limit:
          "This is only as good as the line numbers in the graph, which is why the stale check matters. A graph one commit behind maps your diff onto whatever now sits at those lines.",
        refs: ["internal/analyzer/diff_mapper.go:19", "diff_mapper.go:111"],
      },
      {
        id: "seeds",
        spine: "radius",
        t: "Pick the starting points",
        h: "What the search begins from",
        k: "A rule that came from a bug: never start from a file and its functions at once.",
        mech: "Seeds",
        why: (
          <p>
            Functions are preferred. A file is only used as a starting point when none of its
            functions matched, which happens for config files or where the backend could not resolve
            anything.
          </p>
        ),
        limit:
          "When a source file contributes no functions, it starts the search at file level and the result is much broader. Nothing in the report distinguishes that from a genuinely small blast radius.",
        refs: ["internal/analyzer/blast_radius.go:112"],
      },
      {
        id: "walk",
        spine: "radius",
        t: "Walk the callers",
        h: "Outward from the change, one hop at a time",
        k: "Follow calls backwards to find what depends on what you touched. Stop at five hops.",
        mech: "Traverse",
        why: (
          <>
            <p>
              Edges are followed in reverse. If A calls B and B changed, A is affected. Calls, file
              membership and type inheritance all carry the effect outward. Imports do not, because
              file-level imports would drag in most of the repository.
            </p>
            <p>
              Two details decide correctness. The depth limit is checked before a node expands, so an
              over-deep frontier is never built. And a node is marked as seen when it joins the
              queue, not when it leaves, which is what makes the recorded route the shortest one.
            </p>
          </>
        ),
        limit:
          "Five hops is a default with no reasoning behind it. Anything further away is invisible, and the report does not say whether the search finished or simply ran out of room.",
        refs: ["internal/graph/query.go:23", "query.go:100"],
      },
      {
        id: "rank",
        spine: "score",
        t: "Rank them",
        h: "A category, then an order",
        k: "Two separate things that are easy to confuse: how bad it is, and what to read first.",
        mech: "Score",
        why: (
          <>
            <p>
              The category comes from a short list of rules about how many callers a function has,
              whether it is public, and how far away it is. The score then orders functions inside
              each category.
            </p>
            <p>
              Only callers outside tests are counted. Something called from forty tests and one
              handler is not a review priority, and counting every caller would say otherwise.
            </p>
          </>
        ),
        limit:
          "The categories are blunt. Anything from three to nine callers lands in the same band, and the ordering inside a band rests on numbers chosen by hand rather than measured. The last act shows what happens when they are measured.",
        refs: ["internal/analyzer/risk_scorer.go:40", "internal/analyzer/blast_radius.go:259"],
      },
      {
        id: "explain",
        spine: "score",
        t: "Show the route",
        h: "Why each result is in the list",
        k: "A ranked list nobody can check is a list nobody trusts.",
        mech: "Why",
        why: (
          <>
            <p>
              Every result carries the chain of calls that led to it, rebuilt from the search. That
              makes each row something you can verify by opening two files.
            </p>
            <p>
              A route is only as trustworthy as its weakest step. One guessed link marks the whole
              chain as guessed and costs the row points.
            </p>
          </>
        ),
        limit:
          "The route shown is the shortest one, which is not always the most important one. A function reachable by one odd path and twenty obvious ones shows only the path the search happened to find first.",
        refs: ["internal/analyzer/blast_radius.go:207", "internal/graph/query.go:125"],
      },
      {
        id: "owners",
        spine: "output",
        t: "Who should look",
        h: "Turning results into people",
        k: "The part most easily left out, and the first thing a reviewer actually wants.",
        mech: "Owners",
        why: (
          <p>
            Two sources that do not depend on each other: CODEOWNERS for what the project says, and
            git history for what actually happened. Reviewers are suggested for the affected files as
            well as the changed ones, since the whole point is that the risk is not where you typed.
          </p>
        ),
        limit:
          "Git history gives the last person to touch a file, which is not the same as the person who understands it, and it works per file, so one name stands in for everyone who works in a large one.",
        refs: ["internal/ownership/ownership.go:16", "internal/ownership/codeowners.go:79"],
      },
      {
        id: "report",
        spine: "exit",
        t: "Report and exit",
        h: "Four renderings and one number",
        k: "Everything so far is advice until it becomes an exit code.",
        mech: "Exit",
        why: (
          <>
            <p>
              Terminal, JSON, a pull request comment and a standalone page all render the same
              result, so they cannot disagree. The comment carries a hidden marker so a second run
              edits the existing one instead of adding another.
            </p>
            <p>
              Then it ends in a number. Zero is clean, one is high risk, two is medium, three means
              the tool could not do its job. Keeping three separate is the point: a broken run must
              never look like a dangerous change.
            </p>
          </>
        ),
        limit:
          "The gate is blunt. One high-risk result anywhere fails the build no matter how weak the route to it was, and the category is the only thing wired to it. A change touching seventy functions can pass while a four-line helper fails.",
        refs: ["internal/cmd/exit.go:22", "internal/output/github.go:19"],
      },
    ],
  },

  {
    id: "check",
    heroTitle: "Does any of it work?",
    heroThesis:
      "The rest of the tool, and the measurement that tries to prove the ranking does not work.",
    label: "IV · Checking",
    title: "The other commands, the settings, and the evidence",
    stages: [
      {
        id: "queries",
        spine: "graph",
        t: "Asking questions",
        h: "Two commands that need no diff",
        k: "One answers is there a path between these. The other answers what is fragile.",
        mech: "Queries",
        why: (
          <p>
            Both read the saved graph and add nothing to it. <strong>why</strong> finds the shortest
            chain of calls between two functions and refuses to guess when a name is ambiguous.{" "}
            <strong>hotspots</strong> ranks by how connected a function is, with no change involved at
            all.
          </p>
        ),
        limit:
          "why follows calls only, so a link through a shared type is invisible to it even though the blast radius follows those. hotspots ignores how trustworthy each edge is, counting a guessed call the same as a verified one, which puts a known mistake at the top of the list on this very repository.",
        refs: ["internal/graph/pathfinder.go:99", "internal/analyzer/hotspots.go:37"],
      },
      {
        id: "surfaces",
        spine: "output",
        t: "Seeing it",
        h: "A page you can send, an API you can call",
        k: "Two ways to look at the same graph without a terminal.",
        mech: "VisualSurfaces",
        why: (
          <p>
            <strong>report</strong> writes one HTML file with the graph drawing built in, so it opens
            anywhere with no network. <strong>serve</strong> exposes the same engine over HTTP for the
            dashboard.
          </p>
        ),
        limit:
          "The server has no login. Binding to localhost is the only thing protecting it, and the flag that exposes it removes that protection. The report embeds the whole graph no matter how small the change was, so its size follows the repository.",
        refs: ["internal/output/report.go:29", "internal/server/server.go:65"],
      },
      {
        id: "config",
        spine: "files",
        t: "Settings",
        h: "What you can change",
        k: "A flag beats the config file, which beats the built-in default.",
        mech: "Config",
        why: (
          <p>
            The file is found by searching upward from wherever you run, so one file at the top of a
            repository applies everywhere inside it. A value that is present but out of range is
            reported rather than silently ignored.
          </p>
        ),
        limit:
          "The risk thresholds decide what fails your build and have no command line flags, so trying a different setting means editing a file. A bad value leaves you on the defaults until you read the warning.",
        refs: ["internal/config/config.go:44", "internal/cmd/root.go:21"],
      },
      {
        id: "eval",
        spine: "score",
        t: "Measuring it",
        h: "Replaying history to grade the ranking",
        k: "The part of the tool built to prove the rest of it wrong.",
        mech: "Eval",
        why: (
          <>
            <p>
              It replays TraceScope across a repository&apos;s own history, marks which changes were
              later reverted or hot-fixed, and checks whether the ranking puts those first. It scores
              two other rankers at the same time: one based on how many lines changed, and one that is
              purely random.
            </p>
            <p>Those two comparisons are the point. Without them a number like 0.613 means nothing.</p>
          </>
        ),
        limit:
          "One repository, three hundred changes, and labels taken from commit messages rather than tracing blame properly. Read the direction, not the decimal. The random ranker landing near the halfway mark is the only evidence the measurement itself is sound.",
        refs: ["internal/eval/eval.go:86", "docs/EVALUATION.md"],
      },
    ],
  },
];

/** Flattened, in reading order, so the sidebar and prev/next agree on one sequence. */
export const FLAT = ACTS.flatMap((act, ai) =>
  act.stages.map((stage, si) => ({ act, stage, ai, si })),
);
