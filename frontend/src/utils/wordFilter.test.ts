import { describe, expect, it } from 'vitest'
import { filterWords, pickSavedIndex, positionKey } from '@/utils/wordFilter'

interface W {
  word: string
  category: string
}

// 三种分类、5 个词，足够覆盖「分类 × 掌握状态」的所有组合
const WORDS: W[] = [
  { word: 'architecture', category: '架构设计' },
  { word: 'component', category: '架构设计' },
  { word: 'scalability', category: '质量属性' },
  { word: 'latency', category: '质量属性' },
  { word: 'agile', category: '软件工程' },
]

const masteredOf = (...words: string[]) => new Set(words)
const words = (list: W[]) => list.map((w) => w.word)

describe('filterWords —— 分类 × 掌握状态', () => {
  it("category = '全部' + mastery = 'all'：原样返回", () => {
    const out = filterWords(WORDS, { category: '全部', mastery: 'all', mastered: new Set() })
    expect(words(out)).toEqual(['architecture', 'component', 'scalability', 'latency', 'agile'])
  })

  it("category = '全部'：只按掌握状态过滤", () => {
    const out = filterWords(WORDS, {
      category: '全部',
      mastery: 'unmastered',
      mastered: masteredOf('component', 'latency'),
    })
    expect(words(out)).toEqual(['architecture', 'scalability', 'agile'])
  })

  it('具体分类 + all：等价于只按分类过滤', () => {
    const out = filterWords(WORDS, { category: '质量属性', mastery: 'all', mastered: new Set() })
    expect(words(out)).toEqual(['scalability', 'latency'])
  })

  it('分类与掌握状态取交集', () => {
    const out = filterWords(WORDS, {
      category: '架构设计',
      mastery: 'unmastered',
      mastered: masteredOf('architecture'),
    })
    expect(words(out)).toEqual(['component'])
  })

  it('mastered：只保留已掌握的词', () => {
    const out = filterWords(WORDS, {
      category: '全部',
      mastery: 'mastered',
      mastered: masteredOf('architecture', 'agile'),
    })
    expect(words(out)).toEqual(['architecture', 'agile'])
  })

  it('空词库 → 空数组', () => {
    expect(filterWords([], { category: '全部', mastery: 'mastered', mastered: masteredOf('architecture') })).toEqual([])
  })

  it('筛选结果为空 → 空数组，不抛错', () => {
    expect(
      words(filterWords(WORDS, { category: '架构设计', mastery: 'mastered', mastered: new Set() })),
    ).toEqual([])
    expect(
      words(filterWords(WORDS, { category: '不存在的分类', mastery: 'all', mastered: new Set() })),
    ).toEqual([])
  })

  it('保持入参顺序', () => {
    const out = filterWords(WORDS, { category: '全部', mastery: 'all', mastered: new Set() })
    expect(words(out)).toEqual(words(WORDS))
  })

  it('不修改入参', () => {
    const input = WORDS.map((w) => ({ ...w }))
    const mastered = masteredOf('architecture')
    filterWords(input, { category: '全部', mastery: 'unmastered', mastered })
    expect(input).toEqual(WORDS)
    expect([...mastered]).toEqual(['architecture'])
  })

  it('掌握集合含词库外的词：不影响结果', () => {
    const unmastered = filterWords(WORDS, {
      category: '全部',
      mastery: 'unmastered',
      mastered: masteredOf('obsolete'),
    })
    expect(words(unmastered)).toEqual(words(WORDS))
    const mastered = filterWords(WORDS, {
      category: '全部',
      mastery: 'mastered',
      mastered: masteredOf('obsolete'),
    })
    expect(mastered).toEqual([])
  })
})

describe('pickSavedIndex —— 进度位置读取', () => {
  it('新键优先', () => {
    const positions = { '架构设计::unmastered': 7, '架构设计::all': 3 }
    expect(pickSavedIndex(positions, '架构设计', 'unmastered')).toBe(7)
  })

  it('mastery = all 且新键缺失：回退旧版按分类存的键', () => {
    expect(pickSavedIndex({ 架构设计: 5 }, '架构设计', 'all')).toBe(5)
  })

  it('unmastered / mastered 且新键缺失：不回退旧键，返回 0', () => {
    expect(pickSavedIndex({ 架构设计: 5, '架构设计::all': 3 }, '架构设计', 'unmastered')).toBe(0)
    expect(pickSavedIndex({ 架构设计: 5 }, '架构设计', 'mastered')).toBe(0)
  })

  it('都缺失：返回 0', () => {
    expect(pickSavedIndex({}, '架构设计', 'all')).toBe(0)
  })

  it('新键值为 0 时仍以新键为准（不能当缺失处理）', () => {
    expect(pickSavedIndex({ '架构设计::all': 0, 架构设计: 9 }, '架构设计', 'all')).toBe(0)
  })
})

describe('positionKey', () => {
  it('与 pickSavedIndex 使用同一键格式', () => {
    expect(positionKey('架构设计', 'unmastered')).toBe('架构设计::unmastered')
    expect(pickSavedIndex({ [positionKey('架构设计', 'unmastered')]: 4 }, '架构设计', 'unmastered')).toBe(4)
  })
})
