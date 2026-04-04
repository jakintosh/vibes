// Input — attaches DOM event listeners, translates raw events into API calls.

import { offsetFromPoint } from "./layout.js";

export function createInput(containerEl, model, view, commands) {
  let isDragging = false;
  let lastClickOffset = -1;
  let lastClickTime = 0;

  function isMac() {
    return /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
  }

  function isCtrl(e) {
    return isMac() ? e.metaKey : e.ctrlKey;
  }

  function isAlt(e) {
    return e.altKey;
  }

  function hitTest(clientX, clientY) {
    const rect = containerEl.getBoundingClientRect();
    const x = clientX - rect.left + containerEl.scrollLeft;
    const y = clientY - rect.top + containerEl.scrollTop;
    const layout = view.getLayout();
    if (!layout) return 0;
    return offsetFromPoint(x, y, layout);
  }

  function findWordBoundary(buffer, offset, direction) {
    // direction: -1 for left, +1 for right
    let pos = offset;
    if (direction < 0) {
      if (pos === 0) return 0;
      pos--;
      // Skip whitespace
      while (pos > 0 && /\W/.test(buffer[pos])) pos--;
      // Skip word chars
      while (pos > 0 && /\w/.test(buffer[pos - 1])) pos--;
    } else {
      if (pos >= buffer.length) return buffer.length;
      // Skip whitespace
      while (pos < buffer.length && /\W/.test(buffer[pos])) pos++;
      // Skip word chars
      while (pos < buffer.length && /\w/.test(buffer[pos])) pos++;
    }
    return pos;
  }

  function findLineStart(offset) {
    const buffer = model.getText();
    let pos = offset;
    while (pos > 0 && buffer[pos - 1] !== "\n") pos--;
    return pos;
  }

  function findLineEnd(offset) {
    const buffer = model.getText();
    let pos = offset;
    while (pos < buffer.length && buffer[pos] !== "\n") pos++;
    return pos;
  }

  function targetLeft(buffer, cursor, ctrl, alt) {
    if (ctrl) return findLineStart(cursor);
    if (alt)  return findWordBoundary(buffer, cursor, -1);
    return Math.max(0, cursor - 1);
  }

  function targetRight(buffer, cursor, ctrl, alt) {
    if (ctrl) return findLineEnd(cursor);
    if (alt)  return findWordBoundary(buffer, cursor, 1);
    return Math.min(buffer.length, cursor + 1);
  }

  function getVisualXForOffset(offset) {
    const layout = view.getLayout();
    if (!layout) return 0;
    for (const line of layout.lines) {
      for (const box of line.charBoxes) {
        if (box.offset === offset) return box.x;
      }
    }
    return 0;
  }

  function moveVertically(offset, direction, extend) {
    const layout = view.getLayout();
    if (!layout || !layout.lines.length) return;

    // Find current visual line and x
    let currentLineIdx = -1;
    let currentX = 0;
    for (let i = 0; i < layout.lines.length; i++) {
      const line = layout.lines[i];
      for (const box of line.charBoxes) {
        if (box.offset === offset) {
          currentLineIdx = i;
          currentX = box.x;
          break;
        }
      }
      if (currentLineIdx >= 0) break;
    }

    if (currentLineIdx < 0) {
      // fallback
      currentLineIdx = direction < 0 ? 1 : layout.lines.length - 2;
      currentX = 0;
    }

    const targetIdx = currentLineIdx + direction;
    if (targetIdx < 0) {
      model.setCursor(0, extend);
      return;
    }
    if (targetIdx >= layout.lines.length) {
      model.setCursor(model.getText().length, extend);
      return;
    }

    const targetLine = layout.lines[targetIdx];
    const newOffset = offsetFromPoint(currentX, targetLine.y + 1, layout);
    model.setCursor(newOffset, extend);
  }

  function onKeydown(e) {
    const cursor = model.getCursor();
    const buffer = model.getText();
    const selection = model.getSelection();
    const opts = model.getOptions();

    const ctrl = isCtrl(e);
    const alt = isAlt(e);
    const shift = e.shiftKey;

    // Command shortcuts
    if (ctrl && !alt) {
      switch (e.key.toLowerCase()) {
        case "a":
          e.preventDefault();
          commands.exec("selectAll");
          return;
        case "c":
          e.preventDefault();
          commands.exec("copy");
          return;
        case "x":
          e.preventDefault();
          commands.exec("cut");
          return;
        case "v":
          e.preventDefault();
          commands.exec("paste");
          return;
        case "z":
          e.preventDefault();
          if (shift) commands.exec("redo");
          else commands.exec("undo");
          return;
        case "y":
          e.preventDefault();
          commands.exec("redo");
          return;
      }
    }

    switch (e.key) {
      case "ArrowLeft": {
        e.preventDefault();
        if (!shift && selection) {
          model.setCursor(selection[0], false);
        } else {
          model.setCursor(targetLeft(buffer, cursor, ctrl, alt), shift);
        }
        break;
      }
      case "ArrowRight": {
        e.preventDefault();
        if (!shift && selection) {
          model.setCursor(selection[1], false);
        } else {
          model.setCursor(targetRight(buffer, cursor, ctrl, alt), shift);
        }
        break;
      }
      case "ArrowUp": {
        e.preventDefault();
        if (ctrl) model.setCursor(0, shift);
        else moveVertically(cursor, -1, shift);
        break;
      }
      case "ArrowDown": {
        e.preventDefault();
        if (ctrl) model.setCursor(buffer.length, shift);
        else moveVertically(cursor, 1, shift);
        break;
      }
      case "Home": {
        e.preventDefault();
        if (ctrl) model.setCursor(0, shift);
        else model.setCursor(findLineStart(cursor), shift);
        break;
      }
      case "End": {
        e.preventDefault();
        if (ctrl) model.setCursor(buffer.length, shift);
        else model.setCursor(findLineEnd(cursor), shift);
        break;
      }
      case "Backspace": {
        e.preventDefault();
        if (selection) {
          model.edit(selection, "");
        } else {
          const target = targetLeft(buffer, cursor, ctrl, alt);
          if (target !== cursor) model.edit([target, cursor], "");
        }
        break;
      }
      case "Delete": {
        e.preventDefault();
        if (selection) {
          model.edit(selection, "");
        } else {
          const target = targetRight(buffer, cursor, ctrl, alt);
          if (target !== cursor) model.edit([cursor, target], "");
        }
        break;
      }
      case "Enter": {
        e.preventDefault();
        const range = selection || [cursor, cursor];
        model.edit(range, "\n");
        break;
      }
      case "Tab": {
        e.preventDefault();
        const range = selection || [cursor, cursor];
        model.edit(range, " ".repeat(opts.tabSize));
        break;
      }
    }
  }

  function onInput(e) {
    const textarea = view.textarea;
    const value = textarea.value;
    if (!value) return;
    textarea.value = "";

    const selection = model.getSelection();
    const cursor = model.getCursor();
    const range = selection || [cursor, cursor];
    model.edit(range, value);
  }

  function onMousedown(e) {
    if (e.button !== 0) return;
    const now = Date.now();
    const offset = hitTest(e.clientX, e.clientY);

    // Double-click detection
    if (now - lastClickTime < 300 && lastClickOffset === offset) {
      // Select word
      const buffer = model.getText();
      const start = findWordBoundary(buffer, offset, -1);
      const end = findWordBoundary(buffer, offset, 1);
      model.setCursor(start, false);
      model.setCursor(end, true);
      lastClickTime = 0;
      view.focus();
      e.preventDefault();
      return;
    }

    lastClickTime = now;
    lastClickOffset = offset;

    model.setCursor(offset, e.shiftKey);
    isDragging = true;
    view.focus();
    e.preventDefault();
  }

  function onMousemove(e) {
    if (!isDragging) return;
    const offset = hitTest(e.clientX, e.clientY);
    model.setCursor(offset, true);
  }

  function onMouseup() {
    isDragging = false;
  }

  // Attach listeners
  const textarea = view.textarea;

  textarea.addEventListener("keydown", onKeydown);
  textarea.addEventListener("input", onInput);
  containerEl.addEventListener("mousedown", onMousedown);
  window.addEventListener("mousemove", onMousemove);
  window.addEventListener("mouseup", onMouseup);

  return {
    detach() {
      textarea.removeEventListener("keydown", onKeydown);
      textarea.removeEventListener("input", onInput);
      containerEl.removeEventListener("mousedown", onMousedown);
      window.removeEventListener("mousemove", onMousemove);
      window.removeEventListener("mouseup", onMouseup);
    },
  };
}
