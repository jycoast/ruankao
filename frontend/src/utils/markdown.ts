import { marked } from 'marked'
import DOMPurify from 'dompurify'

// GitHub-flavored markdown; single newlines become <br> (matches exam text layout)
marked.use({ gfm: true, breaks: true })

const IMG_PLACEHOLDER =
  '<span class="inline-block bg-yellow-50 border border-yellow-200 rounded px-2 py-1 text-sm text-yellow-700">📷 图片暂未加载</span>'

// 题干、投稿论文、知识点等内容用户可写入且经 v-html 渲染，
// marked 默认放行原始 HTML，不过滤即为存储型 XSS。出口统一 sanitize。
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A' && node.hasAttribute('href')) {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer')
  }
})

// 限制 style 属性：仅放行 color / background-color。
// 知识点"红色重点"用的就是 style="color:red"，默认放行；但默认 DOMPurify 对
// position/left/top 等布局属性也放行（实测 3.4.12），有 clickjack 风险。
// 用 uponSanitizeAttribute 主动收口到视觉属性，丢弃其他声明或整条 style。
DOMPurify.addHook('uponSanitizeAttribute', (_node, ev) => {
  if (ev.attrName !== 'style') return
  const raw = ev.attrValue || ''
  const kept = raw
    .split(';')
    .map((s) => s.trim())
    .filter((s) => /^(color|background-color)\s*:/i.test(s))
    .join('; ')
  if (kept) ev.attrValue = kept
  else ev.keepAttr = false
})

function sanitize(html: string): string {
  // 先净化再插入占位符：占位符是本模块的可信 HTML，不应被当作用户输入过滤
  return DOMPurify.sanitize(html, { USE_PROFILES: { html: true } }).replace(
    /\[图片\]/g,
    IMG_PLACEHOLDER
  )
}

/** Render markdown (headings, tables, code, images, bold, lists, blockquote…) to HTML for v-html. */
export function renderMd(text?: string | null): string {
  if (!text) return ''
  return sanitize(marked.parse(text) as string)
}

/** Inline markdown only (no block <p> wrapping) — for option text that sits inline. */
export function renderMdInline(text?: string | null): string {
  if (!text) return ''
  return sanitize(marked.parseInline(text) as string)
}

// AI 偶尔会把整篇范文包进 ```markdown ... ``` 里再吐出来。marked 会把整篇
// 渲染成一整个 <pre><code>，用户看到的就成了 Markdown 源码而不是排版 ——
// 症状是「生成的论文显示的是原文件语法」。
//
// 指令层（sysGenEssay）已经要求不要包裹，但模型是否遵守是概率性的：
// 实测旧指令 5 次里中 1 次，加指令后 5 次 0 次 —— 仍属概率，不能只靠它。
// 这里做确定性收口，保证无论模型怎么写，渲染出的都是排版后的正文。
//
// 只处理「整篇被包裹」，正文里正常的代码块原样保留：若首行是围栏、中途另有围栏
// 而结尾又不是围栏，说明这篇本来就是以代码块开头，原样返回。
export function stripOuterCodeFence(text?: string | null): string {
  if (!text) return ''
  const lines = text.split('\n')
  const isFence = (s: string) => /^\s*```/.test(s)
  if (!isFence(lines[0] ?? '')) return text

  const fenceCount = lines.filter(isFence).length
  let lastIdx = -1
  for (let i = lines.length - 1; i >= 0; i--) {
    if (lines[i].trim() !== '') {
      lastIdx = i
      break
    }
  }
  const closedAtEnd = lastIdx >= 0 && isFence(lines[lastIdx])

  // fenceCount === 1：流式进行中，闭合围栏还没吐出来
  // closedAtEnd：首尾都是围栏，整篇被包裹
  if (fenceCount !== 1 && !closedAtEnd) return text

  const end = closedAtEnd ? lastIdx : lines.length
  return lines.slice(1, end).join('\n').replace(/^\s*\n/, '')
}

// 纯文本摘要：去掉 markdown 语法，给表格/列表的「一眼认题」列用。
// 图片 ![alt](url) 与 [图片] 占位符统一抽成字面量「图」，避免一行被 base64 撑爆。
export function stripMd(text?: string | null): string {  if (!text) return ''
  return text
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '图')
    .replace(/\[图片\]/g, '图')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1') // 链接只留锚文本
    .replace(/[*_`>~#\-]+/g, '') // 强调/引用/标题标记
    .replace(/\s+/g, ' ')
    .trim()
}

