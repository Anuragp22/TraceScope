// Single vendored bundle for docs/tracescope-motion.html.
// Built with esbuild so the page runs with the network off — no CDN, no
// importmap, no build step at view time.
import React from "react";
import { createRoot } from "react-dom/client";
import { motion, AnimatePresence, useScroll, useSpring } from "framer-motion";
import htm from "htm";

export { React, createRoot, motion, AnimatePresence, useScroll, useSpring, htm };
export const { useState, useEffect, useMemo, useRef, Fragment } = React;
