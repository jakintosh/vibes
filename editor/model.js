// Model — owns the text buffer, cursor, selection, and font config.

export function createModel(initialText = "", fontConfig = {}, options = {}) {
  let buffer = initialText;
  let cursor = 0;
  let anchor = 0;

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

  const changeListeners = [];
  const selectionListeners = [];

  function notify() {
    for (const fn of changeListeners) fn(buffer);
  }

  function notifySelection() {
    for (const fn of selectionListeners) fn();
  }

  function adjustOffset(pos, start, end, replacementLen) {
    if (pos < start) return pos;
    if (pos < end) return start + replacementLen;
    return pos + (start + replacementLen - end);
  }

  const model = {
    // Mutation API
    edit(range, replacement) {
      if (opts.readOnly) return;
      const [start, end] = [Math.min(range[0], range[1]), Math.max(range[0], range[1])];
      const clampedStart = Math.max(0, Math.min(start, buffer.length));
      const clampedEnd = Math.max(0, Math.min(end, buffer.length));

      buffer = buffer.slice(0, clampedStart) + replacement + buffer.slice(clampedEnd);

      cursor = adjustOffset(cursor, clampedStart, clampedEnd, replacement.length);
      anchor = adjustOffset(anchor, clampedStart, clampedEnd, replacement.length);

      notify();
    },

    setCursor(offset, extend = false) {
      const clamped = Math.max(0, Math.min(offset, buffer.length));
      cursor = clamped;
      if (!extend) anchor = clamped;
      notifySelection();
      notify();
    },

    // Read helpers
    getText() { return buffer; },
    setText(text) {
      buffer = text;
      cursor = Math.min(cursor, buffer.length);
      anchor = Math.min(anchor, buffer.length);
      notify();
    },
    getCursor() { return cursor; },
    getAnchor() { return anchor; },
    getSelection() {
      if (anchor === cursor) return null;
      return [Math.min(anchor, cursor), Math.max(anchor, cursor)];
    },
    getSelectedText() {
      const sel = model.getSelection();
      if (!sel) return "";
      return buffer.slice(sel[0], sel[1]);
    },

    getLines() {
      const lines = [];
      let offset = 0;
      const parts = buffer.split("\n");
      for (const text of parts) {
        lines.push({ text, startOffset: offset });
        offset += text.length + 1; // +1 for the \n
      }
      return lines;
    },

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
      Object.assign(font, config);
      notify();
    },

    getOptions() { return { ...opts }; },
    setOptions(config) {
      Object.assign(opts, config);
      notify();
    },

    onChange(fn) {
      changeListeners.push(fn);
      return () => { const i = changeListeners.indexOf(fn); if (i >= 0) changeListeners.splice(i, 1); };
    },

    onSelectionChange(fn) {
      selectionListeners.push(fn);
      return () => { const i = selectionListeners.indexOf(fn); if (i >= 0) selectionListeners.splice(i, 1); };
    },
  };

  return model;
}
