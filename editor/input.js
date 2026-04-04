import { nextCursor, previousCursor } from "./vendor/pretext/layout.js";
import { findVisualLineForOffset, getCursorBox, offsetFromPoint } from "./layout.js";

export function createInput(containerEl, model, view, commands) {
  let isDragging = false;
  let lastClickOffset = -1;
  let lastClickTime = 0;
  let preferredX = null;

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
    let pos = offset;
    if (direction < 0) {
      if (pos === 0) return 0;
      pos--;
      while (pos > 0 && /\W/.test(buffer[pos])) pos--;
      while (pos > 0 && /\w/.test(buffer[pos - 1])) pos--;
    } else {
      if (pos >= buffer.length) return buffer.length;
      while (pos < buffer.length && /\W/.test(buffer[pos])) pos++;
      while (pos < buffer.length && /\w/.test(buffer[pos])) pos++;
    }
    return pos;
  }

  function currentLineInfo() {
    const layout = view.getLayout();
    if (!layout) return null;
    const cursorOffset = model.getCursorOffset();
    const line = findVisualLineForOffset(cursorOffset, layout);
    if (!line) return null;
    const box = getCursorBox(cursorOffset, layout);
    return { layout, line, x: box.x };
  }

  function moveVertically(direction, extend) {
    const info = currentLineInfo();
    if (!info) return;

    const currentIndex = info.layout.lines.indexOf(info.line);
    const targetIndex = currentIndex + direction;
    if (targetIndex < 0) {
      model.setCursorOffset(0, extend);
      return;
    }
    if (targetIndex >= info.layout.lines.length) {
      model.setCursorOffset(model.getText().length, extend);
      return;
    }

    const targetLine = info.layout.lines[targetIndex];
    const targetX = preferredX ?? info.x;
    const offset = offsetFromPoint(targetX, targetLine.y + 1, info.layout);
    preferredX = targetX;
    model.setCursorOffset(offset, extend);
  }

  function onKeydown(e) {
    const buffer = model.getText();
    const cursor = model.getCursor();
    const cursorOffset = model.getCursorOffset();
    const selection = model.getSelection();
    const selectionOffsets = model.getSelectionOffsets();
    const opts = model.getOptions();

    const ctrl = isCtrl(e);
    const alt = isAlt(e);
    const shift = e.shiftKey;

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
        preferredX = null;
        if (!shift && selection) {
          model.setCursor(selection[0], false);
        } else {
          const info = currentLineInfo();
          const target = ctrl
            ? (info ? info.line.start : cursor)
            : alt
              ? model.offsetToCursor(findWordBoundary(buffer, cursorOffset, -1), "backward")
              : previousCursor(model.getPrepared(), cursor) ?? cursor;
          model.setCursor(target, shift);
        }
        break;
      }
      case "ArrowRight": {
        e.preventDefault();
        preferredX = null;
        if (!shift && selection) {
          model.setCursor(selection[1], false);
        } else {
          const info = currentLineInfo();
          const target = ctrl
            ? (info ? model.offsetToCursor(info.line.contentEndOffset, "forward") : cursor)
            : alt
              ? model.offsetToCursor(findWordBoundary(buffer, cursorOffset, 1), "forward")
              : nextCursor(model.getPrepared(), cursor) ?? cursor;
          model.setCursor(target, shift);
        }
        break;
      }
      case "ArrowUp": {
        e.preventDefault();
        if (ctrl) {
          preferredX = null;
          model.setCursorOffset(0, shift);
        } else {
          moveVertically(-1, shift);
        }
        break;
      }
      case "ArrowDown": {
        e.preventDefault();
        if (ctrl) {
          preferredX = null;
          model.setCursorOffset(buffer.length, shift);
        } else {
          moveVertically(1, shift);
        }
        break;
      }
      case "Home": {
        e.preventDefault();
        preferredX = null;
        const info = currentLineInfo();
        if (ctrl || !info) model.setCursorOffset(0, shift);
        else model.setCursor(info.line.start, shift);
        break;
      }
      case "End": {
        e.preventDefault();
        preferredX = null;
        const info = currentLineInfo();
        if (ctrl || !info) {
          model.setCursorOffset(buffer.length, shift);
        } else {
          model.setCursorOffset(info.line.contentEndOffset, shift);
        }
        break;
      }
      case "Backspace": {
        e.preventDefault();
        preferredX = null;
        if (selectionOffsets) {
          model.edit(selectionOffsets, "");
        } else {
          const target = ctrl
            ? 0
            : alt
              ? findWordBoundary(buffer, cursorOffset, -1)
              : model.cursorToOffset(previousCursor(model.getPrepared(), cursor) ?? cursor);
          if (target !== cursorOffset) model.edit([target, cursorOffset], "");
        }
        break;
      }
      case "Delete": {
        e.preventDefault();
        preferredX = null;
        if (selectionOffsets) {
          model.edit(selectionOffsets, "");
        } else {
          const target = ctrl
            ? buffer.length
            : alt
              ? findWordBoundary(buffer, cursorOffset, 1)
              : model.cursorToOffset(nextCursor(model.getPrepared(), cursor) ?? cursor);
          if (target !== cursorOffset) model.edit([cursorOffset, target], "");
        }
        break;
      }
      case "Enter": {
        e.preventDefault();
        preferredX = null;
        const range = selectionOffsets || [cursorOffset, cursorOffset];
        model.edit(range, "\n");
        break;
      }
      case "Tab": {
        e.preventDefault();
        preferredX = null;
        const range = selectionOffsets || [cursorOffset, cursorOffset];
        model.edit(range, " ".repeat(opts.tabSize));
        break;
      }
      default:
        preferredX = null;
    }
  }

  function onInput() {
    const textarea = view.textarea;
    const value = textarea.value;
    if (!value) return;
    textarea.value = "";

    preferredX = null;
    const cursorOffset = model.getCursorOffset();
    const selection = model.getSelectionOffsets();
    model.edit(selection || [cursorOffset, cursorOffset], value);
  }

  function onMousedown(e) {
    if (e.button !== 0) return;
    const now = Date.now();
    const offset = hitTest(e.clientX, e.clientY);

    if (now - lastClickTime < 300 && lastClickOffset === offset) {
      const buffer = model.getText();
      const start = findWordBoundary(buffer, offset, -1);
      const end = findWordBoundary(buffer, offset, 1);
      model.setCursorOffset(start, false, "backward");
      model.setCursorOffset(end, true, "forward");
      lastClickTime = 0;
      preferredX = null;
      view.focus();
      e.preventDefault();
      return;
    }

    lastClickTime = now;
    lastClickOffset = offset;

    preferredX = null;
    model.setCursorOffset(offset, e.shiftKey);
    isDragging = true;
    view.focus();
    e.preventDefault();
  }

  function onMousemove(e) {
    if (!isDragging) return;
    preferredX = null;
    model.setCursorOffset(hitTest(e.clientX, e.clientY), true);
  }

  function onMouseup() {
    isDragging = false;
  }

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
