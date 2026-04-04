// Text measurement for browser environments using canvas measureText.
//
// Problem: DOM-based text measurement (getBoundingClientRect, offsetHeight)
// forces synchronous layout reflow. When components independently measure text,
// each measurement triggers a reflow of the entire document. This creates
// read/write interleaving that can cost 30ms+ per frame for 500 text blocks.
//
// Solution: two-phase measurement centered around canvas measureText.
//   prepare(text, font) — segments text via Intl.Segmenter, measures each word
//     via canvas, caches widths, and does one cached DOM calibration read per
//     font when emoji correction is needed. Call once when text first appears.
//   layout(prepared, maxWidth, lineHeight) — walks cached word widths with pure
//     arithmetic to count lines and compute height. Call on every resize.
//     ~0.0002ms per text.
//
// i18n: Intl.Segmenter handles CJK (per-character breaking), Thai, Arabic, etc.
//   Bidi: simplified rich-path metadata for mixed LTR/RTL custom rendering.
//   Punctuation merging: "better." measured as one unit (matches CSS behavior).
//   Trailing whitespace: hangs past line edge without triggering breaks (CSS behavior).
//   overflow-wrap: pre-measured grapheme widths enable character-level word breaking.
//
// Emoji correction: Chrome/Firefox canvas measures emoji wider than DOM at font
//   sizes <24px on macOS (Apple Color Emoji). The inflation is constant per emoji
//   grapheme at a given size, font-independent. Auto-detected by comparing canvas
//   vs actual DOM emoji width (one cached DOM read per font). Safari canvas and
//   DOM agree (both wider than fontSize), so correction = 0 there.
//
// Limitations:
//   - system-ui font: canvas resolves to different optical variants than DOM on macOS.
//     Use named fonts (Helvetica, Inter, etc.) for guaranteed accuracy.
//     See RESEARCH.md "Discovery: system-ui font resolution mismatch".
//
// Based on Sebastian Markbage's text-layout research (github.com/chenglou/text-layout).

import { computeSegmentLevels } from './bidi.js'
import {
  analyzeText,
  clearAnalysisCaches,
  endsWithClosingQuote,
  isCJK,
  kinsokuEnd,
  kinsokuStart,
  leftStickyPunctuation,
  setAnalysisLocale,
  type AnalysisChunk,
  type SegmentBreakKind,
  type TextAnalysis,
  type WhiteSpaceMode,
} from './analysis.js'
import {
  clearMeasurementCaches,
  getCorrectedSegmentWidth,
  getEngineProfile,
  getFontMeasurementState,
  getSegmentGraphemePrefixWidths,
  getSegmentGraphemeWidths,
  getSegmentMetrics,
  textMayContainEmoji,
} from './measurement.js'
import {
  countPreparedLines,
  layoutNextLineRange as stepPreparedLineRange,
  measurePreparedLineGeometry,
  walkPreparedLines,
  type InternalLayoutLine,
} from './line-break.js'

let sharedGraphemeSegmenter: Intl.Segmenter | null = null
// Rich-path only. Reuses grapheme splits, offset maps, and derived caret data
// without pushing those caches into the public API surface.
type CachedSegmentGraphemeData = {
  graphemes: string[]
  codeUnitOffsets: Int32Array
}

type PreparedRichCache = {
  segmentData: Map<number, CachedSegmentGraphemeData>
  segmentStartOffsets: Int32Array | null
}

let sharedRichCaches = new WeakMap<PreparedTextWithSegments, PreparedRichCache>()

function getSharedGraphemeSegmenter(): Intl.Segmenter {
  if (sharedGraphemeSegmenter === null) {
    sharedGraphemeSegmenter = new Intl.Segmenter(undefined, { granularity: 'grapheme' })
  }
  return sharedGraphemeSegmenter
}

// --- Public types ---

declare const preparedTextBrand: unique symbol

type PreparedCore = {
  widths: number[] // Segment widths, e.g. [42.5, 4.4, 37.2]
  lineEndFitAdvances: number[] // Width contribution when a line ends after this segment
  lineEndPaintAdvances: number[] // Painted width contribution when a line ends after this segment
  kinds: SegmentBreakKind[] // Break behavior per segment, e.g. ['text', 'space', 'text']
  simpleLineWalkFastPath: boolean // Normal text can use the simpler old line walker across all layout APIs
  segLevels: Int8Array | null // Rich-path bidi metadata for custom rendering; layout() never reads it
  breakableWidths: (number[] | null)[] // Grapheme widths for overflow-wrap segments, else null
  breakablePrefixWidths: (number[] | null)[] // Cumulative grapheme prefix widths for narrow browser-policy shims
  segmentGraphemeWidths: (number[] | null)[] // Rich-path grapheme widths for caret geometry and cursor stepping
  discretionaryHyphenWidth: number // Visible width added when a soft hyphen is chosen as the break
  tabStopAdvance: number // Absolute advance between tab stops for pre-wrap tab segments
  chunks: PreparedLineChunk[] // Precompiled hard-break chunks for line walking
}

