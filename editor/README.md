# Minimal Text Editor

A minimal JavaScript text editor component that renders to `<canvas>` and uses a vendored Git subtree copy of Pretext for grapheme-safe text layout, caret geometry, and hit-testing. No `contentEditable`, no frameworks.

## Quick Start

Build the vendored Pretext copy first, then serve this folder over HTTP:

```sh
cd /Users/jak/src/vibes/editor
cd vendor/pretext
bun install
bun run build:package

cd /Users/jak/src/vibes/editor

python3 -m http.server 8080
# open http://localhost:8080/index.html
```

The editor imports Pretext from `./vendor/pretext/dist/layout.js`.

## Vendoring Strategy

`vendor/pretext` is a Git subtree of the upstream Pretext repo. The actual Git root is:

```sh
/Users/jak/src/vibes
```

To update the vendored copy from GitHub:

```sh
cd /Users/jak/src/vibes
git subtree pull --prefix=editor/vendor/pretext pretext-upstream main --squash
```

If you need to export local subtree changes back out as a branch:

```sh
cd /Users/jak/src/vibes
git subtree split --prefix=editor/vendor/pretext -b pretext-export
```

## Building Vendored Pretext

From inside `editor/`, the simplest build loop is:

```sh
cd /Users/jak/src/vibes/editor/vendor/pretext
bun install
bun run build:package
```

Then go back to the editor root and serve it:

```sh
cd /Users/jak/src/vibes/editor
python3 -m http.server 8080
```

If you update the subtree first, rebuild again afterward so `vendor/pretext/dist/` matches the vendored source.

## Longer-Term Workflow

For this setup, the most sustainable workflow is:

1. Keep the full source as a subtree in `vendor/pretext/`.
2. Build the vendored copy locally into `vendor/pretext/dist/`.
3. Treat subtree pulls and local Pretext edits as normal Git changes inside the `vibes` repo.
4. If this becomes a lasting setup, add a small script in `editor/` that runs:
   - subtree pull
   - `cd vendor/pretext && bun run build:package`

That gives you a vendored source tree, a local build artifact for the editor to import, and a clean path to sync from upstream GitHub.

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
