import {
  cursorToOffset,
  offsetToCursor,
  prepareWithSegments,
} from "./vendor/pretext/dist/layout.js";

function cloneCursor(cursor) {
  return { segmentIndex: cursor.segmentIndex, graphemeIndex: cursor.graphemeIndex };
}

function compareCursors(a, b) {
  if (a.segmentIndex !== b.segmentIndex) return a.segmentIndex - b.segmentIndex;
  return a.graphemeIndex - b.graphemeIndex;
}

function sortCursorRange(a, b) {
  return compareCursors(a, b) <= 0 ? [a, b] : [b, a];
}

function fontString(font) {
  return `${font.style} ${font.weight} ${font.size}px ${font.family}`;
}

export function createModel(initialText = "", fontConfig = {}, options = {}) {
  let buffer = initialText;

  const font = {
    family: "monospace",
    size: 14,
    weight: "normal",
    style: "normal",
    lineHeight: 1.5,
    ...fontConfig,
  };

  const opts = {
    readOnly: false,
    tabSize: 2,
    softWrap: false,
    wrapWidth: null,
    ...options,
  };

  let prepared = prepareWithSegments(buffer, fontString(font), { whiteSpace: "pre-wrap" });
  let cursor = { segmentIndex: 0, graphemeIndex: 0 };
  let anchor = { segmentIndex: 0, graphemeIndex: 0 };

  const changeListeners = [];
  const selectionListeners = [];

  function notify() {
    for (const fn of changeListeners) fn(buffer);
  }

  function notifySelection() {
    for (const fn of selectionListeners) fn();
  }

  function reprepare() {
    prepared = prepareWithSegments(buffer, fontString(font), { whiteSpace: "pre-wrap" });
  }

  function getCursorOffsetValue(value) {
    return cursorToOffset(prepared, value);
  }

  function getSelectionRange() {
    if (compareCursors(anchor, cursor) === 0) return null;
    const [start, end] = sortCursorRange(anchor, cursor);
    return [cloneCursor(start), cloneCursor(end)];
  }

  const model = {
    edit(range, replacement) {
      if (opts.readOnly) return;
      const start = Math.max(0, Math.min(range[0], buffer.length));
      const end = Math.max(0, Math.min(range[1], buffer.length));
      const [clampedStart, clampedEnd] = start <= end ? [start, end] : [end, start];

      buffer = buffer.slice(0, clampedStart) + replacement + buffer.slice(clampedEnd);
      reprepare();

      const nextOffset = clampedStart + replacement.length;
      cursor = offsetToCursor(prepared, nextOffset, "forward");
      anchor = cloneCursor(cursor);
      notifySelection();
      notify();
    },

    setCursor(nextCursor, extend = false) {
      cursor = cloneCursor(nextCursor);
      if (!extend) anchor = cloneCursor(nextCursor);
      notifySelection();
      notify();
    },

    setCursorOffset(offset, extend = false, affinity = "forward") {
      model.setCursor(offsetToCursor(prepared, offset, affinity), extend);
    },

    getText() { return buffer; },
    setText(text) {
      const cursorOffset = model.getCursorOffset();
      const anchorOffset = model.getAnchorOffset();
      buffer = text;
      reprepare();
      cursor = offsetToCursor(prepared, Math.min(cursorOffset, buffer.length), "forward");
      anchor = offsetToCursor(prepared, Math.min(anchorOffset, buffer.length), "forward");
      notifySelection();
      notify();
    },

    getPrepared() { return prepared; },
    getCursor() { return cloneCursor(cursor); },
    getAnchor() { return cloneCursor(anchor); },
    getCursorOffset() { return getCursorOffsetValue(cursor); },
    getAnchorOffset() { return getCursorOffsetValue(anchor); },

    getSelection() {
      return getSelectionRange();
    },

    getSelectionOffsets() {
      const selection = getSelectionRange();
      if (!selection) return null;
      return [
        getCursorOffsetValue(selection[0]),
        getCursorOffsetValue(selection[1]),
      ];
    },

    getSelectedText() {
      const selection = model.getSelectionOffsets();
      if (!selection) return "";
      return buffer.slice(selection[0], selection[1]);
    },

    offsetToCursor(offset, affinity = "forward") {
      return offsetToCursor(prepared, offset, affinity);
    },

    cursorToOffset(cursorValue) {
      return getCursorOffsetValue(cursorValue);
    },

    compareCursors,

    positionToLineCol(offset) {
      const clamped = Math.max(0, Math.min(offset, buffer.length));
      const before = buffer.slice(0, clamped);
      const lines = before.split("\n");
      return { line: lines.length - 1, col: lines[lines.length - 1].length };
    },

    lineColToPosition(line, col) {
      const lines = buffer.split("\n");
      let offset = 0;
      for (let i = 0; i < Math.min(line, lines.length - 1); i++) {
        offset += lines[i].length + 1;
      }
      const lineText = lines[Math.min(line, lines.length - 1)] || "";
      return offset + Math.min(col, lineText.length);
    },

    getFont() { return { ...font }; },
    setFont(config) {
      const cursorOffset = model.getCursorOffset();
      const anchorOffset = model.getAnchorOffset();
      Object.assign(font, config);
      reprepare();
      cursor = offsetToCursor(prepared, cursorOffset, "forward");
      anchor = offsetToCursor(prepared, anchorOffset, "forward");
      notify();
    },

    getOptions() { return { ...opts }; },
    setOptions(config) {
      Object.assign(opts, config);
      notify();
    },

    onChange(fn) {
      changeListeners.push(fn);
      return () => {
        const i = changeListeners.indexOf(fn);
        if (i >= 0) changeListeners.splice(i, 1);
      };
    },

    onSelectionChange(fn) {
      selectionListeners.push(fn);
      return () => {
        const i = selectionListeners.indexOf(fn);
        if (i >= 0) selectionListeners.splice(i, 1);
      };
    },
  };

  return model;
}
