import type { Question } from '@/types'

// 「一键生成范文」从真题跳到 AI 生成范文页的交接。
//
// 为什么走 sessionStorage 而不是 URL query：要点可能很长 —— 生产库里单道论文题的
// 写作要点实测 5000+ 字符，还常带 Markdown 表格，塞 query 会被截断。
// 写入方：题目详情页、背题模式；读取方：views/ai/GenerateEssay.vue，读完即删
// （否则用户下次自己进生成页，还会被上次的题面占着输入框）。
const PREFILL_KEY = 'essay-prefill'

// 论文题专属：一键生成范文
// （本文件只做「从真题跳去生成页」的交接，不含生成逻辑）

export const GENERATE_ESSAY_PATH = '/essays/ai-generate'

export interface EssayPrefill {
  topic: string
  points: string
}

export function setEssayPrefill(p: EssayPrefill) {
  sessionStorage.setItem(PREFILL_KEY, JSON.stringify(p))
}

/** 读取并清除交接数据。没有、或数据损坏时返回 null，调用方按「没有预填」处理，不打断用户。 */
export function takeEssayPrefill(): EssayPrefill | null {
  const raw = sessionStorage.getItem(PREFILL_KEY)
  if (!raw) return null
  sessionStorage.removeItem(PREFILL_KEY)
  try {
    const parsed = JSON.parse(raw)
    if (typeof parsed?.topic !== 'string') return null
    return {
      topic: parsed.topic,
      points: typeof parsed.points === 'string' ? parsed.points : '',
    }
  } catch {
    return null
  }
}

// 论文题的「写作要点」存在哪个字段，各科目不一致（生产库实测，question_type=4）：
//   架构设计师(1)  92/147 在 analysis，其余只有 content
//   系统分析师(3)   0/58  在 analysis，58/58 在 explanation
//   项目管理师(4)   0/74  在 analysis，74/74 在 explanation
// 所以不能只取 analysis：那样后两个科目会把整道试题正文当成「要点」喂进去，
// AI 就会去逐条论述题干本身。三级回退，最后兜到 content ——
// 它是试题正文，里面本来就有「从以下三个方面进行论述」那几条要求，比空着强。
export function essayPointsOf(q: Pick<Question, 'analysis' | 'explanation' | 'content'>): string {
  return q.analysis || q.explanation || q.content || ''
}

// 题目名不能直接用 title：生产库 279 道论文题里有 102 道 title 是「第1题」这种占位符
// （架构设计师 56 道、系统分析师 46 道），直接带过去，用户看到主题栏写着「第1题」。
// 真正的论题在 content 里，两种常见写法：
//   「请围绕“论面向方面的编程技术及其应用（AOP）”论题，依次从以下三个方面进行论述。」
//   content 首行直接就是「论模型驱动架构在系统开发中的应用」
// 先认引号里的论题，再认首行；都认不出就退回 title —— 宁可带个占位符，也别塞一段正文。
const PLACEHOLDER_TITLE = /^第[0-9一二三四五六七八九十]+题$/

/** 只有 title 是占位符时才去 content 里抠；正常标题原样返回。 */
export function essayTopicOf(q: Pick<Question, 'title' | 'content'>): string {
  const title = (q.title || '').trim()
  if (!PLACEHOLDER_TITLE.test(title)) return title

  const content = q.content || ''
  const quoted = content.match(/请围绕[“"]([^”"]{4,80})[”"]/)
  if (quoted) return quoted[1].trim()

  // 首行可能是「题目1：X」「试题一 论Y」，前缀要去掉；用 <br> 当行尾的老数据也要断行
  const head = content.split(/\n|<br\s*\/?>/i).map((s) => s.trim()).find(Boolean) || ''
  const stripped = head
    .replace(/^题目\s*\d+\s*[：:、.]?\s*/, '')
    .replace(/^试题\s*[一二三四五六七八九十\d]+\s*[、.．:：]?\s*/, '')
    .trim()
  // 首行是正文段落（不以「论」开头、又很长）时不能拿来当主题，退回 title
  if (/^论/.test(stripped) && stripped.length <= 60) return stripped

  return title
}
