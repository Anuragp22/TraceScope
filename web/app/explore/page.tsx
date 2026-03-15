"use client";

import { useQuery } from "@tanstack/react-query";
import { api, type GraphNode, type GraphEdge } from "@/lib/api";
import { useState, useMemo, useCallback, useRef, useEffect } from "react";
import dynamic from "next/dynamic";
import { X, Layers, FolderTree, Braces, ArrowDownLeft, ArrowUpRight } from "lucide-react";

// Dynamic import — react-force-graph-3d needs window
const ForceGraph3D = dynamic(() => import("react-force-graph-3d"), {
  ssr: false,
  loading: () => (
    <div className="flex items-center justify-center h-full text-muted-foreground">
      Loading 3D engine...
    </div>
  ),
});

type ViewLevel = "architecture" | "modules" | "functions";

interface GraphViewNode {
  id: string;
  name: string;
  val: number; // size
  color: string;
  type: string;
  group?: string;
  data?: GraphNode;
  // For architecture view
  packageName?: string;
  fileCount?: number;
  funcCount?: number;
}

interface GraphViewLink {
  source: string;
  target: string;
  color: string;
  particles?: number;
}

const PACKAGE_COLORS = [
  "#3b82f6", "#22c55e", "#f59e0b", "#ef4444", "#8b5cf6",
  "#ec4899", "#06b6d4", "#f97316", "#14b8a6", "#6366f1",
];

function getPackagePath(filePath: string): string {
  const parts = filePath.replace(/\\/g, "/").split("/");
  const idx = parts.findIndex(
    (p) => p === "internal" || p === "cmd" || p === "pkg" || p === "src" || p === "app" || p === "lib"
  );
  if (idx >= 0) {
    // Take up to 2 levels: internal/graph, cmd/tracescope, etc.
    return parts.slice(idx, Math.min(idx + 2, parts.length - 1)).join("/");
  }
  // Fallback: directory name
  return parts.slice(0, -1).join("/") || "root";
}

function shortPath(filePath: string): string {
  const parts = filePath.replace(/\\/g, "/").split("/");
  return parts[parts.length - 1];
}

function buildArchitectureView(
  nodes: GraphNode[],
  edges: GraphEdge[]
): { nodes: GraphViewNode[]; links: GraphViewLink[] } {
  // Group by package
  const packages = new Map<string, { files: Set<string>; funcs: number; nodes: GraphNode[] }>();

  nodes.forEach((n) => {
    if (n.type === "function" || n.type === "file") {
      const pkg = getPackagePath(n.file_path);
      if (!packages.has(pkg)) {
        packages.set(pkg, { files: new Set(), funcs: 0, nodes: [] });
      }
      const p = packages.get(pkg)!;
      p.files.add(n.file_path);
      if (n.type === "function") p.funcs++;
      p.nodes.push(n);
    }
  });

  const colorMap = new Map<string, string>();
  let colorIdx = 0;
  packages.forEach((_, pkg) => {
    colorMap.set(pkg, PACKAGE_COLORS[colorIdx % PACKAGE_COLORS.length]);
    colorIdx++;
  });

  const viewNodes: GraphViewNode[] = [];
  packages.forEach((data, pkg) => {
    viewNodes.push({
      id: `pkg:${pkg}`,
      name: pkg,
      val: Math.max(3, Math.sqrt(data.funcs) * 2),
      color: colorMap.get(pkg) || "#6b7280",
      type: "package",
      packageName: pkg,
      fileCount: data.files.size,
      funcCount: data.funcs,
    });
  });

  // Build cross-package edges
  const nodePackage = new Map<string, string>();
  nodes.forEach((n) => {
    nodePackage.set(n.id, `pkg:${getPackagePath(n.file_path)}`);
  });

  const linkSet = new Set<string>();
  const viewLinks: GraphViewLink[] = [];
  edges.forEach((e) => {
    if (e.type !== "CALLS") return;
    const srcPkg = nodePackage.get(e.source);
    const tgtPkg = nodePackage.get(e.target);
    if (srcPkg && tgtPkg && srcPkg !== tgtPkg) {
      const key = `${srcPkg}→${tgtPkg}`;
      if (!linkSet.has(key)) {
        linkSet.add(key);
        viewLinks.push({
          source: srcPkg,
          target: tgtPkg,
          color: "rgba(100, 150, 255, 0.3)",
          particles: 2,
        });
      }
    }
  });

  return { nodes: viewNodes, links: viewLinks };
}

