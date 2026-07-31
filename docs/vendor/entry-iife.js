// Same surface as entry.js, but exposed on window for a classic <script>,
// so the page works from file:// with no server and no module loader.
import React from "react";
import { createRoot } from "react-dom/client";
import { motion, AnimatePresence, useScroll, useSpring } from "framer-motion";
import htm from "htm";
window.__TS_DEPS__ = {
  React, createRoot, motion, AnimatePresence, useScroll, useSpring, htm,
  useState: React.useState, useEffect: React.useEffect,
  useMemo: React.useMemo, useRef: React.useRef
};
