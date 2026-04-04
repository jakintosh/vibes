import { computeLayout, getCursorBox, fontString } from "./layout.js";

export function createView(containerEl, model) {
  containerEl.style.position = "relative";
  containerEl.style.overflow = "auto";
  containerEl.style.cursor = "text";

  const canvas = document.createElement("canvas");
  canvas.style.display = "block";
  containerEl.appendChild(canvas);
  const ctx = canvas.getContext("2d");

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

  function getContainerWidth() {
    return containerEl.getBoundingClientRect().width || 600;
  }

  function recomputeLayout() {
    layout = computeLayout(model, getContainerWidth());
  }

  function resizeCanvas() {
    const dpr = window.devicePixelRatio || 1;
    const cssW = Math.max(getContainerWidth(), layout ? layout.contentWidth + 40 : 0);
    const cssH = Math.max(containerEl.clientHeight || 200, layout ? layout.contentHeight + 20 : 0);
    const physW = Math.round(cssW * dpr);
    const physH = Math.round(cssH * dpr);
    if (canvas.width !== physW || canvas.height !== physH) {
      canvas.width = physW;
      canvas.height = physH;
      canvas.style.width = cssW + "px";
      canvas.style.height = cssH + "px";
    }
  }

  function paint() {
    if (!layout) return;
    resizeCanvas();

    const dpr = window.devicePixelRatio || 1;
    const font = model.getFont();
    const cursorOffset = model.getCursorOffset();
    const selection = model.getSelectionOffsets();

    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, canvas.width, canvas.height);

    ctx.fillStyle = colors.background;
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    const cursorBox = getCursorBox(cursorOffset, layout);
    ctx.fillStyle = colors.lineHighlight;
    ctx.fillRect(0, cursorBox.y, canvas.width, cursorBox.height);

    if (selection) {
      const [selStart, selEnd] = selection;
      ctx.fillStyle = colors.selection;

      for (const line of layout.lines) {
        if (selEnd < line.caretOffsets[0] || selStart > line.endOffset) continue;

        let x1 = null;
        let x2 = null;
        for (let i = 0; i < line.caretOffsets.length; i++) {
          const offset = line.caretOffsets[i];
          if (offset < selStart || offset > selEnd) continue;
          if (x1 === null) x1 = line.caretX[i];
          x2 = line.caretX[i];
        }

        const includesHardBreak = line.endsWithHardBreak && selStart <= line.endOffset && selEnd >= line.endOffset;
        if (x1 !== null) {
          const nubWidth = layout.charHeight * 0.5;
          const selWidth = (x2 - x1) + (includesHardBreak ? nubWidth : 0);
          ctx.fillRect(x1, line.y, selWidth, line.height);
        }
      }
    }

    ctx.font = fontString(font);
    ctx.fillStyle = colors.text;
    ctx.textBaseline = "middle";

    for (const line of layout.lines) {
      if (line.text.length > 0) {
        const midY = line.y + line.height / 2;
        ctx.fillText(line.text, line.caretX[0], midY);
      }
    }

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

  refresh();

  const unsub = model.onChange(() => refresh());
  const resizeObserver = new ResizeObserver(() => refresh());
  resizeObserver.observe(containerEl);

  return {
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
      resizeObserver.disconnect();
      if (blinkTimer) clearInterval(blinkTimer);
      canvas.remove();
      textarea.remove();
    },
    setColors(c) {
      Object.assign(colors, c);
      paint();
    },
  };
}