function buildModulesView(
  nodes: GraphNode[],
  edges: GraphEdge[],
  filterPackage?: string
): { nodes: GraphViewNode[]; links: GraphViewLink[] } {
  const filtered = filterPackage
    ? nodes.filter((n) => getPackagePath(n.file_path) === filterPackage)
    : nodes;

  // Group by file
  const files = new Map<string, GraphNode[]>();
  filtered.forEach((n) => {
    if (n.type === "function") {
      const fp = n.file_path.replace(/\\/g, "/");
      if (!files.has(fp)) files.set(fp, []);
      files.get(fp)!.push(n);
    }
  });

  const colorMap = new Map<string, string>();
  let colorIdx = 0;
  files.forEach((_, fp) => {
    colorMap.set(fp, PACKAGE_COLORS[colorIdx % PACKAGE_COLORS.length]);
    colorIdx++;
  });

  const viewNodes: GraphViewNode[] = [];
  files.forEach((funcs, fp) => {
    viewNodes.push({
      id: `file:${fp}`,
      name: shortPath(fp),
      val: Math.max(2, Math.sqrt(funcs.length) * 1.5),
      color: colorMap.get(fp) || "#6b7280",
      type: "file",
      funcCount: funcs.length,
    });
  });

  // Cross-file call edges
  const nodeFile = new Map<string, string>();
  filtered.forEach((n) => {
    nodeFile.set(n.id, `file:${n.file_path.replace(/\\/g, "/")}`);
  });

  const linkSet = new Set<string>();
  const viewLinks: GraphViewLink[] = [];
  edges.forEach((e) => {
    if (e.type !== "CALLS") return;
    const srcFile = nodeFile.get(e.source);
    const tgtFile = nodeFile.get(e.target);
    if (srcFile && tgtFile && srcFile !== tgtFile) {
      const key = `${srcFile}→${tgtFile}`;
      if (!linkSet.has(key)) {
        linkSet.add(key);
        viewLinks.push({
          source: srcFile,
          target: tgtFile,
          color: "rgba(100, 150, 255, 0.2)",
          particles: 1,
        });
      }
    }
  });

  return { nodes: viewNodes, links: viewLinks };
}

function buildFunctionsView(
  nodes: GraphNode[],
  edges: GraphEdge[],
  filterFile?: string
): { nodes: GraphViewNode[]; links: GraphViewLink[] } {
  const inbound: Record<string, number> = {};
  edges.forEach((e) => {
    if (e.type === "CALLS") inbound[e.target] = (inbound[e.target] || 0) + 1;
  });

  let funcs = nodes.filter((n) => n.type === "function");
  if (filterFile) {
    funcs = funcs.filter((n) => n.file_path.replace(/\\/g, "/") === filterFile);
  }

  const funcIds = new Set(funcs.map((n) => n.id));

  const viewNodes: GraphViewNode[] = funcs.map((n) => {
    const callers = inbound[n.id] || 0;
    const risk = callers >= 10 ? "HIGH" : callers >= 5 ? "MEDIUM" : "LOW";
    const color = risk === "HIGH" ? "#ef4444" : risk === "MEDIUM" ? "#f59e0b" : "#3b82f6";
    return {
      id: n.id,
      name: n.name,
      val: Math.max(1, Math.sqrt(callers + 1)),
      color,
      type: "function",
      data: n,
    };
  });

  const viewLinks: GraphViewLink[] = [];
  edges.forEach((e) => {
    if (e.type === "CALLS" && funcIds.has(e.source) && funcIds.has(e.target)) {
      viewLinks.push({
        source: e.source,
        target: e.target,
        color: "rgba(100, 150, 255, 0.15)",
        particles: 1,
      });
    }
  });

  return { nodes: viewNodes, links: viewLinks };
}