// Keep the main prepared handle opaque so the public API does not accidentally
// calcify around the current parallel-array representation.
export type PreparedText = {
  readonly [preparedTextBrand]: true
}

type InternalPreparedText = PreparedText & PreparedCore

// Rich/diagnostic variant that still exposes the structural segment data.
// Treat this as the unstable escape hatch for experiments and custom rendering.
export type PreparedTextWithSegments = InternalPreparedText & {
  segments: string[] // Segment text aligned with the parallel arrays, e.g. ['hello', ' ', 'world']
}

export type LayoutCursor = {
  segmentIndex: number // Segment index in `segments`
  graphemeIndex: number // Grapheme index within that segment; `0` at segment boundaries
}

export type LayoutResult = {
  lineCount: number // Number of wrapped lines, e.g. 3
  height: number // Total block height, e.g. lineCount * lineHeight = 57
}

export type LineGeometry = {
  lineCount: number
  maxLineWidth: number
}

export type LayoutLine = {
  text: string // Full text content of this line, e.g. 'hello world'
  width: number // Measured width of this line, e.g. 87.5
  start: LayoutCursor // Inclusive start cursor in prepared segments/graphemes
  end: LayoutCursor // Exclusive end cursor in prepared segments/graphemes
}

export type LayoutLineRange = {
  width: number // Measured width of this line, e.g. 87.5
  start: LayoutCursor // Inclusive start cursor in prepared segments/graphemes
  end: LayoutCursor // Exclusive end cursor in prepared segments/graphemes
}

export type OffsetAffinity = 'backward' | 'forward'

export type LineCaretGeometry = {
  x: Float32Array
  offsets: Int32Array
  contentEndOffset: number
  endOffset: number
  endsWithHardBreak: boolean
}

export type LayoutLinesResult = LayoutResult & {
  lines: LayoutLine[] // Per-line text/width pairs for custom rendering
}

export type PrepareProfile = {
  analysisMs: number
  measureMs: number
  totalMs: number
  analysisSegments: number
  preparedSegments: number
  breakableSegments: number
}

export type PrepareOptions = {
  whiteSpace?: WhiteSpaceMode
}

export type PreparedLineChunk = {
  startSegmentIndex: number
  endSegmentIndex: number
  consumedEndSegmentIndex: number
}

// --- Public API ---

function createEmptyPrepared(includeSegments: boolean): InternalPreparedText | PreparedTextWithSegments {
  if (includeSegments) {
    return {
      widths: [],
      lineEndFitAdvances: [],
      lineEndPaintAdvances: [],
      kinds: [],
      simpleLineWalkFastPath: true,
      segLevels: null,
      breakableWidths: [],
      breakablePrefixWidths: [],
      segmentGraphemeWidths: [],
      discretionaryHyphenWidth: 0,
      tabStopAdvance: 0,
      chunks: [],
      segments: [],
    } as unknown as PreparedTextWithSegments
  }
  return {
    widths: [],
    lineEndFitAdvances: [],
    lineEndPaintAdvances: [],
    kinds: [],
    simpleLineWalkFastPath: true,
    segLevels: null,
    breakableWidths: [],
    breakablePrefixWidths: [],
    segmentGraphemeWidths: [],
    discretionaryHyphenWidth: 0,
    tabStopAdvance: 0,
    chunks: [],
  } as unknown as InternalPreparedText
}

