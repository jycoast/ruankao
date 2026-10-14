// Soft exam morning questions often pack multiple blanks into one item.
// Options are usually grouped by 4 (A/B/C/D) per blank.
// Some imported papers instead split one multi-blank stem into several
// single-choice questions, each with a red banner like「本题作答（_问题2_）」.

import { renderMd } from '@/utils/markdown'

export const BLANK_SIZE = 4

// Empty parentheses / underscore blanks, with or without「问题N」text.
// Also accepts HTML wrappers commonly present in imported stems, e.g.
// （<u> 问题2</u>） / （_问题2_） / （ ） / （  ）(nbsp).
const BLANK_CORE =
  '(?:' +
  // （问题N） / （_问题N_） / （ 问题 N ） with optional inner tags / nbsp
  '\\uFF08\\s*(?:<[^>]+>\\s*)*_?\\s*(?:\\u95EE\\u9898\\s*)?\\d*\\s*_?(?:\\s*<[^>]+>)*\\s*\\uFF09' +
  '|' +
  // half-width ()
  '\\(\\s*(?:<[^>]+>\\s*)*_?\\s*(?:\\u95EE\\u9898\\s*)?\\d*\\s*_?(?:\\s*<[^>]+>)*\\s*\\)' +
  '|' +
  // bare underscores used as blanks
  '_{2,}' +
  ')'

const BLANK_PATTERN = new RegExp(BLANK_CORE, 'gi')

// Red banner that names the blank this *single* question is targeting.
// Example: <font color='red'>本题作答（_问题2_）：</font>
const TARGET_BLANK_PATTERN =
  /\u672C\u9898\u4F5C\u7B54\s*[（(]\s*_?\s*\u95EE\u9898\s*(\d+)\s*_?\s*[）)]/i

// Banner as a leading prefix of content, e.g.
// <font color='red'>本题作答（_问题2_）：</font> 题干…
// Strip it before comparing content against title (stemCoversTitle),
// since content with the banner no longer starts with the plain title.
const TARGET_BLANK_BANNER_PREFIX =
  /^\s*<[^>]*font[^>]*>\s*本题作答\s*[（(]\s*_?\s*问题\s*\d+\s*_?\s*[）)]\s*[：:]\s*<\/[^>]*font[^>]*>\s*/i

export type BlankOption = {
  id?: number
  label: string
  is_correct?: number
  sort_order?: number
  [key: string]: any
}

/** Keep multi-blank option order stable.
 * Prefer original order when sort_order repeats across groups (common import dirt).
 */
export function orderOptionsForBlanks<T extends BlankOption>(options: T[] | null | undefined): T[] {
  const opts = options ? [...options] : []
  if (opts.length <= 1) return opts
  const seen = new Set<number>()
  let unique = true
  for (const o of opts) {
    const so = o.sort_order ?? 0
    if (seen.has(so)) {
      unique = false
      break
    }
    seen.add(so)
  }
  if (!unique) return opts
  return opts.sort((a, b) => {
    const sa = a.sort_order ?? 0
    const sb = b.sort_order ?? 0
    if (sa !== sb) return sa - sb
    return (a.id ?? 0) - (b.id ?? 0)
  })
}

export function groupOptionsByBlank<T extends BlankOption>(
  options: T[] | null | undefined,
  blankSize = BLANK_SIZE,
): T[][] {
  const opts = orderOptionsForBlanks(options)
  if (!opts.length) return []
  const groups: T[][] = []
  for (let i = 0; i < opts.length; i += blankSize) {
    groups.push(opts.slice(i, i + blankSize))
  }
  return groups
}

export function blankCountFromOptions(optionCount: number, blankSize = BLANK_SIZE): number {
  if (optionCount <= 0) return 0
  return Math.ceil(optionCount / blankSize)
}

/**
 * One correct label per blank.
 * Handles dirty imports where every group marks the same multi-correct set
 * (e.g. both groups mark C+D => answer CD, not CC).
 */
export function correctLabelsByBlank<T extends BlankOption>(
  options: T[] | null | undefined,
  blankSize = BLANK_SIZE,
): string[] {
  const groups = groupOptionsByBlank(options, blankSize)
  if (!groups.length) return []
  const correctsPerGroup = groups.map((g) => g.filter((o) => o.is_correct === 1).map((o) => o.label))

  if (
    correctsPerGroup.length > 1 &&
    correctsPerGroup[0].length === correctsPerGroup.length &&
    correctsPerGroup[0].length > 1 &&
    correctsPerGroup.every(
      (g) => g.length === correctsPerGroup[0].length && g.every((lab, i) => lab === correctsPerGroup[0][i]),
    )
  ) {
    return correctsPerGroup[0]
  }

  return correctsPerGroup.map((g) => g[0] || '?')
}

export function correctAnswerFromOptions<T extends BlankOption>(
  options: T[] | null | undefined,
  blankSize = BLANK_SIZE,
): string {
  return correctLabelsByBlank(options, blankSize).join('')
}

/** 1-based blank number this question is targeting, if the stem has「本题作答（_问题N_）」. */
export function extractTargetBlankNumber(text?: string | null): number | null {
  if (!text) return null
  const m = text.match(TARGET_BLANK_PATTERN)
  if (!m) return null
  const n = parseInt(m[1], 10)
  return Number.isFinite(n) && n > 0 ? n : null
}

