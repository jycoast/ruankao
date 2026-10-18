export type MasteryFilter = 'all' | 'unmastered' | 'mastered'

/** 进度键：卡片位置按「分类::掌握筛选」分别记录。 */
export function positionKey(category: string, mastery: MasteryFilter): string {
  return `${category}::${mastery}`
}

/**
 * 单词筛选：分类与掌握状态取交集。
 * category 为「全部」表示不按分类过滤；mastery 为 all 时不过滤掌握状态。
 */
export function filterWords<T extends { word: string; category: string }>(
  words: readonly T[],
  opts: { category: string; mastery: MasteryFilter; mastered: ReadonlySet<string> },
): T[] {
  const { category, mastery, mastered } = opts
  return words.filter((w) => {
    if (category !== '全部' && w.category !== category) return false
    if (mastery === 'unmastered') return !mastered.has(w.word)
    if (mastery === 'mastered') return mastered.has(w.word)
    return true
  })
}

/**
 * 读取某筛选组合上次的卡片位置。
 * 新键优先；mastery = all 时回退旧版按分类存的键，老用户进度不丢。
 */
export function pickSavedIndex(
  positions: Record<string, number>,
  category: string,
  mastery: MasteryFilter,
): number {
  const key = positionKey(category, mastery)
  if (typeof positions[key] === 'number') return positions[key]
  if (mastery === 'all' && typeof positions[category] === 'number') return positions[category]
  return 0
}