function measureAnalysis(
  analysis: TextAnalysis,
  font: string,
  includeSegments: boolean,
): InternalPreparedText | PreparedTextWithSegments {
  const graphemeSegmenter = getSharedGraphemeSegmenter()
  const engineProfile = getEngineProfile()
  const { cache, emojiCorrection } = getFontMeasurementState(
    font,
    textMayContainEmoji(analysis.normalized),
  )
  const discretionaryHyphenWidth = getCorrectedSegmentWidth('-', getSegmentMetrics('-', cache), emojiCorrection)
  const spaceWidth = getCorrectedSegmentWidth(' ', getSegmentMetrics(' ', cache), emojiCorrection)
  const tabStopAdvance = spaceWidth * 8

  if (analysis.len === 0) return createEmptyPrepared(includeSegments)

  const widths: number[] = []
  const lineEndFitAdvances: number[] = []
  const lineEndPaintAdvances: number[] = []
  const kinds: SegmentBreakKind[] = []
  let simpleLineWalkFastPath = analysis.chunks.length <= 1
  const segStarts = includeSegments ? [] as number[] : null
  const breakableWidths: (number[] | null)[] = []
  const breakablePrefixWidths: (number[] | null)[] = []
  const segmentGraphemeWidths: (number[] | null)[] = []
  const segments = includeSegments ? [] as string[] : null
  const preparedStartByAnalysisIndex = Array.from<number>({ length: analysis.len })
  const preparedEndByAnalysisIndex = Array.from<number>({ length: analysis.len })

  function pushMeasuredSegment(
    text: string,
    width: number,
    lineEndFitAdvance: number,
    lineEndPaintAdvance: number,
    kind: SegmentBreakKind,
    start: number,
    breakable: number[] | null,
    breakablePrefix: number[] | null,
    segmentGraphemeWidth: number[] | null,
  ): void {
    if (kind !== 'text' && kind !== 'space' && kind !== 'zero-width-break') {
      simpleLineWalkFastPath = false
    }
    widths.push(width)
    lineEndFitAdvances.push(lineEndFitAdvance)
    lineEndPaintAdvances.push(lineEndPaintAdvance)
    kinds.push(kind)
    segStarts?.push(start)
    breakableWidths.push(breakable)
    breakablePrefixWidths.push(breakablePrefix)
    segmentGraphemeWidths.push(segmentGraphemeWidth)
    if (segments !== null) segments.push(text)
  }

  for (let mi = 0; mi < analysis.len; mi++) {
    preparedStartByAnalysisIndex[mi] = widths.length
    const segText = analysis.texts[mi]!
    const segWordLike = analysis.isWordLike[mi]!
    const segKind = analysis.kinds[mi]!
    const segStart = analysis.starts[mi]!

    if (segKind === 'soft-hyphen') {
      pushMeasuredSegment(
        segText,
        0,
        discretionaryHyphenWidth,
        discretionaryHyphenWidth,
        segKind,
        segStart,
        null,
        null,
        null,
      )
      preparedEndByAnalysisIndex[mi] = widths.length
      continue
    }

    if (segKind === 'hard-break') {
      pushMeasuredSegment(segText, 0, 0, 0, segKind, segStart, null, null, null)
      preparedEndByAnalysisIndex[mi] = widths.length
      continue
    }

    if (segKind === 'tab') {
      pushMeasuredSegment(segText, 0, 0, 0, segKind, segStart, null, null, null)
      preparedEndByAnalysisIndex[mi] = widths.length
      continue
    }

    const segMetrics = getSegmentMetrics(segText, cache)

    if (segKind === 'text' && segMetrics.containsCJK) {
      let unitText = ''
      let unitStart = 0

      for (const gs of graphemeSegmenter.segment(segText)) {
        const grapheme = gs.segment

        if (unitText.length === 0) {
          unitText = grapheme
          unitStart = gs.index
          continue
        }

        if (
          kinsokuEnd.has(unitText) ||
          kinsokuStart.has(grapheme) ||
          leftStickyPunctuation.has(grapheme) ||
          (engineProfile.carryCJKAfterClosingQuote &&
            isCJK(grapheme) &&
            endsWithClosingQuote(unitText))
        ) {
          unitText += grapheme
          continue
        }

        const unitMetrics = getSegmentMetrics(unitText, cache)
        const w = getCorrectedSegmentWidth(unitText, unitMetrics, emojiCorrection)
        pushMeasuredSegment(unitText, w, w, w, 'text', segStart + unitStart, null, null, null)

        unitText = grapheme
        unitStart = gs.index
      }

      if (unitText.length > 0) {
        const unitMetrics = getSegmentMetrics(unitText, cache)
        const w = getCorrectedSegmentWidth(unitText, unitMetrics, emojiCorrection)
        pushMeasuredSegment(unitText, w, w, w, 'text', segStart + unitStart, null, null, null)
      }
      preparedEndByAnalysisIndex[mi] = widths.length
      continue
    }

    const w = getCorrectedSegmentWidth(segText, segMetrics, emojiCorrection)
    const lineEndFitAdvance =
      segKind === 'space' || segKind === 'preserved-space' || segKind === 'zero-width-break'
        ? 0
        : w
    const lineEndPaintAdvance =
      segKind === 'space' || segKind === 'zero-width-break'
        ? 0
        : w

    const richGraphemeWidths =
      includeSegments && segText.length > 1
        ? getSegmentGraphemeWidths(segText, segMetrics, cache, emojiCorrection)
        : null

    if (segWordLike && segText.length > 1) {
      const graphemeWidths = getSegmentGraphemeWidths(segText, segMetrics, cache, emojiCorrection)
      const graphemePrefixWidths = engineProfile.preferPrefixWidthsForBreakableRuns
        ? getSegmentGraphemePrefixWidths(segText, segMetrics, cache, emojiCorrection)
        : null
      pushMeasuredSegment(
        segText,
        w,
        lineEndFitAdvance,
        lineEndPaintAdvance,
        segKind,
        segStart,
        graphemeWidths,
        graphemePrefixWidths,
        richGraphemeWidths ?? graphemeWidths,
      )
    } else {
      pushMeasuredSegment(
        segText,
        w,
        lineEndFitAdvance,
        lineEndPaintAdvance,
        segKind,
        segStart,
        null,
        null,
        richGraphemeWidths,
      )
    }
    preparedEndByAnalysisIndex[mi] = widths.length
  }

  const chunks = mapAnalysisChunksToPreparedChunks(analysis.chunks, preparedStartByAnalysisIndex, preparedEndByAnalysisIndex)
  const segLevels = segStarts === null ? null : computeSegmentLevels(analysis.normalized, segStarts)
  if (segments !== null) {
    return {
      widths,
      lineEndFitAdvances,
      lineEndPaintAdvances,
      kinds,
      simpleLineWalkFastPath,
      segLevels,
      breakableWidths,
      breakablePrefixWidths,
      segmentGraphemeWidths,
      discretionaryHyphenWidth,
      tabStopAdvance,
      chunks,
      segments,
    } as unknown as PreparedTextWithSegments
  }
  return {
    widths,
    lineEndFitAdvances,
    lineEndPaintAdvances,
    kinds,
    simpleLineWalkFastPath,
    segLevels,
    breakableWidths,
    breakablePrefixWidths,
    segmentGraphemeWidths,
    discretionaryHyphenWidth,
    tabStopAdvance,
    chunks,
  } as unknown as InternalPreparedText
}

