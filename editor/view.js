// View — reads Layout output and paints characters, cursor, and selection.

import { computeLayout, offsetFromPoint, getCursorBox, createMeasureContext } from "./layout.js";

export function createView(containerEl, model) {
  const measureCtx = createMeasureContext(model.getFont());

  // Container setup
  containerEl.style.position = "relative";
  containerEl.style.overflow = "auto";
  containerEl.style.cursor = "text";

  // Canvas
  const canvas = document.createElement("canvas");
  canvas.style.display = "block";
  containerEl.appendChild(canvas);
  const ctx = canvas.getContext("2d");

  // Hidden textarea for input capture
  const textarea = document.createElement("textarea");
  textarea.setAttribute("autocomplete", "off");
  textarea.setAttribute("autocorrect", "off");
  textarea.setAttribute("autocapitalize", "off");
  textarea.setAttribute("spellcheck", "false");
  Object.assign(textarea.style, {
    position: "fixed",
    top: "0",
    left: "0",
    width: "1px",
    height: "1px",
    opacity: "0",
    padding: "0",
    margin: "0",
    border: "none",
    outline: "none",
    resize: "none",
    overflow: "hidden",
    zIndex: "-1",
    pointerEvents: "none",
  });
  containerEl.appendChild(textarea);

  // Colors
  const colors = {
    background: "#1e1e1e",
    text: "#d4d4d4",
    cursor: "#aeafad",
    selection: "rgba(38, 79, 120, 0.8)",
    lineHighlight: "rgba(255,255,255,0.04)",
  };

  let layout = null;
  let cursorVisible = true;
  let blinkTimer = null;

  function fontString(font) {
    return `${font.style} ${font.weight} ${font.size}px ${font.family}`;
  }

  function getContainerWidth() {
    return containerEl.clientWidth || 600;
  }

  function recomputeLayout() {
    layout = computeLayout(model, measureCtx, getContainerWidth());
  }

  function resizeCanvas() {
    const w = Math.max(getContainerWidth(), layout ? layout.contentWidth + 40 : 0);
    const h = Math.max(containerEl.clientHeight || 200, layout ? layout.contentHeight + 20 : 0);
    if (canvas.width !== w || canvas.height !== h) {
      canvas.width = w;
      canvas.height = h;
    }
  }

  function paint() {
    if (!layout) return;
    resizeCanvas();

    const font = model.getFont();
    const cursor = model.getCursor();
    const selection = model.getSelection();

    ctx.clearRect(0, 0, canvas.width, canvas.height);

    // Background
    ctx.fillStyle = colors.background;
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    // Current line highlight
    const cursorBox = getCursorBox(cursor, layout);
    ctx.fillStyle = colors.lineHighlight;
    ctx.fillRect(0, cursorBox.y, canvas.width, cursorBox.height);

    // Selection rectangles
    if (selection) {
      const [selStart, selEnd] = selection;
      ctx.fillStyle = colors.selection;

      for (const line of layout.lines) {
        const boxes = line.charBoxes;
        if (!boxes.length) continue;

        const lineStart = boxes[0].offset;
        const lineEnd = boxes[boxes.length - 1].offset;

        if (lineEnd < selStart || lineStart >= selEnd) continue;

        let x1 = null, x2 = null;
        for (const box of boxes) {
          if (box.offset >= selStart && box.offset < selEnd) {
            if (x1 === null) x1 = box.x;
            x2 = box.x + box.width;
          }
        }
        if (x1 !== null) {
          ctx.fillRect(x1, line.y, x2 - x1, line.height);
        }
      }
    }

    // Text
    ctx.font = fontString(font);
    ctx.fillStyle = colors.text;
    ctx.textBaseline = "middle";

    for (const line of layout.lines) {
      if (line.text.length > 0) {
        const midY = line.y + line.height / 2;
        ctx.fillText(line.text, line.charBoxes[0].x, midY);
      }
    }

    // Cursor
    if (cursorVisible) {
      ctx.fillStyle = colors.cursor;
      ctx.fillRect(cursorBox.x, cursorBox.y + 2, 2, cursorBox.height - 4);
    }
  }

  function resetBlink() {
    cursorVisible = true;
    if (blinkTimer) clearInterval(blinkTimer);
    blinkTimer = setInterval(() => {
      cursorVisible = !cursorVisible;
      paint();
    }, 530);
  }

  function refresh() {
    recomputeLayout();
    resetBlink();
    paint();
  }

  // Initial render
  refresh();

  // Subscribe to model changes
  const unsub = model.onChange(() => refresh());

  const view = {
    canvas,
    textarea,
    getLayout() { return layout; },
    getContainerWidth,
    paint,
    refresh,
    focus() {
      textarea.focus();
      resetBlink();
    },
    destroy() {
      unsub();
      if (blinkTimer) clearInterval(blinkTimer);
      canvas.remove();
      textarea.remove();
    },
    setColors(c) {
      Object.assign(colors, c);
      paint();
    },
  };

  return view;
}
