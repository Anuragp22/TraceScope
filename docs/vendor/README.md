# vendor/

`deps.js` is React 18.3.1, React DOM 18.3.1, Framer Motion 12.23.12 and htm 3.1.1,
bundled into a single ES module by esbuild.

It exists so `docs/tracescope-motion.html` runs with **no network at all** — no CDN,
no importmap, no install step. That matters because the page is shown live, and a
walkthrough that needs working wifi is a walkthrough that fails in the one room where
it counts.

## Rebuilding

```
npm install react@18.3.1 react-dom@18.3.1 framer-motion@12.23.12 \
            motion-dom@12.23.12 motion-utils@12.23.6 htm@3.1.1 esbuild
npx esbuild entry.js --bundle --format=esm --minify \
    --define:process.env.NODE_ENV='"production"' --outfile=deps.js
```

`motion-dom` and `motion-utils` are pinned deliberately. Framer Motion 12.23.12 imports
`activeAnimations` from `motion-dom`, which later 12.x releases no longer export — leaving
the range unpinned resolves to a newer one and the bundle fails to build.

`entry.js` is just the re-export surface:

```js
import React from "react";
import { createRoot } from "react-dom/client";
import { motion, AnimatePresence, useScroll, useSpring } from "framer-motion";
import htm from "htm";
export { React, createRoot, motion, AnimatePresence, useScroll, useSpring, htm };
export const { useState, useEffect, useMemo, useRef, Fragment } = React;
```
