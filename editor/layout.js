// Layout — pure transformation: (Model state, MeasureContext) → LayoutResult

export function createMeasureContext(font) {
  const canvas = document.createElement("canvas");
  const ctx = canvas.getContext("2d");
  ctx.font = fontString(font);
  return {
    ctx,
    setFont(f) {
      ctx.font = fontString(f);
    },
    measureText(text) {
      return ctx.measureText(text);
    },
  };
}

function fontString(font) {
  return `${font.style} ${font.weight} ${font.size}px ${font.family}`;
}

// Compute x positions for each character boundary in a string (0..text.length)
// Returns array of length text.length + 1
function computeXPositions(ctx, text) {
  const positions = new Float32Array(text.length + 1);
  positions[0] = 0;
  for (let i = 0; i < text.length; i++) {
    positions[i + 1] = ctx.measureText(text.slice(0, i + 1)).width;
  }
  return positions;
}

export function computeLayout(model, measureCtx, containerWidth) {
  const font = model.getFont();
  const opts = model.getOptions();
  const lines = model.getLines();

  measureCtx.setFont(font);

  const charHeight = font.size;
  const lineHeight = typeof font.lineHeight === "number" && font.lineHeight < 10
    ? font.size * font.lineHeight
    : (typeof font.lineHeight === "number" ? font.lineHeight : font.size * 1.5);

  const wrapWidth = opts.softWrap
    ? (opts.wrapWidth || containerWidth || Infinity)
    : Infinity;

  const visualLines = [];
  let y = 0;
  let maxX = 0;

  for (const { text, startOffset } of lines) {
    const xPositions = computeXPositions(measureCtx.ctx, text);
    const charBoxes = [];

    // Build char boxes for each character in the logical line
    for (let i = 0; i <= text.length; i++) {
      charBoxes.push({
        offset: startOffset + i,
        x: xPositions[i],
        width: i < text.length ? (xPositions[i + 1] - xPositions[i]) : 0,
      });
    }

    if (!opts.softWrap || wrapWidth === Infinity) {
      // No wrapping
      const lineMaxX = xPositions[text.length];
      if (lineMaxX > maxX) maxX = lineMaxX;
      visualLines.push({ y, height: lineHeight, charBoxes, startOffset, text });
      y += lineHeight;
    } else {
      // Soft wrap: split char boxes into visual sub-lines
      let segStart = 0;
      while (segStart <= text.length) {
        // Find how many chars fit
        let segEnd = segStart;
        const baseX = xPositions[segStart];
        while (segEnd < text.length && (xPositions[segEnd + 1] - baseX) <= wrapWidth) {
          segEnd++;
        }
        if (segEnd === segStart && segEnd < text.length) segEnd++; // at least one char

        // Build sub-line char boxes with adjusted x
        const subBoxes = [];
        for (let i = segStart; i <= segEnd; i++) {
          subBoxes.push({
            offset: startOffset + i,
            x: xPositions[i] - baseX,
            width: i < text.length ? (xPositions[i + 1] - xPositions[i]) : 0,
          });
        }

        const lineW = xPositions[segEnd] - baseX;
        if (lineW > maxX) maxX = lineW;

        visualLines.push({
          y,
          height: lineHeight,
          charBoxes: subBoxes,
          startOffset: startOffset + segStart,
          text: text.slice(segStart, segEnd),
        });
        y += lineHeight;

        if (segEnd === text.length) break;
        segStart = segEnd;
      }
    }
  }

  return {
    lines: visualLines,
    contentWidth: maxX,
    contentHeight: y,
    charHeight,
    lineHeight,
  };
}

export function offsetFromPoint(x, y, layout) {
  if (!layout || !layout.lines.length) return 0;

  // Find the visual line
  let bestLine = layout.lines[0];
  for (const line of layout.lines) {
    if (y >= line.y) bestLine = line;
    else break;
  }

  const boxes = bestLine.charBoxes;
  if (!boxes.length) return bestLine.startOffset;

  // Find nearest char boundary by checking each box's left edge
  // The last box in a visual line represents end-of-line (width 0, or the \n position)
  let best = boxes[0];
  let bestDist = Infinity;

  for (let i = 0; i < boxes.length; i++) {
    const box = boxes[i];
    const dist = Math.abs(x - box.x);
    if (dist < bestDist) {
      bestDist = dist;
      best = box;
    }
  }

  return best.offset;
}

export function getCursorBox(offset, layout) {
  if (!layout || !layout.lines.length) return { x: 0, y: 0, height: layout ? layout.lineHeight : 14 };

  for (const line of layout.lines) {
    const boxes = line.charBoxes;
    for (const box of boxes) {
      if (box.offset === offset) {
        return { x: box.x, y: line.y, height: line.height };
      }
    }
  }

  // Fallback: last position
  const last = layout.lines[layout.lines.length - 1];
  const lastBoxes = last.charBoxes;
  const lastBox = lastBoxes[lastBoxes.length - 1];
  return { x: lastBox ? lastBox.x : 0, y: last.y, height: last.height };
}