function mapAnalysisChunksToPreparedChunks(
  chunks: AnalysisChunk[],
  preparedStartByAnalysisIndex: number[],
  preparedEndByAnalysisIndex: number[],
): PreparedLineChunk[] {
  const preparedChunks: PreparedLineChunk[] = []
  for (let i = 0; i < chunks.length; i++) {
    const chunk = chunks[i]!
    const startSegmentIndex =
      chunk.startSegmentIndex < preparedStartByAnalysisIndex.length
        ? preparedStartByAnalysisIndex[chunk.startSegmentIndex]!
        : preparedEndByAnalysisIndex[preparedEndByAnalysisIndex.length - 1] ?? 0
    const endSegmentIndex =
      chunk.endSegmentIndex < preparedStartByAnalysisIndex.length
        ? preparedStartByAnalysisIndex[chunk.endSegmentIndex]!
        : preparedEndByAnalysisIndex[preparedEndByAnalysisIndex.length - 1] ?? 0
    const consumedEndSegmentIndex =
      chunk.consumedEndSegmentIndex < preparedStartByAnalysisIndex.length
        ? preparedStartByAnalysisIndex[chunk.consumedEndSegmentIndex]!
        : preparedEndByAnalysisIndex[preparedEndByAnalysisIndex.length - 1] ?? 0

    preparedChunks.push({
      startSegmentIndex,
      endSegmentIndex,
      consumedEndSegmentIndex,
    })
  }
  return preparedChunks
}

function prepareInternal(
  text: string,
  font: string,
  includeSegments: boolean,
  options?: PrepareOptions,
): InternalPreparedText | PreparedTextWithSegments {
  const analysis = analyzeText(text, getEngineProfile(), options?.whiteSpace)
  return measureAnalysis(analysis, font, includeSegments)
}

// Diagnostic-only helper used by the browser benchmark harness to separate the
// text-analysis and measurement phases without duplicating the prepare logic.
export function profilePrepare(text: string, font: string, options?: PrepareOptions): PrepareProfile {
  const t0 = performance.now()
  const analysis = analyzeText(text, getEngineProfile(), options?.whiteSpace)
  const t1 = performance.now()
  const prepared = measureAnalysis(analysis, font, false) as InternalPreparedText
  const t2 = performance.now()

  let breakableSegments = 0
  for (const widths of prepared.breakableWidths) {
    if (widths !== null) breakableSegments++
  }

  return {
    analysisMs: t1 - t0,
    measureMs: t2 - t1,
    totalMs: t2 - t0,
    analysisSegments: analysis.len,
    preparedSegments: prepared.widths.length,
    breakableSegments,
  }
}

// Prepare text for layout. Segments the text, measures each segment via canvas,
// and stores the widths for fast relayout at any width. Call once per text block
// (e.g. when a comment first appears). The result is width-independent — the
// same PreparedText can be laid out at any maxWidth and lineHeight via layout().
//
// Steps:
//   1. Normalize collapsible whitespace (CSS white-space: normal behavior)
//   2. Segment via Intl.Segmenter (handles CJK, Thai, etc.)
//   3. Merge punctuation into preceding word ("better." as one unit)
//   4. Split CJK words into individual graphemes (per-character line breaks)
//   5. Measure each segment via canvas measureText, cache by (segment, font)
//   6. Pre-measure graphemes of long words (for overflow-wrap: break-word)
//   7. Correct emoji canvas inflation (auto-detected per font size)
//   8. Optionally compute rich-path bidi metadata for custom renderers
export function prepare(text: string, font: string, options?: PrepareOptions): PreparedText {
  return prepareInternal(text, font, false, options) as PreparedText
}

// Rich variant used by callers that need enough information to render the
// laid-out lines themselves.
export function prepareWithSegments(text: string, font: string, options?: PrepareOptions): PreparedTextWithSegments {
  return prepareInternal(text, font, true, options) as PreparedTextWithSegments
}

function getInternalPrepared(prepared: PreparedText): InternalPreparedText {
  return prepared as InternalPreparedText
}

