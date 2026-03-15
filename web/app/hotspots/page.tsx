"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { useState } from "react";
import { Info } from "lucide-react";

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

export default function HotspotsPage() {
  const [topN, setTopN] = useState(20);

  const { data, isLoading } = useQuery({
    queryKey: ["hotspots", topN],
    queryFn: () => api.getHotspots(topN),
  });

  return (
    <div className="p-6 max-w-6xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Hotspots</h1>
          <p className="text-muted-foreground text-sm mt-1">
            The riskiest functions in your codebase — most likely to cause breakage when changed
          </p>
        </div>
        <select
          value={topN}
          onChange={(e) => setTopN(Number(e.target.value))}
          className="bg-card border border-border rounded-md px-3 py-1.5 text-sm"
        >
          <option value={10}>Top 10</option>
          <option value={20}>Top 20</option>
          <option value={50}>Top 50</option>
          <option value={100}>Top 100</option>
        </select>
      </div>

      {isLoading ? (
        <div className="text-muted-foreground">Loading...</div>
      ) : (
        <>
          {data && data.total && data.total > topN && (
            <p className="text-sm text-muted-foreground">
              Showing {topN} of {data.total} functions with connections
            </p>
          )}
          {/* Column explanations */}
          <div className="bg-card/50 border border-border rounded-lg p-4 grid grid-cols-2 md:grid-cols-4 gap-4 text-xs">
            <div>
              <p className="font-medium text-foreground flex items-center gap-1 mb-1">
                <Info className="h-3 w-3 text-muted-foreground" /> Inbound
              </p>
              <p className="text-muted-foreground">How many other functions call this one. Higher = more things break if you change it.</p>
            </div>
            <div>
              <p className="font-medium text-foreground flex items-center gap-1 mb-1">
                <Info className="h-3 w-3 text-muted-foreground" /> Outbound
              </p>
              <p className="text-muted-foreground">How many functions this one calls. Higher = more dependencies it relies on.</p>
            </div>
            <div>
              <p className="font-medium text-foreground flex items-center gap-1 mb-1">
                <Info className="h-3 w-3 text-muted-foreground" /> Coupling
              </p>
              <p className="text-muted-foreground">Inbound × Outbound. High coupling = the function is a critical nexus in your code.</p>
            </div>
            <div>
              <p className="font-medium text-foreground flex items-center gap-1 mb-1">
                <Info className="h-3 w-3 text-muted-foreground" /> Risk
              </p>
              <p className="text-muted-foreground">
                <span className="text-red-500">HIGH</span> = 10+ callers,{" "}
                <span className="text-yellow-500">MEDIUM</span> = 5-9,{" "}
                <span className="text-green-500">LOW</span> = under 5.
              </p>
            </div>
          </div>

          <div className="bg-card border border-border rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-muted-foreground">
                  <th className="px-4 py-3 w-12">#</th>
                  <th className="px-4 py-3">Function</th>
                  <th className="px-4 py-3">File</th>
                  <th className="px-4 py-3 text-right">Inbound</th>
                  <th className="px-4 py-3 text-right">Outbound</th>
                  <th className="px-4 py-3 text-right">Coupling</th>
                  <th className="px-4 py-3 text-center">Risk</th>
                </tr>
              </thead>
              <tbody>
                {data?.hotspots.map((h, i) => (
                  <tr
                    key={h.node.id}
                    className="border-b border-border/50 hover:bg-accent/30 transition-colors"
                  >
                    <td className="px-4 py-3 text-muted-foreground">{i + 1}</td>
                    <td className="px-4 py-3">
                      <span className="font-mono font-medium">{h.node.name}</span>
                      {h.node.is_export && (
                        <span className="ml-2 text-xs text-muted-foreground">[exported]</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground font-mono text-xs">
                      {shortPath(h.node.file_path)}:{h.node.start_line}
                    </td>
                    <td className="px-4 py-3 text-right font-mono">{h.inbound_callers}</td>
                    <td className="px-4 py-3 text-right font-mono">{h.outbound_calls}</td>
                    <td className="px-4 py-3 text-right font-mono font-medium">
                      {h.coupling_score}
                    </td>
                    <td className="px-4 py-3 text-center">{riskBadge(h.risk)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