// 笔记 HTML sanitize：双保险链的前端一层。
//
// 与 renderMd 不同：
//   - 入参是 TipTap getHTML() 的 raw HTML（不是 markdown），不走 marked；
//   - <img src> 仅放行 https:// 与 data:image/...，与后端 isSafeImgSrc 对齐；
//   - 严格白名单 tag/attr，禁掉 class/style/on* 等所有可执行/可劫持属性。
//
// 后端 question_note.Save 仍会再走一遍 sanitizeNoteHTML（regex 兜底），
// 所以这里不必拦下"被后端代理注入"的攻击——只挡前端富文本编辑器的脏数据即可。
const NOTE_ALLOWED_TAGS = [
  'p', 'br', 'strong', 'em', 'u', 's',
  'code', 'pre', 'blockquote',
  'ul', 'ol', 'li',
  'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
  'a', 'img',
]
const NOTE_ALLOWED_ATTR = ['href', 'src', 'alt', 'title', 'target', 'rel']
// 自定义 ALLOWED_URI_REGEXP：仅放行 https:// 与 data:image/...，拒绝 javascript: / vbscript:
// / data:text/html / file:// / ftp:// 等一切非白名单 scheme。与后端 isSafeImgSrc 对齐。
const NOTE_ALLOWED_URI = /^(?:https:\/\/|data:image\/)/i

export function sanitizeNoteHtml(html: string): string {
  if (!html) return ''
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: NOTE_ALLOWED_TAGS,
    ALLOWED_ATTR: NOTE_ALLOWED_ATTR,
    ALLOWED_URI_REGEXP: NOTE_ALLOWED_URI,
    // 拒掉所有 data-* 属性，避免通过 data-onclick / data-handler 之类注入脚本载体
    ALLOW_DATA_ATTR: false,
    // 关闭 DOMPurify 自身对 DOM 的反射（仅输出字符串）
    RETURN_DOM: false,
    RETURN_DOM_FRAGMENT: false,
  })
}

/** 剥掉 HTML 标签取纯文本（&nbsp; 归一为空格、连续空白折叠）。给反馈字数校验 / 管理表格摘要用。 */
export function stripHtml(html?: string | null): string {
  if (!html) return ''
  return html
    .replace(/<[^>]*>/g, ' ')
    .replace(/&nbsp;/g, ' ')
    .replace(/&amp;/g, '&')
    .replace(/\s+/g, ' ')
    .trim()
}

// 反馈内容展示的统一安全出口：
//   - 历史反馈（textarea 时代）存的是纯文本，可能含任意字符 —— 先转义再 \n→<br>，
//     不然 v-html 直插用户文本就是存储型 XSS，换行也会被折叠掉；
//   - 新反馈是富文本编辑器吐出的 HTML —— 走 sanitizeNoteHtml（白名单 + https/data:image）。
// 靠「是否像标签」分流，而不是存在多少年，因此两条分支对各自的数据都稳。
const HTML_TAG_START = /<(?:p|div|br|strong|em|u|s|code|pre|blockquote|ul|ol|li|h[1-6]|a|img)\b/i

export function feedbackContentHtml(content?: string | null): string {
  if (!content) return ''
  if (!HTML_TAG_START.test(content)) {
    return content
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/\n/g, '<br>')
  }
  return sanitizeNoteHtml(content)
}
