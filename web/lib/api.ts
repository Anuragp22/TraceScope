const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:4000";

export interface GraphNode {
  id: string;
  type: "function" | "file" | "class";
  name: string;
  file_path: string;
  start_line?: number;
  end_line?: number;
  package?: string;
  language?: string;
  is_export?: boolean;
  is_test?: boolean;
  is_init?: boolean;
}

export interface GraphEdge {
  source: string;
  target: string;
  type: "CALLS" | "CONTAINS" | "IMPORTS" | "EXTENDS" | "IMPLEMENTS";
}

export interface GraphData {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface StatsData {
  total_nodes: number;
  total_edges: number;
  node_types: Record<string, number>;
  edge_types: Record<string, number>;
  languages: Record<string, number>;
}

export interface HotspotFunction {
  node: GraphNode;
  inbound_callers: number;
  outbound_calls: number;
  coupling_score: number;
  risk: "HIGH" | "MEDIUM" | "LOW";
}

export interface HotspotsResult {
  hotspots: HotspotFunction[];
  total_nodes: number;
  total_edges: number;
  top_n?: number;
  total?: number;
}

export interface AffectedFunction {
  node: GraphNode;
  depth: number;
  risk: "HIGH" | "MEDIUM" | "LOW";
  caller_count: number;
  reason: string;
  last_author?: string;
  last_modified?: string;
}

export interface ChangedFunction {
  node_id: string;
  node: GraphNode;
  file_path: string;
}

export interface ChangedFile {
  path: string;
  is_new?: boolean;
  is_deleted?: boolean;
}

export interface AnalysisResult {
  changed_functions: ChangedFunction[];
  affected_functions: AffectedFunction[];
  changed_files: ChangedFile[];
  total_nodes: number;
  total_edges: number;
  max_depth: number;
}

// RepoInfo identifies the repo + revision the served graph was built from, so
// the dashboard can scope PR analysis to the same codebase.
export interface RepoInfo {
  remote: string;
  commit: string;
  root_path: string;
}

async function fetchAPI<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, options);
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || `API error: ${res.status}`);
  }
  return res.json();
}

export const api = {
  getGraph: () => fetchAPI<GraphData>("/api/graph"),
  getStats: () => fetchAPI<StatsData>("/api/stats"),
  getHotspots: (top = 20, lang?: string) => {
    const params = new URLSearchParams({ top: String(top) });
    if (lang) params.set("lang", lang);
    return fetchAPI<HotspotsResult>(`/api/hotspots?${params}`);
  },
  getRepo: () => fetchAPI<RepoInfo>("/api/repo"),
  analyze: (diff: string, depth = 5) =>
    fetchAPI<AnalysisResult>(`/api/analyze?depth=${depth}`, {
      method: "POST",
      body: diff,
    }),
  // Analyze the served repo's own working-tree changes (server is localhost-only).
  analyzeLocal: (opts: { uncommitted?: boolean; depth?: number } = {}) => {
    const params = new URLSearchParams();
    if (opts.uncommitted !== false) params.set("uncommitted", "true");
    if (opts.depth) params.set("depth", String(opts.depth));
    return fetchAPI<AnalysisResult>(`/api/analyze/diff?${params}`);
  },
};
