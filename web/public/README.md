# web/public

`how-it-works.html` is the interactive walkthrough, served statically by Next.js at
`/how-it-works.html` and therefore live on the Vercel deployment. It is linked from the
repository README.

It is a build output, not a file to edit by hand. The source is
`docs/tracescope-motion.html`, which loads its libraries from `docs/vendor/deps.js`. The
standalone version inlines that bundle as a plain script so the page works from a URL, from
`file://`, and with no network at all.

## Rebuilding after editing the source

```
node - <<'JS'
const fs = require('fs');
const SRC = 'docs/tracescope-motion.html';
const OUT = 'web/public/how-it-works.html';
let s = fs.readFileSync(SRC, 'utf8');
const bundle = fs.readFileSync('docs/vendor/deps-iife.js', 'utf8');
s = s.replace(/import \{[\s\S]*?\} from "\.\/vendor\/deps\.js";/,
  () => 'const {\n  React, useState, useMemo, useEffect,\n  createRoot, motion, AnimatePresence, useScroll, useSpring, htm\n} = window.__TS_DEPS__;');
s = s.split('<script type="module">').join('<script>');
const anchor = '<script>\n// React, React DOM, Framer Motion';
s = s.replace(anchor, () => '<script>\n' + bundle + '\n</script>\n\n' + anchor);
fs.writeFileSync(OUT, s);
JS
```

Both `.replace` calls pass a function rather than a string on purpose. React's minified code
contains `"$&/"`, and `$&` inside a replacement *string* means "the text that matched", which
silently splices the match into the middle of React and produces a syntax error a few thousand
bytes in.
