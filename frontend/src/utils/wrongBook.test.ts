import { describe, expect, it } from 'vitest'
import {
  isDueReview,
  matchKeyword,
  questionTypeLabel,
  sortByLatestWrong,
  summarizeStem,
} from '@/utils/wrongBook'
import type { WrongQuestion } from '@/types'

describe('questionTypeLabel', () => {
  it('映射 1-4 为标准题型名', () => {
    expect(questionTypeLabel(1)).toBe('单选')
    expect(questionTypeLabel(2)).toBe('多选')
    expect(questionTypeLabel(3)).toBe('案例')
    expect(questionTypeLabel(4)).toBe('论文')
  })
  it('未知题型回退「未知」', () => {
    expect(questionTypeLabel(0)).toBe('未知')
  })
})

describe('isDueReview', () => {
  const now = new Date('2026-09-17T12:00:00+08:00')
  it('到期时间早于/等于当前时刻 → true', () => {
    expect(isDueReview({ next_review_time: '2026-09-17T11:00:00+08:00' }, now)).toBe(true)
    expect(isDueReview({ next_review_time: '2026-09-17T12:00:00+08:00' }, now)).toBe(true)
  })
  it('未到期 / 缺失 / 空串 → false', () => {
    expect(isDueReview({ next_review_time: '2026-09-18T10:00:00+08:00' }, now)).toBe(false)
    expect(isDueReview({ next_review_time: '' }, now)).toBe(false)
    expect(isDueReview({ next_review_time: null }, now)).toBe(false)
  })
  it('无法解析的时间串 → false', () => {
    expect(isDueReview({ next_review_time: 'not-a-date' }, now)).toBe(false)
  })
})

describe('matchKeyword', () => {
  const wq = {
    question: {
      title: '流水线技术的主要特点',
      content: '关于流水线吞吐率的计算，下列说法正确的是……',
      knowledge_point: '计算机系统>流水线',
      options: [
        { content: '吞吐率与流水线段数成正比' },
        { content: '流水线建立时间不随段数变化' },
      ],
    } as any,
  }
  it('命中题干/选项，大小写不敏感；知识点不参与检索', () => {
    expect(matchKeyword(wq, '流水线')).toBe(true)
    expect(matchKeyword(wq, '吞吐率')).toBe(true)
    expect(matchKeyword(wq, '计算机系统')).toBe(false)
    expect(matchKeyword(wq, 'FLOW')).toBe(false)
    expect(matchKeyword({ question: { title: 'ABC', content: '', options: [] } as any }, 'abc')).toBe(true)
  })
  it('空关键词恒 true', () => {
    expect(matchKeyword(wq, '')).toBe(true)
    expect(matchKeyword(wq, '   ')).toBe(true)
  })
  it('题目已删除（question 缺失）→ false', () => {
    expect(matchKeyword({ question: null }, '流水线')).toBe(false)
  })
})

describe('sortByLatestWrong', () => {
  const mk = (id: number, updated_at?: string): WrongQuestion => ({
    id,
    question_id: id,
    user_id: 1,
    wrong_count: 1,
    correct_count: 0,
    next_review_time: '',
    interval_days: 1,
    ease_factor: 2,
    status: 0,
    question: null as any,
    ...(updated_at ? { updated_at } : {}),
  })
  it('按 updated_at 降序：最新答错在最前', () => {
    const ids = sortByLatestWrong([
      mk(1, '2026-09-16T10:00:00+08:00'),
      mk(2, '2026-09-17T10:00:00+08:00'),
      mk(3, '2026-09-15T10:00:00+08:00'),
    ]).map((w) => w.id)
    expect(ids).toEqual([2, 1, 3])
  })
  it('缺失 updated_at 排最后；同 updated_at 按 id 降序兜底', () => {
    const ids = sortByLatestWrong([
      mk(1, '2026-09-17T10:00:00+08:00'),
      mk(2),
      mk(3, '2026-09-17T10:00:00+08:00'),
    ]).map((w) => w.id)
    expect(ids).toEqual([3, 1, 2])
  })
  it('不改变入参数组（返回新数组）', () => {
    const arr = [mk(2, '2026-09-17T10:00:00+08:00'), mk(1, '2026-09-16T10:00:00+08:00')]
    sortByLatestWrong(arr)
    expect(arr.map((w) => w.id)).toEqual([2, 1])
  })
})

describe('summarizeStem', () => {
  it('有非空 title 用 title', () => {
    expect(summarizeStem({ title: '螺旋模型的特点', content: '很长的题干……' } as any)).toBe('螺旋模型的特点')
  })
  it('无 title 取 content 第一个非空行', () => {
    expect(summarizeStem({ title: '', content: '\n\n第一行题干\n第二行' } as any)).toBe('第一行题干')
  })
  it('q 为空 / 全空 → 空串', () => {
    expect(summarizeStem(null)).toBe('')
    expect(summarizeStem({ title: '  ', content: '' } as any)).toBe('')
  })
  it('undefined 输入返回空串', () => {
    expect(summarizeStem(undefined)).toBe('')
  })

  it('空白 title 回落 content 首段', () => {
    expect(summarizeStem({ title: '   ', content: '真实内容' } as any)).toBe('真实内容')
  })

  it('title 为纯序号「第 N 题」时不拿序号当摘要，回落 content 首段', () => {
    expect(summarizeStem({ title: '第9题', content: '真正的题干第一行\n第二行' } as any)).toBe('真正的题干第一行')
  })
  it('content 首段含富文本标签时剥离标签再作摘要', () => {
    expect(summarizeStem({ title: '第13题', content: "<font color='blue'>（<u> 问题1</u>）</font>。\n后续" } as any)).toBe('（ 问题1）。')
  })
})
