"use client";

import { useQuery, useMutation } from "@tanstack/react-query";
import { api, type AnalysisResult } from "@/lib/api";
import { github, type GitHubRepo, type GitHubPR } from "@/lib/github";
import { useSession } from "@/lib/auth-client";
import { useState } from "react";
import { AlertTriangle, FileCode, Braces, Shield, GitBranch, Github, LogIn } from "lucide-react";
import { signIn } from "@/lib/auth-client";

function riskBadge(risk: string) {
  const styles = {
    HIGH: "bg-red-500/10 text-red-500 border-red-500/20",
    MEDIUM: "bg-yellow-500/10 text-yellow-500 border-yellow-500/20",
    LOW: "bg-green-500/10 text-green-500 border-green-500/20",
  }[risk] ?? "";
  return (
    <span className={`px-2 py-0.5 rounded-full text-xs font-medium border ${styles}`}>
      {risk}
    </span>
  );
}

function shortPath(filePath: string) {
  const parts = filePath.replace(/\\/g, "/").split("/");
  const idx = parts.findIndex((p) => p === "internal" || p === "cmd" || p === "pkg" || p === "src");
  return idx >= 0 ? parts.slice(idx).join("/") : parts.slice(-3).join("/");
}

function ResultsView({ result }: { result: AnalysisResult }) {
  const high = result.affected_functions.filter((f) => f.risk === "HIGH");
  const medium = result.affected_functions.filter((f) => f.risk === "MEDIUM");
  const low = result.affected_functions.filter((f) => f.risk === "LOW");

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-card border border-border rounded-lg p-4 text-center">
          <p className="text-2xl font-bold">{result.changed_files.length}</p>
          <p className="text-sm text-muted-foreground flex items-center justify-center gap-1">
            <FileCode className="h-3 w-3" /> Changed Files
          </p>
        </div>
        <div className="bg-card border border-border rounded-lg p-4 text-center">
          <p className="text-2xl font-bold">{result.changed_functions.length}</p>
          <p className="text-sm text-muted-foreground flex items-center justify-center gap-1">
            <Braces className="h-3 w-3" /> Changed Functions
          </p>
        </div>
        <div className="bg-card border border-border rounded-lg p-4 text-center">
          <p className="text-2xl font-bold">{result.affected_functions.length}</p>
          <p className="text-sm text-muted-foreground flex items-center justify-center gap-1">
            <AlertTriangle className="h-3 w-3" /> Affected
          </p>
        </div>
        <div className="bg-card border border-border rounded-lg p-4 text-center">
          <div className="flex justify-center gap-2">
            {high.length > 0 && <span className="text-red-500 font-bold">{high.length}H</span>}
            {medium.length > 0 && <span className="text-yellow-500 font-bold">{medium.length}M</span>}
            {low.length > 0 && <span className="text-green-500 font-bold">{low.length}L</span>}
            {result.affected_functions.length === 0 && <span className="text-green-500 font-bold">Clean</span>}
          </div>
          <p className="text-sm text-muted-foreground flex items-center justify-center gap-1 mt-1">
            <Shield className="h-3 w-3" /> Risk
          </p>
        </div>
      </div>

      <div className="bg-card border border-border rounded-lg p-4">
        <h3 className="font-medium mb-3">Changed Files</h3>
        <div className="space-y-1">
          {result.changed_files.map((f) => (
            <div key={f.path} className="text-sm font-mono text-muted-foreground flex items-center gap-2">
              <span>{f.path}</span>
              {f.is_new && <span className="text-green-500 text-xs">[NEW]</span>}
              {f.is_deleted && <span className="text-red-500 text-xs">[DELETED]</span>}
            </div>
          ))}
        </div>
      </div>

      {result.affected_functions.length > 0 && (
        <div className="bg-card border border-border rounded-lg overflow-hidden">
          <div className="px-4 py-3 border-b border-border">
            <h3 className="font-medium">Blast Radius ({result.affected_functions.length} affected)</h3>
          </div>
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-left text-muted-foreground">
                <th className="px-4 py-2">Function</th>
                <th className="px-4 py-2">File</th>
                <th className="px-4 py-2 text-right">Callers</th>
                <th className="px-4 py-2 text-right">Depth</th>
                <th className="px-4 py-2 text-center">Risk</th>
              </tr>
            </thead>
            <tbody>
              {result.affected_functions.map((af) => (
                <tr key={af.node.id} className="border-b border-border/50 hover:bg-accent/30">
                  <td className="px-4 py-2 font-mono font-medium">{af.node.name}</td>
                  <td className="px-4 py-2 text-muted-foreground font-mono text-xs">
                    {shortPath(af.node.file_path)}:{af.node.start_line}
                  </td>
                  <td className="px-4 py-2 text-right font-mono">{af.caller_count}</td>
                  <td className="px-4 py-2 text-right font-mono">{af.depth}</td>
                  <td className="px-4 py-2 text-center">{riskBadge(af.risk)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

export default function AnalyzePage() {
  const { data: session } = useSession();
  const [mode, setMode] = useState<"github" | "paste">("github");
  const [selectedRepo, setSelectedRepo] = useState("");
  const [selectedPR, setSelectedPR] = useState<number | null>(null);
  const [diff, setDiff] = useState("");

  const accessToken = (session as any)?.session?.accessToken || "";
  const isLoggedIn = !!session?.user;

  // Fetch repos
  const { data: repos } = useQuery({
    queryKey: ["github-repos", accessToken],
    queryFn: () => github.getRepos(accessToken),
    enabled: isLoggedIn && !!accessToken,
  });

  // Parse owner/repo
  const [repoOwner, repoName] = selectedRepo.split("/");

  // Fetch PRs for selected repo
  const { data: prs, isLoading: prsLoading } = useQuery({
    queryKey: ["github-prs", selectedRepo],
    queryFn: () => github.getPRs(accessToken, repoOwner, repoName),
    enabled: !!selectedRepo && !!accessToken,
  });

  // Analyze PR mutation
  const prMutation = useMutation({
    mutationFn: async (prNumber: number) => {
      const diffText = await github.getPRDiff(accessToken, repoOwner, repoName, prNumber);
      return api.analyze(diffText);
    },
  });

  // Analyze paste mutation
  const pasteMutation = useMutation({
    mutationFn: (diffText: string) => api.analyze(diffText),
  });

  const result = prMutation.data || pasteMutation.data;
  const isPending = prMutation.isPending || pasteMutation.isPending;
  const error = prMutation.error || pasteMutation.error;

  return (
    <div className="p-6 max-w-6xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Analyze</h1>
        <p className="text-muted-foreground text-sm mt-1">
          Compute blast radius from a pull request or diff
        </p>
      </div>

      {/* Mode tabs */}
      <div className="flex gap-1 bg-muted p-1 rounded-lg w-fit">
        <button
          onClick={() => setMode("github")}
          className={`px-4 py-1.5 rounded-md text-sm transition-colors flex items-center gap-1.5 ${
            mode === "github" ? "bg-background text-foreground shadow-sm" : "text-muted-foreground"
          }`}
        >
          <Github className="h-3.5 w-3.5" />
          GitHub PR
        </button>
        <button
          onClick={() => setMode("paste")}
          className={`px-4 py-1.5 rounded-md text-sm transition-colors ${
            mode === "paste" ? "bg-background text-foreground shadow-sm" : "text-muted-foreground"
          }`}
        >
          Paste Diff
        </button>
      </div>

      {/* GitHub PR mode */}
      {mode === "github" && (
        <div className="bg-card border border-border rounded-lg p-4 space-y-4">
          {!isLoggedIn ? (
            <div className="text-center py-8 space-y-3">
              <Github className="h-10 w-10 mx-auto text-muted-foreground" />
              <p className="text-muted-foreground">Sign in with GitHub to analyze pull requests</p>
              <button
                onClick={() => signIn.social({ provider: "github" })}
                className="px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm font-medium hover:bg-primary/90 inline-flex items-center gap-2"
              >
                <LogIn className="h-4 w-4" />
                Sign in with GitHub
              </button>
            </div>
          ) : (
            <>
              <div className="flex flex-wrap items-end gap-4">
                {/* Repo selector */}
                <div className="space-y-1.5">
                  <label className="text-sm text-muted-foreground">Repository</label>
                  <select
                    value={selectedRepo}
                    onChange={(e) => {
                      setSelectedRepo(e.target.value);
                      setSelectedPR(null);
                    }}
                    className="bg-background border border-border rounded-md px-3 py-1.5 text-sm w-64"
                  >
                    <option value="">Select repository...</option>
                    {repos?.map((r) => (
                      <option key={r.id} value={r.full_name}>
                        {r.full_name}
                      </option>
                    ))}
                  </select>
                </div>

                {/* PR selector */}
                <div className="space-y-1.5">
                  <label className="text-sm text-muted-foreground">Pull Request</label>
                  <select
                    value={selectedPR ?? ""}
                    onChange={(e) => setSelectedPR(e.target.value ? Number(e.target.value) : null)}
                    disabled={!selectedRepo || prsLoading}
                    className="bg-background border border-border rounded-md px-3 py-1.5 text-sm w-80 disabled:opacity-50"
                  >
                    <option value="">
                      {prsLoading ? "Loading PRs..." : "Select pull request..."}
                    </option>
                    {prs?.map((pr) => (
                      <option key={pr.number} value={pr.number}>
                        #{pr.number} — {pr.title}
                      </option>
                    ))}
                  </select>
                </div>

                {/* Analyze button */}
                <button
                  onClick={() => selectedPR && prMutation.mutate(selectedPR)}
                  disabled={!selectedPR || isPending}
                  className="px-4 py-1.5 bg-primary text-primary-foreground rounded-md text-sm font-medium hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {prMutation.isPending ? "Analyzing..." : "Analyze PR"}
                </button>
              </div>

              {selectedPR && prs && (
                <div className="text-xs text-muted-foreground">
                  {(() => {
                    const pr = prs.find((p) => p.number === selectedPR);
                    if (!pr) return null;
                    return (
                      <span>
                        {pr.base.ref} &larr; {pr.head.ref} by {pr.user.login}
                      </span>
                    );
                  })()}
                </div>
              )}

              {!repos?.length && (
                <p className="text-sm text-muted-foreground">
                  No repositories found. Make sure the GitHub App is installed on your repos.
                </p>
              )}
            </>
          )}
        </div>
      )}

      {/* Paste diff mode */}
      {mode === "paste" && (
        <div className="bg-card border border-border rounded-lg p-4 space-y-3">
          <textarea
            value={diff}
            onChange={(e) => setDiff(e.target.value)}
            placeholder={"Paste your git diff here...\n\nTip: Run `git diff` and paste the output"}
            className="w-full h-48 bg-background border border-border rounded-md p-3 font-mono text-sm resize-y focus:outline-none focus:ring-1 focus:ring-ring"
          />
          <button
            onClick={() => diff.trim() && pasteMutation.mutate(diff)}
            disabled={!diff.trim() || isPending}
            className="px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm font-medium hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {pasteMutation.isPending ? "Analyzing..." : "Analyze Diff"}
          </button>
        </div>
      )}

      {error && (
        <div className="bg-destructive/10 border border-destructive/20 rounded-lg p-3 text-sm text-destructive">
          {(error as Error).message}
        </div>
      )}

      {result && <ResultsView result={result} />}
    </div>
  );
}
