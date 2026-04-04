import {
  layoutWithLines,
  measureLineCarets,
} from "./vendor/pretext/layout.js";

function findLineIndex(y, layout) {
  if (!layout || !layout.lines.length) return -1;
  let bestIndex = 0;
  for (let i = 0; i < layout.lines.length; i++) {
    if (y >= layout.lines[i].y) bestIndex = i;
    else break;
  }
  return bestIndex;
}

function nearestBoundaryIndex(line, x) {
  const positions = line.caretX;
  let lo = 0;
  let hi = positions.length - 1;

  while (lo < hi) {
    const mid = Math.floor((lo + hi) / 2);
    if (positions[mid] < x) lo = mid + 1;
    else hi = mid;
  }

  if (lo === 0) return 0;
  const prev = positions[lo - 1];
  const curr = positions[lo];
  return Math.abs(x - prev) <= Math.abs(curr - x) ? lo - 1 : lo;
}

export function fontString(font) {
  return `${font.style} ${font.weight} ${font.size}px ${font.family}`;
}

export function computeLayout(model, containerWidth) {
  const font = model.getFont();
  const opts = model.getOptions();
  const prepared = model.getPrepared();
  const lineHeight = typeof font.lineHeight === "number" && font.lineHeight < 10
    ? font.size * font.lineHeight
    : (typeof font.lineHeight === "number" ? font.lineHeight : font.size * 1.5);
  const wrapWidth = opts.softWrap
    ? (opts.wrapWidth || containerWidth || Infinity)
    : Infinity;
  const result = layoutWithLines(prepared, wrapWidth, lineHeight);

  const lines = [];
  let y = 0;
  let maxX = 0;
  for (const line of result.lines) {
    const geometry = measureLineCarets(prepared, line);
    if (line.width > maxX) maxX = line.width;
    lines.push({
      text: line.text,
      y,
      height: lineHeight,
      width: line.width,
      start: line.start,
      end: line.end,
      caretX: geometry.x,
      caretOffsets: geometry.offsets,
      contentEndOffset: geometry.contentEndOffset,
      endOffset: geometry.endOffset,
      endsWithHardBreak: geometry.endsWithHardBreak,
    });
    y += lineHeight;
  }

  return {
    lines,
    contentWidth: maxX,
    contentHeight: result.height,
    charHeight: font.size,
    lineHeight,
  };
}

export function offsetFromPoint(x, y, layout) {
  if (!layout || !layout.lines.length) return 0;
  const lineIndex = findLineIndex(y, layout);
  const line = layout.lines[Math.max(0, lineIndex)];
  const boundaryIndex = nearestBoundaryIndex(line, x);
  return line.caretOffsets[boundaryIndex];
}

export function findVisualLineForOffset(offset, layout) {
  if (!layout || !layout.lines.length) return null;

  for (const line of layout.lines) {
    if (offset < line.caretOffsets[0]) break;
    if (offset <= line.contentEndOffset) return line;
    if (line.endsWithHardBreak && offset === line.endOffset) return line;
  }

  return layout.lines[layout.lines.length - 1];
}

export function getCursorBox(offset, layout) {
  if (!layout || !layout.lines.length) {
    return { x: 0, y: 0, height: layout ? layout.lineHeight : 14 };
  }

  const line = findVisualLineForOffset(offset, layout);
  if (!line) return { x: 0, y: 0, height: layout.lineHeight };

  let boundaryIndex = line.caretOffsets.indexOf(offset);
  if (boundaryIndex < 0 && line.endsWithHardBreak && offset === line.endOffset) {
    boundaryIndex = line.caretOffsets.length - 1;
  }
  if (boundaryIndex < 0) boundaryIndex = 0;

  return { x: line.caretX[boundaryIndex], y: line.y, height: line.height, line };
}
