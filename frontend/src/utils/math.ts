// LaTeX 公式抽取与渲染。
//
// AI 解析 / 助教经常直接输出 LaTeX（\[...\]、\(...\)、$$...$$、$...$），
// 而 marked 只认 markdown：\[ 会被当成转义压成 [，\text{...} 原样留在正文里，
// 用户看到的就是一整行 LaTeX 源码。
//
// 因此必须在 marked 之前先把公式抠出来换成占位符，marked + DOMPurify 处理完之后
// 再把占位符替换成 KaTeX 生成的 HTML。KaTeX 输出是本模块的可信 HTML，
// 与 markdown.ts 里的图片占位符同理，不应再被 sanitize 过滤
// （DOMPurify 的 html profile 会把 MathML 节点整个剥掉）。

import katex from 'katex'

export interface MathSegment {
  placeholder: string
  tex: string
  display: boolean
}

export interface MathExtraction {
  text: string
  segments: MathSegment[]
}

// 占位符只用大写字母和数字：markdown 语法与 DOMPurify 都不会改动它。
const placeholderOf = (n: number): string => `KATEXMATHPLACEHOLDER${n}END`

function isBlank(ch: string | undefined): boolean {
  return ch === undefined || /\s/.test(ch)
}

/**
 * 把文本里的 LaTeX 公式替换成占位符，返回替换后的文本与公式列表。
 * 代码块 / 行内代码里的公式原样保留 —— 那是示例代码，不该渲染。
 */
export function extractMath(input: string): MathExtraction {
  const segments: MathSegment[] = []
  let out = ''
  let i = 0

  const push = (tex: string, display: boolean): void => {
    const seg = { placeholder: placeholderOf(segments.length), tex, display }
    segments.push(seg)
    out += seg.placeholder
  }

  while (i < input.length) {
    // 围栏代码块整体跳过
    const fence = input.startsWith('```', i) ? '```' : input.startsWith('~~~', i) ? '~~~' : ''
    if (fence) {
      const end = input.indexOf(fence, i + 3)
      const stop = end === -1 ? input.length : end + 3
      out += input.slice(i, stop)
      i = stop
      continue
    }

    // 行内代码跳过（只处理同一行内闭合的那种）
    if (input[i] === '`') {
      const nl = input.indexOf('\n', i + 1)
      const end = input.indexOf('`', i + 1)
      if (end !== -1 && (nl === -1 || end < nl)) {
        out += input.slice(i, end + 1)
        i = end + 1
        continue
      }
    }

    // $$...$$ 块级公式
    if (input.startsWith('$$', i)) {
      const end = input.indexOf('$$', i + 2)
      if (end !== -1) {
        const tex = input.slice(i + 2, end).trim()
        if (tex) {
          push(tex, true)
          i = end + 2
          continue
        }
      }
    }

    // \[...\] 块级公式
    if (input.startsWith('\\[', i)) {
      const end = input.indexOf('\\]', i + 2)
      if (end !== -1) {
        const tex = input.slice(i + 2, end).trim()
        if (tex) {
          push(tex, true)
          i = end + 2
          continue
        }
      }
    }

    // \(...\) 行内公式
    if (input.startsWith('\\(', i)) {
      const end = input.indexOf('\\)', i + 2)
      if (end !== -1) {
        const tex = input.slice(i + 2, end).trim()
        if (tex) {
          push(tex, false)
          i = end + 2
          continue
        }
      }
    }

    // $...$ 行内公式。沿用 Pandoc 的定界规则，避免把货币金额当成公式：
    //   - 开头的 $ 后不能是空白；
    //   - 结尾的 $ 前不能是空白、后不能是数字（因此「$5，还有 $10」不会被误判）；
    //   - 转义的 \$ 不算定界符。
    if (
      input[i] === '$' &&
      input[i - 1] !== '\\' &&
      input[i + 1] !== '$' &&
      !isBlank(input[i + 1])
    ) {
      let j = i + 1
      let closed = false
      while (j < input.length) {
        const ch = input[j]
        if (ch === '\n') break
        if (
          ch === '$' &&
          input[j - 1] !== '\\' &&
          !isBlank(input[j - 1]) &&
          !/^\d/.test(input[j + 1] ?? '')
        ) {
          push(input.slice(i + 1, j), false)
          i = j + 1
          closed = true
          break
        }
        j++
      }
      if (closed) continue
    }

    out += input[i]
    i++
  }

  return { text: out, segments }
}

/**
 * 把 extractMath 产生的占位符替换成 KaTeX 渲染出的 HTML。
 * 注意用 split/join 而不是 String.replace：KaTeX 输出里可能出现 `$`，
 * 会被 replace 的替换字符串当成 `$&` 之类的特殊模式解析。
 */
export function renderMathHtml(html: string, segments: readonly MathSegment[]): string {
  if (!segments.length) return html
  let out = html
  for (const seg of segments) {
    const rendered = katex.renderToString(seg.tex, {
      displayMode: seg.display,
      throwOnError: false, // 公式写错时输出红色错误片段，不中断整段渲染
      strict: 'ignore', // 忽略 \text{} 里中文等非 ASCII 字符的告警
      trust: false, // 禁用 \href / \html 等可注入命令
    })
    out = out.split(seg.placeholder).join(rendered)
  }
  return out
}
