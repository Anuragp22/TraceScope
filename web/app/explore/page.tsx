"use client";

import { useQuery } from "@tanstack/react-query";
import { api, type GraphNode, type GraphEdge } from "@/lib/api";
import { useState, useCallback, useMemo } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  MarkerType,
  Panel,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

const NODE_COLORS: Record<string, string> = {
  function: "#3b82f6",
  file: "#6b7280",
  class: "#22c55e",
};

const EDGE_COLORS: Record<string, string> = {
  CALLS: "#3b82f680",
  CONTAINS: "#6b728040",
  IMPORTS: "#f59e0b60",
  EXTENDS: "#22c55e80",
  IMPLEMENTS: "#22c55e80",
};

function shortName(node: GraphNode) {
  if (node.type === "file") {
    const parts = node.file_path.replace(/\\/g, "/").split("/");
    return parts[parts.length - 1];
  }
  return node.name;
}

function buildFlowGraph(
  graphNodes: GraphNode[],
  graphEdges: GraphEdge[],
  filters: { functions: boolean; files: boolean; classes: boolean; edgeTypes: Set<string> },
  searchQuery: string,
  focusNodeId: string | null
) {
  // Filter nodes
  const visibleTypes = new Set<string>();
  if (filters.functions) visibleTypes.add("function");
  if (filters.files) visibleTypes.add("file");
  if (filters.classes) visibleTypes.add("class");

  let filteredNodes = graphNodes.filter((n) => visibleTypes.has(n.type));

  // If focusing on a node, only show it + direct connections
  let connectedIds: Set<string> | null = null;
  if (focusNodeId) {
    connectedIds = new Set([focusNodeId]);
    graphEdges.forEach((e) => {
      if (e.source === focusNodeId) connectedIds!.add(e.target);
      if (e.target === focusNodeId) connectedIds!.add(e.source);
    });
    filteredNodes = filteredNodes.filter((n) => connectedIds!.has(n.id));
  }

  // Search filter
  if (searchQuery) {
    const q = searchQuery.toLowerCase();
    filteredNodes = filteredNodes.filter(
      (n) => n.name.toLowerCase().includes(q) || (connectedIds && connectedIds.has(n.id))
    );
  }

  const nodeIds = new Set(filteredNodes.map((n) => n.id));

  // Build inbound count for sizing
  const inbound: Record<string, number> = {};
  graphEdges.forEach((e) => {
    if (e.type === "CALLS") {
      inbound[e.target] = (inbound[e.target] || 0) + 1;
    }
  });

  // Layout: simple grid with some randomness
  const cols = Math.max(1, Math.ceil(Math.sqrt(filteredNodes.length)));

  const nodes: Node[] = filteredNodes.map((n, i) => {
    const callers = inbound[n.id] || 0;
    const size = Math.max(40, Math.min(120, 40 + callers * 5));

    return {
      id: n.id,
      type: "default",
      position: {
        x: (i % cols) * 200 + Math.random() * 40,
        y: Math.floor(i / cols) * 100 + Math.random() * 40,
      },
      data: {
        label: shortName(n),
        ...n,
      },
      style: {
        background: NODE_COLORS[n.type] || "#6b7280",
        color: "#fff",
        border: "none",
        borderRadius: "6px",
        fontSize: "11px",
        padding: "4px 8px",
        width: size,
        textAlign: "center" as const,
      },
    };
  });

  const edges: Edge[] = graphEdges
    .filter(
      (e) =>
        nodeIds.has(e.source) &&
        nodeIds.has(e.target) &&
        filters.edgeTypes.has(e.type)
    )
    .map((e, i) => ({
      id: `e-${i}`,
      source: e.source,
      target: e.target,
      animated: e.type === "CALLS",
      style: { stroke: EDGE_COLORS[e.type] || "#6b728040", strokeWidth: 1 },
      markerEnd: e.type === "CALLS" ? { type: MarkerType.ArrowClosed, width: 10, height: 10 } : undefined,
      label: undefined,
    }));

  return { nodes, edges };
}