export default function ExplorePage() {
  const { data: graphData, isLoading } = useQuery({
    queryKey: ["graph"],
    queryFn: api.getGraph,
  });

  const [viewLevel, setViewLevel] = useState<ViewLevel>("architecture");
  const [filterPackage, setFilterPackage] = useState<string | undefined>();
  const [filterFile, setFilterFile] = useState<string | undefined>();
  const [selectedNode, setSelectedNode] = useState<GraphViewNode | null>(null);
  const fgRef = useRef<any>(null);

  const graphView = useMemo(() => {
    if (!graphData) return { nodes: [], links: [] };
    switch (viewLevel) {
      case "architecture":
        return buildArchitectureView(graphData.nodes, graphData.edges);
      case "modules":
        return buildModulesView(graphData.nodes, graphData.edges, filterPackage);
      case "functions":
        return buildFunctionsView(graphData.nodes, graphData.edges, filterFile);
    }
  }, [graphData, viewLevel, filterPackage, filterFile]);

  // Callers/callees for selected function
  const { callerNodes, calleeNodes } = useMemo(() => {
    if (!selectedNode?.data || !graphData) return { callerNodes: [], calleeNodes: [] };
    const nodeMap = new Map(graphData.nodes.map((n) => [n.id, n]));
    const callers: GraphNode[] = [];
    const callees: GraphNode[] = [];
    graphData.edges.forEach((e) => {
      if (e.type === "CALLS") {
        if (e.target === selectedNode.data!.id) {
          const n = nodeMap.get(e.source);
          if (n) callers.push(n);
        }
        if (e.source === selectedNode.data!.id) {
          const n = nodeMap.get(e.target);
          if (n) callees.push(n);
        }
      }
    });
    return { callerNodes: callers, calleeNodes: callees };
  }, [selectedNode, graphData]);

  const handleNodeClick = useCallback(
    (node: any) => {
      setSelectedNode(node);

      // Drill down on double-purpose click
      if (viewLevel === "architecture" && node.packageName) {
        setFilterPackage(node.packageName);
        setViewLevel("modules");
        setSelectedNode(null);
      } else if (viewLevel === "modules" && node.type === "file") {
        const fp = node.id.replace("file:", "");
        setFilterFile(fp);
        setViewLevel("functions");
        setSelectedNode(null);
      }

      // Camera focus
      if (fgRef.current) {
        const distance = 120;
        const distRatio = 1 + distance / Math.hypot(node.x, node.y, node.z);
        fgRef.current.cameraPosition(
          { x: node.x * distRatio, y: node.y * distRatio, z: node.z * distRatio },
          node,
          1000
        );
      }
    },
    [viewLevel]
  );

  const handleBack = () => {
    if (viewLevel === "functions") {
      setViewLevel("modules");
      setFilterFile(undefined);
      setSelectedNode(null);
    } else if (viewLevel === "modules") {
      setViewLevel("architecture");
      setFilterPackage(undefined);
      setSelectedNode(null);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground">
        Loading...
      </div>
    );
  }

  return (
    <div className="h-full relative bg-[#0a0a1a]">
      {/* 3D Graph */}
      <ForceGraph3D
        ref={fgRef}
        graphData={graphView}
        nodeLabel={(node: any) => {
          if (node.type === "package") return `${node.name} (${node.funcCount} functions, ${node.fileCount} files)`;
          if (node.type === "file") return `${node.name} (${node.funcCount} functions)`;
          return node.name;
        }}
        nodeColor={(node: any) => node.color}
        nodeVal={(node: any) => node.val}
        nodeOpacity={0.9}
        linkColor={(link: any) => link.color}
        linkDirectionalParticles={(link: any) => link.particles || 0}
        linkDirectionalParticleSpeed={0.005}
        linkDirectionalParticleColor={() => "#6b9fff"}
        linkDirectionalParticleWidth={1.5}
        linkWidth={0.5}
        linkOpacity={0.4}
        onNodeClick={handleNodeClick}
        backgroundColor="#0a0a1a"
        showNavInfo={false}
        enableNodeDrag={true}
      />

      {/* View level tabs */}
      <div className="absolute top-4 left-4 flex items-center gap-2 z-10">
        <div className="flex gap-1 bg-card/80 backdrop-blur border border-border p-1 rounded-lg">
          <button
            onClick={() => { setViewLevel("architecture"); setFilterPackage(undefined); setFilterFile(undefined); setSelectedNode(null); }}
            className={`px-3 py-1.5 rounded-md text-xs flex items-center gap-1.5 transition-colors ${
              viewLevel === "architecture" ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <Layers className="h-3 w-3" /> Architecture
          </button>
          <button
            onClick={() => { setViewLevel("modules"); setFilterFile(undefined); setSelectedNode(null); }}
            className={`px-3 py-1.5 rounded-md text-xs flex items-center gap-1.5 transition-colors ${
              viewLevel === "modules" ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <FolderTree className="h-3 w-3" /> Modules
          </button>
          <button
            onClick={() => { setViewLevel("functions"); setSelectedNode(null); }}
            className={`px-3 py-1.5 rounded-md text-xs flex items-center gap-1.5 transition-colors ${
              viewLevel === "functions" ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <Braces className="h-3 w-3" /> Functions
          </button>
        </div>

        {(viewLevel === "modules" || viewLevel === "functions") && (
          <button
            onClick={handleBack}
            className="px-3 py-1.5 bg-card/80 backdrop-blur border border-border rounded-lg text-xs text-muted-foreground hover:text-foreground"
          >
            ← Back
          </button>
        )}

        {filterPackage && (
          <span className="px-2 py-1 bg-primary/20 text-primary rounded text-xs">
            {filterPackage}
          </span>
        )}
        {filterFile && (
          <span className="px-2 py-1 bg-primary/20 text-primary rounded text-xs">
            {shortPath(filterFile)}
          </span>
        )}
      </div>

      {/* Stats */}
      <div className="absolute bottom-4 left-4 bg-card/80 backdrop-blur border border-border rounded-lg px-3 py-2 text-xs text-muted-foreground z-10">
        {graphView.nodes.length} nodes · {graphView.links.length} connections
        <span className="ml-2 text-[10px]">Click nodes to drill down</span>
      </div>

      {/* Detail panel — only for function level */}
      {selectedNode && selectedNode.data && viewLevel === "functions" && (
        <div className="absolute top-4 right-4 w-72 bg-card/90 backdrop-blur border border-border rounded-lg p-4 space-y-3 text-sm z-10 max-h-[80vh] overflow-y-auto">
          <div className="flex items-center justify-between">
            <h3 className="font-mono font-bold">{selectedNode.name}</h3>
            <button onClick={() => setSelectedNode(null)} className="text-muted-foreground hover:text-foreground">
              <X className="h-4 w-4" />
            </button>
          </div>

          <div className="space-y-1 text-xs text-muted-foreground">
            {selectedNode.data.package && <p>Package: {selectedNode.data.package}</p>}
            {selectedNode.data.start_line && <p className="font-mono">{shortPath(selectedNode.data.file_path)}:{selectedNode.data.start_line}</p>}
            {selectedNode.data.is_export && <p>Exported</p>}
          </div>

          <div className="grid grid-cols-2 gap-2">
            <div className="bg-background/50 rounded p-2 text-center">
              <p className="text-lg font-bold">{callerNodes.length}</p>
              <p className="text-[10px] text-muted-foreground">Callers</p>
            </div>
            <div className="bg-background/50 rounded p-2 text-center">
              <p className="text-lg font-bold">{calleeNodes.length}</p>
              <p className="text-[10px] text-muted-foreground">Calls</p>
            </div>
          </div>

          {callerNodes.length > 0 && (
            <div>
              <p className="text-[10px] text-muted-foreground uppercase font-medium flex items-center gap-1 mb-1">
                <ArrowDownLeft className="h-3 w-3" /> Called by
              </p>
              {callerNodes.slice(0, 8).map((c) => (
                <p key={c.id} className="text-xs font-mono py-0.5">{c.name}</p>
              ))}
              {callerNodes.length > 8 && <p className="text-[10px] text-muted-foreground">+{callerNodes.length - 8} more</p>}
            </div>
          )}

          {calleeNodes.length > 0 && (
            <div>
              <p className="text-[10px] text-muted-foreground uppercase font-medium flex items-center gap-1 mb-1">
                <ArrowUpRight className="h-3 w-3" /> Calls
              </p>
              {calleeNodes.slice(0, 8).map((c) => (
                <p key={c.id} className="text-xs font-mono py-0.5">{c.name}</p>
              ))}
              {calleeNodes.length > 8 && <p className="text-[10px] text-muted-foreground">+{calleeNodes.length - 8} more</p>}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
