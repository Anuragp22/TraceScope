import type { Metadata } from "next";
import { Walkthrough } from "@/components/walkthrough/walkthrough";

export const metadata: Metadata = {
  title: "How TraceScope works",
  description:
    "An interactive walkthrough of TraceScope: the architecture, how the call graph is built, how a pull request is judged, and what the evaluation says about whether the ranking works.",
};

export default function HowItWorksPage() {
  return <Walkthrough />;
}