export default function ExplorePage() {
  const { data: graphData, isLoading } = useQuery({
    queryKey: ["graph"],
    queryFn: api.getGraph,
  });

  const [search, setSearch] = useState("");
  const [focusNode, setFocusNode] = useState<string | null>(null);
  const [selectedNode, setSelectedNode] = useState<GraphNode | null>(null);
  const [filters, setFilters] = useState({
    functions: true,
    files: false,
    classes: true,
    edgeTypes: new Set(["CALLS", "EXTENDS", "IMPLEMENTS"]),
  });

  const { nodes: flowNodes, edges: flowEdges } = useMemo(() => {
    if (!graphData) return { nodes: [], edges: [] };
    return buildFlowGraph(graphData.nodes, graphData.edges, filters, search, focusNode);
  }, [graphData, filters, search, focusNode]);

  const [nodes, setNodes, onNodesChange] = useNodesState(flowNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(flowEdges);

  // Update nodes/edges when flow data changes
  useMemo(() => {
    setNodes(flowNodes);
    setEdges(flowEdges);
  }, [flowNodes, flowEdges, setNodes, setEdges]);

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      if (!graphData) return;
      const graphNode = graphData.nodes.find((n) => n.id === node.id);
      if (graphNode) setSelectedNode(graphNode);
    },
    [graphData]
  );

  const toggleEdgeType = (type: string) => {
    setFilters((prev) => {
      const next = new Set(prev.edgeTypes);
      if (next.has(type)) next.delete(type);
      else next.add(type);
      return { ...prev, edgeTypes: next };
    });
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground">
        Loading graph...
      </div>
    );
  }

  // Get callers/callees for selected node
  const callers: GraphNode[] = [];
  const callees: GraphNode[] = [];
  if (selectedNode && graphData) {
    const nodeMap = new Map(graphData.nodes.map((n) => [n.id, n]));
    graphData.edges.forEach((e) => {
      if (e.type === "CALLS") {
        if (e.target === selectedNode.id) {
          const n = nodeMap.get(e.source);
          if (n) callers.push(n);
        }
        if (e.source === selectedNode.id) {
          const n = nodeMap.get(e.target);
          if (n) callees.push(n);
        }
      }
    });
  }

  return (
    <div className="h-full relative">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={onNodeClick}
        fitView
        minZoom={0.1}
        maxZoom={4}
        proOptions={{ hideAttribution: true }}
      >
        <Background />
        <Controls />
        <MiniMap
          nodeColor={(n) => NODE_COLORS[n.data?.type as string] || "#6b7280"}
          style={{ background: "#1a1a2e" }}
        />

        {/* Filters panel */}
        <Panel position="top-left">
          <div className="bg-card border border-border rounded-lg p-3 space-y-3 text-sm w-56">
            <input
              type="text"
              placeholder="Search functions..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full bg-background border border-border rounded-md px-2 py-1 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
            />
            <div className="space-y-1">
              <p className="text-muted-foreground text-xs font-medium uppercase">Nodes</p>
              {(["functions", "files", "classes"] as const).map((key) => (
                <label key={key} className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={filters[key]}
                    onChange={() => setFilters((p) => ({ ...p, [key]: !p[key] }))}
                    className="accent-primary"
                  />
                  <span className="capitalize">{key}</span>
                </label>
              ))}
            </div>
            <div className="space-y-1">
              <p className="text-muted-foreground text-xs font-medium uppercase">Edges</p>
              {["CALLS", "CONTAINS", "IMPORTS", "EXTENDS"].map((type) => (
                <label key={type} className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={filters.edgeTypes.has(type)}
                    onChange={() => toggleEdgeType(type)}
                    className="accent-primary"
                  />
                  <span>{type}</span>
                </label>
              ))}
            </div>
            {focusNode && (
              <button
                onClick={() => setFocusNode(null)}
                className="w-full text-xs text-primary hover:underline"
              >
                Clear focus (show all)
              </button>
            )}
            <p className="text-xs text-muted-foreground">
              {nodes.length} nodes, {edges.length} edges
            </p>
          </div>
        </Panel>
      </ReactFlow>

      {/* Detail panel */}
      {selectedNode && (
        <div className="absolute top-4 right-4 w-72 bg-card border border-border rounded-lg p-4 space-y-3 text-sm z-10 max-h-[80vh] overflow-y-auto">
          <div className="flex items-center justify-between">
            <h3 className="font-mono font-bold text-base">{selectedNode.name}</h3>
            <button
              onClick={() => setSelectedNode(null)}
              className="text-muted-foreground hover:text-foreground"
            >
              &times;
            </button>
          </div>
          <div className="space-y-1 text-muted-foreground text-xs">
            <p>Type: {selectedNode.type}</p>
            {selectedNode.package && <p>Package: {selectedNode.package}</p>}
            {selectedNode.language && <p>Language: {selectedNode.language}</p>}
            {selectedNode.is_export && <p>Exported: yes</p>}
            {selectedNode.start_line && (
              <p className="font-mono">{shortName(selectedNode)}:{selectedNode.start_line}</p>
            )}
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => setFocusNode(selectedNode.id)}
              className="px-2 py-1 bg-primary text-primary-foreground rounded text-xs"
            >
              Focus
            </button>
          </div>
          {callers.length > 0 && (
            <div>
              <p className="text-xs text-muted-foreground font-medium uppercase mb-1">
                Callers ({callers.length})
              </p>
              {callers.slice(0, 10).map((c) => (
                <p
                  key={c.id}
                  className="text-xs font-mono cursor-pointer hover:text-primary"
                  onClick={() => {
                    setSelectedNode(c);
                    setFocusNode(c.id);
                  }}
                >
                  {c.name}
                </p>
              ))}
              {callers.length > 10 && (
                <p className="text-xs text-muted-foreground">+{callers.length - 10} more</p>
              )}
            </div>
          )}
          {callees.length > 0 && (
            <div>
              <p className="text-xs text-muted-foreground font-medium uppercase mb-1">
                Calls ({callees.length})
              </p>
              {callees.slice(0, 10).map((c) => (
                <p
                  key={c.id}
                  className="text-xs font-mono cursor-pointer hover:text-primary"
                  onClick={() => {
                    setSelectedNode(c);
                    setFocusNode(c.id);
                  }}
                >
                  {c.name}
                </p>
              ))}
              {callees.length > 10 && (
                <p className="text-xs text-muted-foreground">+{callees.length - 10} more</p>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
