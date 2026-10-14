/**
 * 粗略去掉 markdown / HTML 噪音，只用于列表卡片的摘要预览。
 * 与详情页的完整渲染（renderMd / sanitizeNoteHtml）无关，不要拿它做展示安全边界。
 */
export function plainNoteText(text: string): string {
  if (!text) return ''
  return text
    .replace(/<[^>]+>/g, ' ')                        // HTML 标签 -> 空格
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '[图片]')        // 图片 -> [图片]
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')           // 链接 -> 文字
    .replace(/[*_`>#~|-]/g, '')                       // markdown 强调/引用符
    .replace(/\s+/g, ' ')
    .trim()
}
