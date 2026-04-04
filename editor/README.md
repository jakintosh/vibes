# Minimal Text Editor

A minimal JavaScript text editor component that renders to `<canvas>` and uses a vendored copy of Pretext for grapheme-safe text layout, caret geometry, and hit-testing. No `contentEditable`, no frameworks.

## Quick Start

Refresh the vendored Pretext build first, then serve this folder over HTTP:

```sh
cd /Users/jak/src/pretext
bun run build:package

cd /Users/jak/src/vibes/editor
rm -rf vendor/pretext
cp -R ../pretext/dist vendor/pretext

python3 -m http.server 8080
# open http://localhost:8080/index.html
```

The editor imports Pretext from `./vendor/pretext/layout.js`.

## Vendoring Strategy

For now, the practical vendoring loop is:

```sh
cd /Users/jak/src/pretext
bun run build:package

cd /Users/jak/src/vibes/editor
rm -rf vendor/pretext
cp -R ../pretext/dist vendor/pretext
```

The best permanent setup is usually one of these:

1. Add a tiny sync script in the editor repo, for example `scripts/vendor-pretext.sh`, that rebuilds Pretext and recopies `dist/`.
2. Pin a specific upstream commit in a small `vendor/pretext/VERSION` or `vendor/pretext/UPSTREAM` text file so updates are traceable.
3. If these repos will keep evolving together, move to a workspace/monorepo or subtree-based flow so vendoring is reproducible instead of manual.

For this editor specifically, I’d recommend option 1 plus a small version file:
- keep vendoring `dist/` only
- add a one-command sync script
- record the upstream Pretext commit hash beside the vendored files

That keeps the runtime dependency tiny while still making updates deliberate and reviewable.

To embed in your own page:

```js
import { createEditor } from "./editor.js";

const editor = createEditor(document.getElementById("my-container"), {
  text: "Hello, world!",
  font: { family: "monospace", size: 14, lineHeight: 1.6 },
  tabSize: 2,
  softWrap: false,
});

editor.enableCommand("copy");
editor.enableCommand("cut");
editor.enableCommand("paste");
editor.enableCommand("undo");
editor.enableCommand("redo");
editor.enableCommand("selectAll");

editor.focus();
editor.onChange((text) => console.log("changed:", text.length, "chars"));
```

The container must have an explicit height set in CSS; the canvas fills it and scrolls when content overflows.

## Architecture

```
Input (keyboard/mouse) → API calls → Model → Layout → View (canvas)
```

Four modules, each with a single responsibility:

| File | Role |
|------|------|
| `model.js` | Text buffer, grapheme cursor/selection state, prepared Pretext handle, and offset interop helpers. |
| `layout.js` | Pure function: model state → visual lines with caret geometry from Pretext. |
| `view.js` | Reads layout output, paints to `<canvas>`. Owns the blink timer and the hidden `<textarea>` used for input capture. |
| `input.js` | Attaches DOM listeners, translates events into `model.edit()` / `model.setCursor()` calls. No state beyond drag tracking. |
| `editor.js` | Wires the four modules together. Exposes the public API. Wraps `model.edit()` to implement the undo stack. |

## Data Model

**Buffer** — a flat string.

**Cursor/Selection** — the primary cursor and anchor are Pretext `LayoutCursor` objects, so movement stays aligned to grapheme boundaries. Legacy offset helpers are still exposed for commands and integration code that want JS string offsets.

**Layout** — the editor prepares the whole buffer with Pretext `whiteSpace: 'pre-wrap'`, lays it out into visual lines, and stores caret x positions plus aligned source offsets for every visible grapheme boundary.

## Public API

```js
// Mutation
editor.edit([start, end], replacement)   // replace half-open range
editor.setCursor(cursor, extend?)        // move to a LayoutCursor; extend=true keeps anchor
editor.setCursorOffset(offset, extend?)  // legacy offset interop

// Read
editor.getText()
editor.setText(text)
editor.getCursor()          // → { segmentIndex, graphemeIndex }
editor.getCursorOffset()    // → number
editor.getSelection()       // → [startCursor, endCursor] | null
editor.getSelectionOffsets() // → [startOffset, endOffset] | null
editor.getSelectedText()    // → string

// Commands (all disabled by default; enable explicitly)
editor.enableCommand(name)               // "copy"|"cut"|"paste"|"undo"|"redo"|"selectAll"
editor.disableCommand(name)
editor.setCommand(name, fn)              // override with custom implementation
editor.execCommand(name)

// Config
editor.setFont({ family, size, weight, style, lineHeight })
editor.setOptions({ readOnly, tabSize, softWrap, wrapWidth })

// Lifecycle
const unsub = editor.onChange(fn)        // returns unsubscribe
const unsub = editor.onSelectionChange(fn)
editor.focus()
editor.destroy()
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Arrows | Move cursor |
| Shift+Arrows | Extend selection |
| Ctrl/Cmd+Arrows | Jump to visual line start/end |
| Alt+Left/Right | Word-left / word-right |
| Home / End | Line start / end |
| Backspace / Delete | Delete char or selection |
| Enter | Insert newline |
| Tab | Insert spaces (`tabSize`) |
| Ctrl/Cmd+A | Select all |
| Ctrl/Cmd+C/X/V | Copy / cut / paste (if enabled) |
| Ctrl/Cmd+Z / Shift+Z | Undo / redo (if enabled) |

## HiDPI

The canvas is sized at `devicePixelRatio × CSS dimensions` and the context is scaled with `setTransform(dpr, 0, 0, dpr, 0, 0)` on every frame, so rendering is sharp on retina displays.

## Undo

Enabled by calling `editor.enableCommand("undo")` (also enables redo). Implemented as a wrapper around `model.edit()` in `editor.js` — the model itself has no knowledge of history. Each `edit()` call is a separate undo step; no coalescing in this version.
