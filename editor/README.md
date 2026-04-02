# Minimal Text Editor

A zero-dependency JavaScript text editor component. Renders to `<canvas>`, driven by a plain data model. No `contentEditable`, no frameworks, no build step.

## Quick Start

Serve the folder over HTTP (ES modules require a server) and open `test.html`:

```sh
python3 -m http.server 8080
# open http://localhost:8080/test.html
```

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
| `model.js` | Text buffer, cursor, selection, font config. The only source of truth. |
| `layout.js` | Pure function: model state → positioned character boxes. Handles soft-wrap and hit-testing. |
| `view.js` | Reads layout output, paints to `<canvas>`. Owns the blink timer and the hidden `<textarea>` used for input capture. |
| `input.js` | Attaches DOM listeners, translates events into `model.edit()` / `model.setCursor()` calls. No state beyond drag tracking. |
| `editor.js` | Wires the four modules together. Exposes the public API. Wraps `model.edit()` to implement the undo stack. |

## Data Model

**Buffer** — a flat string. Positions are integer offsets (0 = before first char).

**Selection** — an `anchor` (fixed end) and a `cursor` (active end). When equal, no selection exists. The selected range is always `[min(anchor, cursor), max(anchor, cursor))`.

**Layout** — each logical line produces a `VisualLine` with a `charBoxes` array. Each `CharBox` holds `{ offset, x, width }`. The last box on every visual line is a zero-width sentinel pointing to the `\n` (or end-of-buffer) position — used for cursor placement and newline selection rendering.

## Public API

```js
// Mutation
editor.edit([start, end], replacement)   // replace half-open range
editor.setCursor(offset, extend?)        // move cursor; extend=true keeps anchor

// Read
editor.getText()
editor.setText(text)
editor.getCursor()          // → number
editor.getSelection()       // → [start, end] | null
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
| Ctrl/Cmd+Arrows | Jump to line/doc start/end |
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
