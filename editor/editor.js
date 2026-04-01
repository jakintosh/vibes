// editor.js — Public API. Ties Model, Layout, View, and Input together.

import { createModel } from "./model.js";
import { createView } from "./view.js";
import { createInput } from "./input.js";

export function createEditor(containerEl, options = {}) {
  const { text = "", font = {}, ...editorOptions } = options;

  const model = createModel(text, font, editorOptions);

  // --- Undo stack ---
  const undoStack = [];
  const redoStack = [];
  let undoEnabled = false;

  // Wrap model.edit to intercept for undo
  const originalEdit = model.edit.bind(model);
  model.edit = function(range, replacement) {
    if (undoEnabled) {
      const [start, end] = [
        Math.max(0, Math.min(range[0], model.getText().length)),
        Math.max(0, Math.min(range[1], model.getText().length)),
      ];
      const removed = model.getText().slice(start, end);
      undoStack.push({ range: [start, end], removed, inserted: replacement, cursorBefore: model.getCursor() });
      redoStack.length = 0;
    }
    originalEdit(range, replacement);
  };

  // --- View ---
  const view = createView(containerEl, model);

  // --- Commands ---
  const commandDefs = {
    copy: {
      default: (ed) => {
        const text = model.getSelectedText();
        if (text) navigator.clipboard.writeText(text).catch(() => {});
      },
    },
    cut: {
      default: (ed) => {
        const sel = model.getSelection();
        if (!sel) return;
        const text = model.getSelectedText();
        navigator.clipboard.writeText(text).catch(() => {});
        model.edit(sel, "");
      },
    },
    paste: {
      default: (ed) => {
        navigator.clipboard.readText().then((text) => {
          const sel = model.getSelection();
          const cursor = model.getCursor();
          model.edit(sel || [cursor, cursor], text);
        }).catch(() => {});
      },
    },
    selectAll: {
      default: (ed) => {
        model.setCursor(0, false);
        model.setCursor(model.getText().length, true);
      },
    },
    undo: {
      default: (ed) => {
        if (!undoEnabled || !undoStack.length) return;
        const entry = undoStack.pop();
        const insertedLen = entry.inserted.length;
        const invertRange = [entry.range[0], entry.range[0] + insertedLen];
        redoStack.push({
          range: invertRange,
          removed: entry.inserted,
          inserted: entry.removed,
          cursorBefore: model.getCursor(),
        });
        originalEdit(invertRange, entry.removed);
        model.setCursor(entry.cursorBefore, false);
      },
    },
    redo: {
      default: (ed) => {
        if (!undoEnabled || !redoStack.length) return;
        const entry = redoStack.pop();
        undoStack.push({
          range: entry.range,
          removed: entry.removed,
          inserted: entry.inserted,
          cursorBefore: model.getCursor(),
        });
        originalEdit(entry.range, entry.inserted);
      },
    },
  };

  const activeCommands = new Map();

  const commands = {
    exec(name) {
      const fn = activeCommands.get(name);
      if (fn) fn(editor);
    },
    enable(name) {
      const def = commandDefs[name];
      if (def) activeCommands.set(name, def.default);
    },
    disable(name) {
      activeCommands.delete(name);
    },
    set(name, fn) {
      activeCommands.set(name, fn);
    },
  };

  // --- Input ---
  const input = createInput(containerEl, model, view, commands);

  // --- Public API ---
  const editor = {
    // Core
    edit(range, replacement) { model.edit(range, replacement); },
    setCursor(offset, extend) { model.setCursor(offset, extend); },

    // Read
    getText() { return model.getText(); },
    setText(text) { model.setText(text); },
    getCursor() { return model.getCursor(); },
    getSelection() { return model.getSelection(); },
    getSelectedText() { return model.getSelectedText(); },

    // Commands
    enableCommand(name) {
      if (name === "undo" || name === "redo") undoEnabled = true;
      commands.enable(name);
    },
    disableCommand(name) {
      commands.disable(name);
    },
    setCommand(name, fn) {
      commands.set(name, fn);
    },
    execCommand(name) {
      commands.exec(name);
    },

    // Config
    setFont(config) { model.setFont(config); },
    setOptions(opts) { model.setOptions(opts); },

    // Lifecycle
    onChange(callback) { return model.onChange(callback); },
    onSelectionChange(callback) { return model.onSelectionChange(callback); },
    focus() { view.focus(); },
    destroy() {
      input.detach();
      view.destroy();
    },
  };

  return editor;
}
