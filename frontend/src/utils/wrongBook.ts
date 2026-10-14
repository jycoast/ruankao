import type { Question, WrongQuestion } from '@/types'

export const QUESTION_TYPE_LABELS: Record<number, string> = {
  1: '单选',
  2: '多选',
  3: '案例',
  4: '论文',
}

export function questionTypeLabel(qType: number): string {
  return QUESTION_TYPE_LABELS[qType] || '未知'
}

/**
 * 待复习判定：next_review_time 存在、可解析、且不晚于 now。
 * 与后端 FindDueReview 的 `next_review_time <= NOW()` 语义一致；
 * 参数故意用可空结构类型——WrongQuestion 的 question 声明非空但运行时可 null，
 * 且 next_review_time 理论上是 string，运行期可能缺/null。
 */
export function isDueReview(wq: { next_review_time?: string | null }, now: Date): boolean {
  if (!wq.next_review_time) return false
  const due = new Date(wq.next_review_time)
  if (Number.isNaN(due.getTime())) return false
  return due.getTime() <= now.getTime()
}

/**
 * 关键词本地检索：题干 + 选项内容，大小写不敏感子串匹配（不按分类/知识点检索）。
 * 空关键词不过滤（恒 true）；题目已删除（question 缺失）永不命中。
 */
export function matchKeyword(wq: { question?: Question | null }, keyword: string): boolean {
  const k = keyword.trim().toLowerCase()
  if (!k) return true
  const q = wq.question
  if (!q) return false
  const haystack = [q.title, q.content, ...(q.options || []).map((o) => o.content)]
    .filter(Boolean)
    .join(' ')
    .toLowerCase()
  return haystack.includes(k)
}

/**
 * 按「最新答错」排序：updated_at 降序（后端每次答错都会刷新 updated_at）。
 * 缺失/空更新时间视为最旧排最后；同时间按 id 降序兜底。返回新数组，不改入参。
 */
export function sortByLatestWrong(items: WrongQuestion[]): WrongQuestion[] {
  return [...items].sort((a, b) => {
    const ta = a.updated_at || ''
    const tb = b.updated_at || ''
    if (ta !== tb) return ta < tb ? 1 : -1
    return b.id - a.id
  })
}

/** 纯序号标题（「第9题」这类，导入来源常见）：不具描述性，视同无标题，回落 content。 */
const ORDINAL_TITLE_RE = /^第\s*[0-9一二三四五六七八九十百]+\s*题$/

/** 剥掉富文本标签，摘要行不出现 <font> 等原始标签。 */
function stripHtmlTags(s: string): string {
  return s.replace(/<[^>]*>/g, '')
}

/** 单行摘要：title 有描述性用 title；否则 content 第一个非空行；都没有返回空串。 */
export function summarizeStem(q: Question | null | undefined): string {
  if (!q) return ''
  if (q.title && q.title.trim() && !ORDINAL_TITLE_RE.test(q.title.trim())) return q.title
  const firstLine = (q.content || '')
    .split('\n')
    .map((s) => s.trim())
    .find(Boolean)
  return firstLine ? stripHtmlTags(firstLine) : ''
}
