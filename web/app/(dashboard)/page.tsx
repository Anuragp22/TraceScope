"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { BarChart3, GitBranch, Box, FileCode, Braces, ArrowRight } from "lucide-react";
import Link from "next/link";

function StatCard({
  label,
  value,
  icon: Icon,
}: {
  label: string;
  value: number | string;
  icon: React.ElementType;
}) {
  return (
    <div className="bg-card border border-border rounded-lg p-4">
      <div className="flex items-center gap-3">
        <div className="p-2 rounded-md bg-accent">
          <Icon className="h-4 w-4 text-accent-foreground" />
        </div>
        <div>
          <p className="text-2xl font-bold">{value}</p>
          <p className="text-sm text-muted-foreground">{label}</p>
        </div>
      </div>
    </div>
  );
}

function HotspotRow({
  rank,
  name,
  coupling,
  risk,
  callers,
}: {
  rank: number;
  name: string;
  coupling: number;
  risk: string;
  callers: number;
}) {
  const riskColor =
    risk === "HIGH"
      ? "text-red-500"
      : risk === "MEDIUM"
      ? "text-yellow-500"
      : "text-green-500";

  return (
    <div className="flex items-center justify-between py-2 px-3 hover:bg-accent/50 rounded-md">
      <div className="flex items-center gap-3">
        <span className="text-muted-foreground text-sm w-6">{rank}</span>
        <span className="font-mono text-sm">{name}</span>
      </div>
      <div className="flex items-center gap-4 text-sm">
        <span className="text-muted-foreground">{callers} callers</span>
        <span className="text-muted-foreground">score {coupling}</span>
        <span className={`font-medium ${riskColor}`}>{risk}</span>
      </div>
    </div>
  );
}

export default function DashboardPage() {
  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: ["stats"],
    queryFn: api.getStats,
  });

  const { data: hotspots, isLoading: hotspotsLoading } = useQuery({
    queryKey: ["hotspots", 5],
    queryFn: () => api.getHotspots(5),
  });

  if (statsLoading) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground">
        Loading...
      </div>
    );
  }

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <p className="text-muted-foreground text-sm mt-1">
          Dependency graph overview
        </p>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard label="Total Nodes" value={stats?.total_nodes ?? 0} icon={Box} />
        <StatCard label="Total Edges" value={stats?.total_edges ?? 0} icon={GitBranch} />
        <StatCard label="Functions" value={stats?.node_types?.function ?? 0} icon={Braces} />
        <StatCard label="Files" value={stats?.node_types?.file ?? 0} icon={FileCode} />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="bg-card border border-border rounded-lg p-4">
          <h3 className="font-medium mb-3">Edge Types</h3>
          <div className="space-y-2">
            {stats && Object.entries(stats.edge_types).map(([type, count]) => (
              <div key={type} className="flex justify-between text-sm">
                <span className="text-muted-foreground">{type}</span>
                <span className="font-mono">{count}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="bg-card border border-border rounded-lg p-4">
          <h3 className="font-medium mb-3">Languages</h3>
          <div className="space-y-2">
            {stats && Object.entries(stats.languages).map(([lang, count]) => (
              <div key={lang} className="flex justify-between text-sm">
                <span className="text-muted-foreground">{lang}</span>
                <span className="font-mono">{count}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="bg-card border border-border rounded-lg p-4">
        <div className="flex items-center justify-between mb-3">
          <h3 className="font-medium flex items-center gap-2">
            <BarChart3 className="h-4 w-4" /> Top Hotspots
          </h3>
          <Link href="/hotspots" className="text-sm text-primary flex items-center gap-1 hover:underline">
            View all <ArrowRight className="h-3 w-3" />
          </Link>
        </div>
        {hotspotsLoading ? (
          <p className="text-muted-foreground text-sm">Loading...</p>
        ) : (
          <div className="space-y-1">
            {hotspots?.hotspots.map((h, i) => (
              <HotspotRow
                key={h.node.id}
                rank={i + 1}
                name={h.node.name}
                coupling={h.coupling_score}
                risk={h.risk}
                callers={h.inbound_callers}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
