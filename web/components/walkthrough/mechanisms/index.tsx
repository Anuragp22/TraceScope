"use client";

import type { ComponentType } from "react";
import type { Scenario } from "@/lib/walkthrough/data";
import { SystemDiagram } from "../system-diagram";
import { Lbl, Callout, Scale } from "../primitives";
import {
  MechArtifact,
  MechBind,
  MechConfig,
  MechFrontends,
  MechIncremental,
  MechResolve,
  MechRun,
  MechScip,
  MechVisualSurfaces,
  MechWalk,
} from "./figures-a";
import {
  MechDiff,
  MechEval,
  MechExit,
  MechMap,
  MechOwners,
  MechScore,
  MechSeeds,
  MechSurfaces,
  MechTraverse,
  MechWhy,
} from "./figures-b";
import {
  MechCycle,
  MechInvariants,
  MechPackages,
  MechQueries,
  MechSeams,
} from "./figures-c";

export type MechProps = { sc: Scenario; source?: string };

/**
 * The system diagram appears twice: alone on the landing page, and inside its
 * own stage with the surrounding argument. This is the stage version.
 */
function MechSystem() {
  return (
    <div>
      <Lbl>every part, and the only ways data moves between them</Lbl>
      <SystemDiagram />
      <Callout className="mt-3.5">
        Read it top to bottom. Your code and your diff go in at the top. One command writes{" "}
        <b>graph.json</b>, the other reads it. Nothing else on the page happens outside this picture.
      </Callout>
      <Callout className="mt-2.5">
        The one thing worth noticing: <b>graph.json is the only link between the two halves</b>. Index
        writes it and stops. Analyze reads it and never touches your source. That is why analysis is
        fast, and why a graph built at the wrong moment gives a wrong answer.
      </Callout>
      <Scale>
        The four other commands at the bottom left read the same file and add nothing to it, which is
        why they need no arguments beyond what you are asking about.
      </Scale>
    </div>
  );
}

/** Stage `mech` keys resolve here. A missing key renders a visible placeholder. */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const MECHANISMS: Record<string, ComponentType<any>> = {
  System: MechSystem,
  Run: MechRun,
  Artifact: MechArtifact,
  Packages: MechPackages,
  Seams: MechSeams,
  Cycle: MechCycle,
  Invariants: MechInvariants,

  Walk: MechWalk,
  Frontends: MechFrontends,
  Scip: MechScip,
  Resolve: MechResolve,
  Incremental: MechIncremental,
  Bind: MechBind,

  Diff: MechDiff,
  Map: MechMap,
  Seeds: MechSeeds,
  Traverse: MechTraverse,
  Score: MechScore,
  Why: MechWhy,
  Owners: MechOwners,
  Surfaces: MechSurfaces,
  Exit: MechExit,

  Queries: MechQueries,
  VisualSurfaces: MechVisualSurfaces,
  Config: MechConfig,
  Eval: MechEval,
};
