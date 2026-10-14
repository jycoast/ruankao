// 论文范文清洗与结构化解析工具。
// 后台编辑使用 Markdown；历史 OCR 纯文本仍可能混入页码/水印噪声。
// Markdown 内容必须保留换行；OCR 纯文本才把段内折行合并。

/** 粗判是否为 Markdown（后台编辑后的内容），避免 OCR 清洗把列表/代码块拼坏。 */
function looksLikeMarkdown(text: string): boolean {
  return /(^|\n)\s{0,3}(#{1,6}\s|```|~~~|\|?.+\|.+\||!\[[^\]]*\]\([^)]+\)|\[[^\]]+\]\([^)]+\))/m.test(text)
    || /(^|\n)\s{0,3}([-*+] |\d+\. |> )/m.test(text)
    || /(\*\*[^*]+\*\*|__[^_]+__|`[^`]+`)/.test(text)
}

/** 去掉 OCR 噪声；Markdown 保留换行，纯文本才合并段内折行。 */
export function cleanEssayText(raw?: string | null): string {
  if (!raw) return ''
  let text = raw

  // 去 OCR 页码标记：--- PAGE 12 (xxx) ---
  text = text.replace(/---\s*PAGE\s*\d+\s*(\([^)]*\))?\s*---\n?/gi, '')
  // 去讲师水印行（如「软考试译葫芦老师…」整行）
  text = text.replace(/^.*葫芦老师.*$/gm, '')
  text = text.replace(/^.*诸葛老师预测.*$/gm, '')

  // 折叠 3+ 空行，避免过大空隙
  text = text.replace(/\n{3,}/g, '\n\n')

  if (looksLikeMarkdown(text)) {
    // Markdown：只去行尾空白，保留行首缩进与单换行
    text = text
      .split('\n')
      .map((l) => l.replace(/[ \t]+$/g, ''))
      .join('\n')
    return text.trim()
  }

  // OCR 纯文本：逐行 trim，段内单换行合并（OCR 折行）
  const lines = text.split('\n').map((l) => l.trim())
  text = lines.join('\n')
  text = text.replace(/\n{3,}/g, '\n\n')
  text = text
    .split('\n\n')
    .map((block) => block.trim().replace(/\n/g, ' ').replace(/ +/g, ' '))
    .filter((b) => b)
    .join('\n\n')

  return text.trim()
}

export interface EssaySection {
  id: string
  title: string
  body: string
}

// 结构标题识别（论文范文固定骨架；兼容「摘要：」与 Markdown「## 摘要」）
const SECTION_PATTERNS: { test: RegExp; label: (m: string) => string }[] = [
  { test: /^(?:#{1,6}\s*)?试题\b/, label: () => '试题' },
  { test: /^(?:#{1,6}\s*)?(找准核心论点|核心论点)/, label: () => '核心论点' },
  { test: /^(?:#{1,6}\s*)?问题\s*[一二三四五1-9]\s*要点[:：]?/, label: (m) => m.replace(/^#{1,6}\s*/, '').replace(/[:：]\s*$/, '') },
  { test: /^(?:#{1,6}\s*)?(理论素材准备|理论素材)/, label: () => '理论素材准备' },
  { test: /^(?:#{1,6}\s*)?(合格范文参考|范文参考|参考范文)/, label: () => '合格范文参考' },
  { test: /^(?:#{1,6}\s*)?摘要[:：]?/, label: () => '摘要' },
  { test: /^(?:#{1,6}\s*)?正文[:：]?/, label: () => '正文' },
]

/**
 * 将清洗后的论文文本按结构标记切分为多个 section。
 * 每段的 body 为原始（清洗后）markdown 文本，由调用方用 renderMd 渲染。
 * 无任何结构标记时回退为单段「正文」。
 */
export function parseEssaySections(clean: string): EssaySection[] {
  if (!clean) return []
  const paragraphs = clean.split('\n\n')

  const sections: EssaySection[] = []
  let current: EssaySection | null = null

  const pushBody = (line: string) => {
    if (!current) {
      current = { id: 'sec-0', title: '正文', body: '' }
      sections.push(current)
    }
    current.body += (current.body ? '\n\n' : '') + line
  }

  for (const para of paragraphs) {
    const firstLine = para.split('\n')[0].trim()
    // 去掉 Markdown 标题前缀后再做骨架匹配，便于「## 摘要」这类写法
    const normalized = firstLine.replace(/^#{1,6}\s*/, '')
    const matched = SECTION_PATTERNS.find((p) => p.test.test(firstLine) || p.test.test(normalized))
    if (matched) {
      current = {
        id: `sec-${sections.length}`,
        title: matched.label(normalized || firstLine).slice(0, 20),
        body: '',
      }
      sections.push(current)
      // 标题行之后若同段还有内容，一并纳入 body
      const rest = para.split('\n').slice(1).join('\n').trim()
      if (rest) current.body = rest
    } else {
      pushBody(para)
    }
  }

  return sections
}

/** 生成列表卡片用的纯文本摘要（去 markdown 符号）。 */
export function essayExcerpt(raw?: string | null, len = 80): string {
  const clean = cleanEssayText(raw)
  if (!clean) return ''
  const flat = clean
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '图')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/[#>*_`~\-]+/g, '')
    .replace(/\n+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
  return flat.length > len ? flat.slice(0, len) + '…' : flat
}