// Layout prepared text at a given max width and caller-provided lineHeight.
// Pure arithmetic on cached widths — no canvas calls, no DOM reads, no string
// operations, no allocations.
// ~0.0002ms per text block. Call on every resize.
//
// Line breaking rules (matching CSS white-space: normal + overflow-wrap: break-word):
//   - Break before any non-space segment that would overflow the line
//   - Trailing whitespace hangs past the line edge (doesn't trigger breaks)
//   - Segments wider than maxWidth are broken at grapheme boundaries
export function layout(prepared: PreparedText, maxWidth: number, lineHeight: number): LayoutResult {
  // Keep the resize hot path specialized. `layoutWithLines()` shares the same
  // break semantics but also tracks line ranges; the extra bookkeeping is too
  // expensive to pay on every hot-path `layout()` call.
  const lineCount = countPreparedLines(getInternalPrepared(prepared), maxWidth)
  return { lineCount, height: lineCount * lineHeight }
}

function getRichCache(prepared: PreparedTextWithSegments): PreparedRichCache {
  let cache = sharedRichCaches.get(prepared)
  if (cache !== undefined) return cache

  cache = {
    segmentData: new Map<number, CachedSegmentGraphemeData>(),
    segmentStartOffsets: null,
  }
  sharedRichCaches.set(prepared, cache)
  return cache
}

function getSegmentStartOffsets(prepared: PreparedTextWithSegments): Int32Array {
  const cache = getRichCache(prepared)
  if (cache.segmentStartOffsets !== null) return cache.segmentStartOffsets

  const offsets = new Int32Array(prepared.segments.length + 1)
  for (let i = 0; i < prepared.segments.length; i++) {
    offsets[i + 1] = offsets[i]! + prepared.segments[i]!.length
  }

  cache.segmentStartOffsets = offsets
  return offsets
}

function getSegmentGraphemeData(
  prepared: PreparedTextWithSegments,
  segmentIndex: number,
): CachedSegmentGraphemeData {
  const cache = getRichCache(prepared)
  let data = cache.segmentData.get(segmentIndex)
  if (data !== undefined) return data

  const graphemes: string[] = []
  const graphemeSegmenter = getSharedGraphemeSegmenter()
  for (const gs of graphemeSegmenter.segment(prepared.segments[segmentIndex]!)) {
    graphemes.push(gs.segment)
  }

  const codeUnitOffsets = new Int32Array(graphemes.length + 1)
  for (let i = 0; i < graphemes.length; i++) {
    codeUnitOffsets[i + 1] = codeUnitOffsets[i]! + graphemes[i]!.length
  }

  data = { graphemes, codeUnitOffsets }
  cache.segmentData.set(segmentIndex, data)
  return data
}

function getSegmentGraphemes(prepared: PreparedTextWithSegments, segmentIndex: number): string[] {
  return getSegmentGraphemeData(prepared, segmentIndex).graphemes
}

function getSegmentCodeUnitOffsets(prepared: PreparedTextWithSegments, segmentIndex: number): Int32Array {
  return getSegmentGraphemeData(prepared, segmentIndex).codeUnitOffsets
}

function getSegmentGraphemeCount(prepared: PreparedTextWithSegments, segmentIndex: number): number {
  return getSegmentGraphemeData(prepared, segmentIndex).graphemes.length
}

function cursorAtSegmentBoundary(
  segmentIndex: number,
  graphemeBoundaryIndex: number,
  graphemeCount: number,
): LayoutCursor {
  if (graphemeBoundaryIndex <= 0) {
    return {
      segmentIndex,
      graphemeIndex: 0,
    }
  }

  if (graphemeBoundaryIndex >= graphemeCount) {
    return {
      segmentIndex: segmentIndex + 1,
      graphemeIndex: 0,
    }
  }

  return {
    segmentIndex,
    graphemeIndex: graphemeBoundaryIndex,
  }
}

function compareCursors(a: LayoutCursor, b: LayoutCursor): number {
  if (a.segmentIndex !== b.segmentIndex) return a.segmentIndex - b.segmentIndex
  return a.graphemeIndex - b.graphemeIndex
}

export function cursorToOffset(prepared: PreparedTextWithSegments, cursor: LayoutCursor): number {
  const segmentStartOffsets = getSegmentStartOffsets(prepared)
  if (cursor.segmentIndex >= prepared.segments.length) {
    return segmentStartOffsets[prepared.segments.length]!
  }

  if (cursor.graphemeIndex <= 0) {
    return segmentStartOffsets[cursor.segmentIndex]!
  }

  const localOffsets = getSegmentCodeUnitOffsets(prepared, cursor.segmentIndex)
  const boundaryIndex = Math.min(cursor.graphemeIndex, localOffsets.length - 1)
  return segmentStartOffsets[cursor.segmentIndex]! + localOffsets[boundaryIndex]!
}

