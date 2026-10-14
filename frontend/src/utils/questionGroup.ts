export interface QuestionGroup {
  // 闭区间，均为全局问下标（0 起）
  start: number
  end: number
  title: string
  count: number
}

// 归一化前缀长度：与后端校验所用 SQL 的 LEFT(..., 40) 一致
const STEM_LIMIT = 40

/**
 * 大题归属的归一化前缀：正文以第一个「问题」切开后的左半段，
 * 去掉全部空白后取前 40 个码点。
 *
 * 与校验所用 SQL 等价（已实测本库 MySQL 8 的 ICU 正则会把全角空格 U+3000、
 * 不换行空格 U+00A0 与 ASCII 空格一并剥掉，与 JS 的 \s 一致）：
 *   LEFT(REGEXP_REPLACE(SUBSTRING_INDEX(content, '问题', 1), '[[:space:]]+', ''), 40)
 * 用 Array.from 按码点切，与 MySQL LEFT 的字符语义一致（非 BMP 字符下才分叉）。
 */
function stemKey(content: string): string {
  return Array.from(content.split('问题')[0].replace(/\s+/g, ''))
    .slice(0, STEM_LIMIT)
    .join('')
}

/**
 * 把整卷题目按大题分组。
 *
 * 换组条件是两个信号的并集，缺一不可：
 *   - title 与上一问不同；或
 *   - 归一化前缀与上一问不同。
 * 只用 title 会漏切「相邻两道大题同名」的情况（title 是知识域标签，会重名）；
 * 只用前缀会误切「多道大题共用开头」的情况（比如都从「阅读以下关于数据库缓存架构的叙述」起）。
 */
export function groupQuestions(
  questions: { title?: string; content?: string }[],
): QuestionGroup[] {
  const groups: QuestionGroup[] = []
  questions.forEach((q, i) => {
    const prev = questions[i - 1]
    const isNew =
      !prev ||
      (q.title ?? '') !== (prev.title ?? '') ||
      stemKey(q.content ?? '') !== stemKey(prev.content ?? '')
    if (isNew) {
      groups.push({ start: i, end: i, title: q.title ?? '', count: 1 })
    } else {
      const last = groups[groups.length - 1]
      last.end = i
      last.count += 1
    }
  })
  return groups
}

const CN_NUMERALS = ['一', '二', '三', '四', '五', '六', '七', '八', '九', '十']

/** 大题序号文案。index 为组下标（0 起）；1–10 用中文数字，超过 10 回退阿拉伯数字。 */
export function groupLabel(index: number): string {
  return `试题${CN_NUMERALS[index] ?? String(index + 1)}`
}