function blankLabel(n: number, answer?: string): string {
  if (answer) return `\uFF08${answer}\uFF09`
  return `\uFF08\u7B2C${n}\u7A7A\uFF09`
}

function currentBlankToken(n: number): string {
  return `@@CURRENTBLANK${n}@@`
}

/**
 * Mark the active blank in stem text before markdown render.
 *
 * Two shapes are supported:
 * 1) True multi-blank item (options.length > 4): highlight by option-group index.
 * 2) Split single-blank item with「本题作答（_问题N_）」: only highlight 问题N.
 */
export function annotateBlanksInStem(
  text: string,
  currentBlank: number,
  totalBlanks: number,
  answers: Record<number, string> = {},
): string {
  if (!text) return ''

  const targetBlank = extractTargetBlankNumber(text)
  if (targetBlank != null) {
    // 拆分题：横幅本身不渲染，空位处已有「第N空·正在作答」标记，
    // 这里只从题干正文识别各空并标注当前作答空位。
    const body = text.replace(TARGET_BLANK_BANNER_PREFIX, '')

    let sequential = 0
    const annotated = body.replace(BLANK_PATTERN, (match) => {
      const numMatch = match.match(/\u95EE\u9898\s*(\d+)/i)
      let n: number
      if (numMatch) {
        n = parseInt(numMatch[1], 10)
        if (n > sequential) sequential = n
      } else {
        n = ++sequential
      }
      if (n === targetBlank) return currentBlankToken(n)
      return blankLabel(n)
    })

    return annotated
  }

  // Classic multi-blank (multiple option groups on one question).
  // 单组选项(totalBlanks<=1)但题干仍含多个空位、且无横幅指定空位时，
  // 无法确定这 4 个选项对应哪一空，标注只会误导——历史脏数据里
  // 「拆分丢了横幅」的题一律被当成第 1 空，题干高亮的空位与选项错位。
  // 此时原样返回题干，由选项内容自行说明问的是哪一问。
  if (totalBlanks <= 1) {
    return text
  }

  let blankIndex = 0
  let detected = 0
  const annotated = text.replace(BLANK_PATTERN, (match) => {
    detected++
    const idx = blankIndex++
    // If totalBlanks is underestimated, still label extras sequentially.
    if (idx === currentBlank) return currentBlankToken(idx + 1)
    if (totalBlanks > 1 && idx >= totalBlanks) return match
    return blankLabel(idx + 1, answers[idx])
  })

  // If we found fewer blanks than option groups, keep original stem to avoid
  // mangling ordinary list parentheses like（1）（2）.
  if (totalBlanks > 1 && detected > 0 && detected < totalBlanks) {
    return text
  }
  return annotated
}

export function replaceBlankMarkers(html: string): string {
  if (!html) return html
  return html
    .replace(/@@CURRENTBLANK(\d+)@@/g, (_m, n: string) => {
      return `<mark class="current-blank-marker" data-blank="${n}">\u7B2C${n}\u7A7A\u00B7\u6B63\u5728\u4F5C\u7B54</mark>`
    })
    .replace(/@@CURRENT[_ ]?BLANK[_ ]?(\d+)@@/gi, (_m, n: string) => {
      return `<mark class="current-blank-marker" data-blank="${n}">\u7B2C${n}\u7A7A\u00B7\u6B63\u5728\u4F5C\u7B54</mark>`
    })
}

/** \u9898\u5E72(content)\u662F\u5426\u5DF2\u5B8C\u6574\u5305\u542B\u6807\u9898(title)\u6587\u672C\u3002
 * \u5BFC\u5165\u9898\u5E93\u4E2D\u7EA6 80% \u5355\u9009\u9898 title \u4E0E content \u5B8C\u5168\u76F8\u540C\u3001\u5C11\u6570 content \u4EE5 title \u5F00\u5934\uFF0C
 * \u9875\u9762\u540C\u65F6\u6E32\u67D3\u6807\u9898\u533A\u4E0E\u9898\u5E72\u533A\u65F6\u4F1A\u91CD\u590D\u663E\u793A\uFF0C\u9700\u8981\u636E\u6B64\u53BB\u91CD\u3002 */
export function stemCoversTitle(title?: string, content?: string): boolean {
  if (!title || !content) return false
  const t = title.trim()
  // Split-question stems carry a red banner prefix like
  // <font color='red'>本题作答（_问题2_）：</font>; strip it before comparing.
  const c = content.trim().replace(TARGET_BLANK_BANNER_PREFIX, '')
  return c === t || c.startsWith(t)
}

/** 列表页也高亮目标空，避免多空题看不出错的是哪一空 */
export function renderWrongStem(q?: { content?: string; title?: string } | null): string {
  if (!q) return ''
  const raw = q.content || q.title || ''
  if (!raw) return ''
  const target = extractTargetBlankNumber(raw)
  // 列表只读：有明确目标空就高亮它；真多空题（无目标横幅）不高亮某一空，避免误导
  const marked = target != null
    ? annotateBlanksInStem(raw, 0, 1, {})
    : raw
  return replaceBlankMarkers(renderMd(marked))
}