export function offsetToCursor(
  prepared: PreparedTextWithSegments,
  offset: number,
  affinity: OffsetAffinity = 'forward',
): LayoutCursor {
  const segmentStartOffsets = getSegmentStartOffsets(prepared)
  const clampedOffset = Math.max(0, Math.min(offset, segmentStartOffsets[prepared.segments.length]!))
  if (clampedOffset === segmentStartOffsets[prepared.segments.length]!) {
    return {
      segmentIndex: prepared.segments.length,
      graphemeIndex: 0,
    }
  }

  let lo = 0
  let hi = prepared.segments.length
  while (lo < hi) {
    const mid = Math.floor((lo + hi) / 2)
    if (segmentStartOffsets[mid + 1]! <= clampedOffset) {
      lo = mid + 1
    } else {
      hi = mid
    }
  }

  const segmentIndex = lo
  const localOffset = clampedOffset - segmentStartOffsets[segmentIndex]!
  if (localOffset <= 0) {
    return {
      segmentIndex,
      graphemeIndex: 0,
    }
  }

  const localOffsets = getSegmentCodeUnitOffsets(prepared, segmentIndex)
  const graphemeCount = localOffsets.length - 1
  for (let i = 1; i < localOffsets.length; i++) {
    const boundary = localOffsets[i]!
    if (boundary === localOffset) {
      return cursorAtSegmentBoundary(segmentIndex, i, graphemeCount)
    }
    if (boundary > localOffset) {
      return affinity === 'backward'
        ? cursorAtSegmentBoundary(segmentIndex, i - 1, graphemeCount)
        : cursorAtSegmentBoundary(segmentIndex, i, graphemeCount)
    }
  }

  return {
    segmentIndex: segmentIndex + 1,
    graphemeIndex: 0,
  }
}

export function nextCursor(prepared: PreparedTextWithSegments, cursor: LayoutCursor): LayoutCursor | null {
  if (cursor.segmentIndex >= prepared.segments.length) return null

  const graphemeCount = getSegmentGraphemeCount(prepared, cursor.segmentIndex)
  if (graphemeCount <= 1) {
    return {
      segmentIndex: cursor.segmentIndex + 1,
      graphemeIndex: 0,
    }
  }

  if (cursor.graphemeIndex <= 0) {
    return {
      segmentIndex: cursor.segmentIndex,
      graphemeIndex: 1,
    }
  }

  if (cursor.graphemeIndex >= graphemeCount - 1) {
    return {
      segmentIndex: cursor.segmentIndex + 1,
      graphemeIndex: 0,
    }
  }

  return {
    segmentIndex: cursor.segmentIndex,
    graphemeIndex: cursor.graphemeIndex + 1,
  }
}

export function previousCursor(prepared: PreparedTextWithSegments, cursor: LayoutCursor): LayoutCursor | null {
  if (cursor.segmentIndex === 0 && cursor.graphemeIndex === 0) return null

  if (cursor.segmentIndex >= prepared.segments.length) {
    const previousSegmentIndex = prepared.segments.length - 1
    const graphemeCount = getSegmentGraphemeCount(prepared, previousSegmentIndex)
    return graphemeCount <= 1
      ? { segmentIndex: previousSegmentIndex, graphemeIndex: 0 }
      : { segmentIndex: previousSegmentIndex, graphemeIndex: graphemeCount - 1 }
  }

  if (cursor.graphemeIndex <= 0) {
    const previousSegmentIndex = cursor.segmentIndex - 1
    const graphemeCount = getSegmentGraphemeCount(prepared, previousSegmentIndex)
    return graphemeCount <= 1
      ? { segmentIndex: previousSegmentIndex, graphemeIndex: 0 }
      : { segmentIndex: previousSegmentIndex, graphemeIndex: graphemeCount - 1 }
  }

  if (cursor.graphemeIndex === 1) {
    return {
      segmentIndex: cursor.segmentIndex,
      graphemeIndex: 0,
    }
  }

  return {
    segmentIndex: cursor.segmentIndex,
    graphemeIndex: cursor.graphemeIndex - 1,
  }
}

function getTabAdvance(lineWidth: number, tabStopAdvance: number): number {
  if (tabStopAdvance <= 0) return 0

  const remainder = lineWidth % tabStopAdvance
  if (Math.abs(remainder) <= 1e-6) return tabStopAdvance
  return tabStopAdvance - remainder
}

function lineHasDiscretionaryHyphen(
  kinds: SegmentBreakKind[],
  startSegmentIndex: number,
  startGraphemeIndex: number,
  endSegmentIndex: number,
): boolean {
  return (
    endSegmentIndex > 0 &&
    kinds[endSegmentIndex - 1] === 'soft-hyphen' &&
    !(startSegmentIndex === endSegmentIndex && startGraphemeIndex > 0)
  )
}

