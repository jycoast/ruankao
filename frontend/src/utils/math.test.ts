import { describe, expect, it } from 'vitest'
import { extractMath, renderMathHtml } from '@/utils/math'

// AI 输出的公式有两种来源：\$...\$ / \$\$...\$\$（常见）与 \[...\] / \(...\)（模型也常写）。
// 抽取必须在 marked 之前完成，否则反斜杠会被 markdown 当转义吃掉。

describe('extractMath', () => {
  it('识别 \\[ ... \\] 块级公式（用户反馈的实际案例）', () => {
    const input = String.raw`[ \text{最大拖延时间} = 16 - (3 + 4 + 2 + 3) - 2 = 2 ]`
    // 用户反馈里外层方括号是 LaTeX 定界符 \[ \] 被渲染掉之后的样子，
    // 这里按模型真实输出（带反斜杠）来测
    const real = String.raw`\[ \text{最大拖延时间} = 16 - (3 + 4 + 2 + 3) - 2 = 2 \]`
    const { text, segments } = extractMath(real)
    expect(segments).toHaveLength(1)
    expect(segments[0].display).toBe(true)
    expect(segments[0].tex).toBe('\\text{最大拖延时间} = 16 - (3 + 4 + 2 + 3) - 2 = 2')
    expect(text).not.toContain('\\text')
    expect(text).toContain(segments[0].placeholder)
    // 普通文本（没有反斜杠的方括号）不应被当作公式
    expect(extractMath(input).segments).toHaveLength(0)
  })

  it('识别 $$ ... $$ 块级公式', () => {
    const { text, segments } = extractMath('电功率：$$P = UI = \\frac{U^2}{R}$$')
    expect(segments).toHaveLength(1)
    expect(segments[0].display).toBe(true)
    expect(segments[0].tex).toBe('P = UI = \\frac{U^2}{R}')
    expect(text).toContain('电功率：')
  })

  it('识别 \\( ... \\) 与 $ ... $ 行内公式', () => {
    const a = extractMath(String.raw`流量 \(Q = \lambda W\) 的推导`)
    expect(a.segments).toHaveLength(1)
    expect(a.segments[0].display).toBe(false)
    expect(a.segments[0].tex).toBe('Q = \\lambda W')

    const b = extractMath('当 $a+b=c$ 时成立')
    expect(b.segments).toHaveLength(1)
    expect(b.segments[0].display).toBe(false)
    expect(b.segments[0].tex).toBe('a+b=c')
    expect(b.text).toBe('当 KATEXMATHPLACEHOLDER0END 时成立')
  })

  it('多个公式按出现顺序编号', () => {
    const { segments } = extractMath('$a$ 和 $b$ 还有 $$c$$')
    expect(segments.map((s) => s.tex)).toEqual(['a', 'b', 'c'])
    expect(segments.map((s) => s.placeholder)).toEqual([
      'KATEXMATHPLACEHOLDER0END',
      'KATEXMATHPLACEHOLDER1END',
      'KATEXMATHPLACEHOLDER2END',
    ])
  })

  it('货币金额不当成公式', () => {
    for (const input of ['价格是 $5，还有 $10', '总计 $20,000 元', '单价 $5 元 / 件']) {
      expect(extractMath(input).segments).toHaveLength(0)
    }
  })

  it('未闭合的公式保持原样，不吞掉后文', () => {
    const input = '这里有未闭合的 $a+b 和后续文字'
    const { text, segments } = extractMath(input)
    expect(segments).toHaveLength(0)
    expect(text).toBe(input)
  })

  it('围栏代码块与行内代码里的公式保持原样', () => {
    const fenced = '```tex\n$$x^2$$\n```\n正文 $y$'
    const a = extractMath(fenced)
    expect(a.segments).toHaveLength(1)
    expect(a.segments[0].tex).toBe('y')
    expect(a.text).toContain('$$x^2$$')

    const inlineCode = '写作 `$x$` 表示公式，实际用 $y$'
    const b = extractMath(inlineCode)
    expect(b.segments).toHaveLength(1)
    expect(b.segments[0].tex).toBe('y')
    expect(b.text).toContain('`$x$`')
  })

  it('转义的 \\$ 不当作公式定界符', () => {
    expect(extractMath(String.raw`价格 \$5 与 \$10`).segments).toHaveLength(0)
  })
})

describe('renderMathHtml', () => {
  it('把占位符替换为 KaTeX 输出的 HTML', () => {
    const { text, segments } = extractMath('$a+b=c$')
    const html = renderMathHtml(`<p>${text}</p>`, segments)
    expect(html).toContain('class="katex"')
    expect(html).not.toContain('KATEXMATHPLACEHOLDER')
  })

  it('非法公式不抛异常（throwOnError: false）', () => {
    const { text, segments } = extractMath('$\\frac{1}$')
    expect(() => renderMathHtml(`<p>${text}</p>`, segments)).not.toThrow()
  })

  it('没有公式时原样返回', () => {
    expect(renderMathHtml('<p>纯文本</p>', [])).toBe('<p>纯文本</p>')
  })
})