function buildLineTextFromRange(
  prepared: PreparedTextWithSegments,
  kinds: SegmentBreakKind[],
  startSegmentIndex: number,
  startGraphemeIndex: number,
  endSegmentIndex: number,
  endGraphemeIndex: number,
): string {
  let text = ''
  const endsWithDiscretionaryHyphen = lineHasDiscretionaryHyphen(
    kinds,
    startSegmentIndex,
    startGraphemeIndex,
    endSegmentIndex,
  )

  for (let i = startSegmentIndex; i < endSegmentIndex; i++) {
    if (kinds[i] === 'soft-hyphen' || kinds[i] === 'hard-break') continue
    if (i === startSegmentIndex && startGraphemeIndex > 0) {
      text += getSegmentGraphemes(prepared, i).slice(startGraphemeIndex).join('')
    } else {
      text += prepared.segments[i]!
    }
  }

  if (endGraphemeIndex > 0) {
    if (endsWithDiscretionaryHyphen) text += '-'
    text += getSegmentGraphemes(prepared, endSegmentIndex).slice(
      startSegmentIndex === endSegmentIndex ? startGraphemeIndex : 0,
      endGraphemeIndex,
    ).join('')
  } else if (endsWithDiscretionaryHyphen) {
    text += '-'
  }

  return text
}

function createLayoutLine(
  prepared: PreparedTextWithSegments,
  width: number,
  startSegmentIndex: number,
  startGraphemeIndex: number,
  endSegmentIndex: number,
  endGraphemeIndex: number,
): LayoutLine {
  return {
    text: buildLineTextFromRange(
      prepared,
      prepared.kinds,
      startSegmentIndex,
      startGraphemeIndex,
      endSegmentIndex,
      endGraphemeIndex,
    ),
    width,
    start: {
      segmentIndex: startSegmentIndex,
      graphemeIndex: startGraphemeIndex,
    },
    end: {
      segmentIndex: endSegmentIndex,
      graphemeIndex: endGraphemeIndex,
    },
  }
}

function materializeLayoutLine(
  prepared: PreparedTextWithSegments,
  line: InternalLayoutLine,
): LayoutLine {
  return createLayoutLine(
    prepared,
    line.width,
    line.startSegmentIndex,
    line.startGraphemeIndex,
    line.endSegmentIndex,
    line.endGraphemeIndex,
  )
}

function toLayoutLineRange(line: InternalLayoutLine): LayoutLineRange {
  return {
    width: line.width,
    start: {
      segmentIndex: line.startSegmentIndex,
      graphemeIndex: line.startGraphemeIndex,
    },
    end: {
      segmentIndex: line.endSegmentIndex,
      graphemeIndex: line.endGraphemeIndex,
    },
  }
}

function stepLineRange(
  prepared: PreparedTextWithSegments,
  start: LayoutCursor,
  maxWidth: number,
): LayoutLineRange | null {
  const line = stepPreparedLineRange(prepared, start, maxWidth)
  if (line === null) return null
  return toLayoutLineRange(line)
}

function materializeLine(
  prepared: PreparedTextWithSegments,
  line: LayoutLineRange,
): LayoutLine {
  return createLayoutLine(
    prepared,
    line.width,
    line.start.segmentIndex,
    line.start.graphemeIndex,
    line.end.segmentIndex,
    line.end.graphemeIndex,
  )
}

export function materializeLineRange(
  prepared: PreparedTextWithSegments,
  line: LayoutLineRange,
): LayoutLine {
  return materializeLine(prepared, line)
}

// Batch low-level line geometry pass. This is the non-materializing counterpart
// to layoutWithLines(), useful for shrinkwrap and other aggregate geometry work.
export function walkLineRanges(
  prepared: PreparedTextWithSegments,
  maxWidth: number,
  onLine: (line: LayoutLineRange) => void,
): number {
  if (prepared.widths.length === 0) return 0

  return walkPreparedLines(getInternalPrepared(prepared), maxWidth, line => {
    onLine(toLayoutLineRange(line))
  })
}

export function measureLineGeometry(
  prepared: PreparedTextWithSegments,
  maxWidth: number,
): LineGeometry {
  return measurePreparedLineGeometry(getInternalPrepared(prepared), maxWidth)
}

// Intrinsic-width helper for rich/userland layout work. This asks "how wide is
// the prepared text when container width is not the thing forcing wraps?".
// Explicit hard breaks still count, so this returns the widest forced line.
export function measureNaturalWidth(prepared: PreparedTextWithSegments): number {
  let maxWidth = 0
  walkLineRanges(prepared, Number.POSITIVE_INFINITY, line => {
    if (line.width > maxWidth) maxWidth = line.width
  })
  return maxWidth
}

function lineEndsWithHardBreak(prepared: PreparedTextWithSegments, line: LayoutLine | LayoutLineRange): boolean {
  return (
    line.end.graphemeIndex === 0 &&
    line.end.segmentIndex > 0 &&
    prepared.kinds[line.end.segmentIndex - 1] === 'hard-break'
  )
}

function getLineContentEndCursor(
  prepared: PreparedTextWithSegments,
  line: LayoutLine | LayoutLineRange,
): LayoutCursor {
  if (!lineEndsWithHardBreak(prepared, line)) {
    return line.end
  }

  return {
    segmentIndex: line.end.segmentIndex - 1,
    graphemeIndex: 0,
  }
}

export function measureLineCarets(
  prepared: PreparedTextWithSegments,
  line: LayoutLine | LayoutLineRange,
): LineCaretGeometry {
  const contentEnd = getLineContentEndCursor(prepared, line)
  const endsWithHardBreak = lineEndsWithHardBreak(prepared, line)
  const contentEndOffset = cursorToOffset(prepared, contentEnd)
  const endOffset = cursorToOffset(prepared, line.end)
  const xPositions = [0]
  const boundaryOffsets = [cursorToOffset(prepared, line.start)]
  const segmentStartOffsets = getSegmentStartOffsets(prepared)
  let x = 0

  function appendSegmentGraphemes(
    segmentIndex: number,
    startGraphemeIndex: number,
    endGraphemeIndex: number,
  ): void {
    if (startGraphemeIndex >= endGraphemeIndex) return

    const kind = prepared.kinds[segmentIndex]!
    if (kind === 'soft-hyphen' || kind === 'hard-break') return

    const graphemeCount = getSegmentGraphemeCount(prepared, segmentIndex)
    const segmentOffsets = getSegmentCodeUnitOffsets(prepared, segmentIndex)
    const segmentWidths = prepared.segmentGraphemeWidths[segmentIndex]

    for (let graphemeIndex = startGraphemeIndex; graphemeIndex < endGraphemeIndex; graphemeIndex++) {
      const graphemeWidth =
        kind === 'tab'
          ? getTabAdvance(x, prepared.tabStopAdvance)
          : segmentWidths?.[graphemeIndex] ?? prepared.widths[segmentIndex]!

      x += graphemeWidth
      xPositions.push(x)
      boundaryOffsets.push(segmentStartOffsets[segmentIndex]! + segmentOffsets[graphemeIndex + 1]!)
    }

    if (endGraphemeIndex === graphemeCount && boundaryOffsets[boundaryOffsets.length - 1] !== segmentStartOffsets[segmentIndex + 1]!) {
      boundaryOffsets[boundaryOffsets.length - 1] = segmentStartOffsets[segmentIndex + 1]!
    }
  }

  if (compareCursors(line.start, contentEnd) < 0) {
    for (let segmentIndex = line.start.segmentIndex; segmentIndex < contentEnd.segmentIndex; segmentIndex++) {
      const startGraphemeIndex = segmentIndex === line.start.segmentIndex ? line.start.graphemeIndex : 0
      appendSegmentGraphemes(
        segmentIndex,
        startGraphemeIndex,
        getSegmentGraphemeCount(prepared, segmentIndex),
      )
    }

    if (contentEnd.graphemeIndex > 0 && contentEnd.segmentIndex < prepared.segments.length) {
      appendSegmentGraphemes(
        contentEnd.segmentIndex,
        contentEnd.segmentIndex === line.start.segmentIndex ? line.start.graphemeIndex : 0,
        contentEnd.graphemeIndex,
      )
    }
  }

  if (lineHasDiscretionaryHyphen(
    prepared.kinds,
    line.start.segmentIndex,
    line.start.graphemeIndex,
    line.end.segmentIndex,
  )) {
    x += prepared.discretionaryHyphenWidth
  }

  xPositions[xPositions.length - 1] = line.width
  boundaryOffsets[boundaryOffsets.length - 1] = contentEndOffset

  return {
    x: Float32Array.from(xPositions),
    offsets: Int32Array.from(boundaryOffsets),
    contentEndOffset,
    endOffset,
    endsWithHardBreak,
  }
}

export function layoutNextLine(
  prepared: PreparedTextWithSegments,
  start: LayoutCursor,
  maxWidth: number,
): LayoutLine | null {
  const line = layoutNextLineRange(prepared, start, maxWidth)
  if (line === null) return null
  return materializeLineRange(prepared, line)
}

export function layoutNextLineRange(
  prepared: PreparedTextWithSegments,
  start: LayoutCursor,
  maxWidth: number,
): LayoutLineRange | null {
  return stepLineRange(prepared, start, maxWidth)
}

// Rich layout API for callers that want the actual line contents and widths.
// Caller still supplies lineHeight at layout time. Mirrors layout()'s break
// decisions, but keeps extra per-line bookkeeping so it should stay off the
// resize hot path.
export function layoutWithLines(prepared: PreparedTextWithSegments, maxWidth: number, lineHeight: number): LayoutLinesResult {
  const lines: LayoutLine[] = []
  if (prepared.widths.length === 0) return { lineCount: 0, height: 0, lines }

  const lineCount = walkPreparedLines(getInternalPrepared(prepared), maxWidth, line => {
    lines.push(materializeLayoutLine(prepared, line))
  })

  return { lineCount, height: lineCount * lineHeight, lines }
}

export function clearCache(): void {
  clearAnalysisCaches()
  sharedGraphemeSegmenter = null
  sharedRichCaches = new WeakMap<PreparedTextWithSegments, PreparedRichCache>()
  clearMeasurementCaches()
}

export function setLocale(locale?: string): void {
  setAnalysisLocale(locale)
  clearCache()
}
