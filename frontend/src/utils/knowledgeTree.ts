// 知识点层级工具：扁平列表（含 parent_id/sort_order）→ 树 → markmap 用的嵌套 Markdown。

export interface KPItem {
  id: number
  name: string
  parent_id?: number
  sort_order?: number
  content?: string
  key_points?: string
  easy_mistakes?: string
  [k: string]: any
}

export interface KPNode extends KPItem {
  children: KPNode[]
}

/** 由扁平列表构建森林（顶级 = parent_id 为 0/空 或指向不存在的父）。 */
export function buildTree(items: KPItem[]): KPNode[] {
  const map = new Map<number, KPNode>()
  items.forEach((it) => map.set(it.id, { ...it, children: [] }))
  const roots: KPNode[] = []
  for (const node of map.values()) {
    const pid = Number(node.parent_id || 0)
    const parent = pid ? map.get(pid) : undefined
    if (parent) parent.children.push(node)
    else roots.push(node)
  }
  const sortRec = (nodes: KPNode[]) => {
    nodes.sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0) || a.id - b.id)
    nodes.forEach((n) => sortRec(n.children))
  }
  sortRec(roots)
  return roots
}

function hasDetail(n: KPItem): boolean {
  return !!(n.content || n.key_points || n.easy_mistakes)
}

function escapeName(s: string): string {
  // markmap 标题里避免把内容当成 markdown 语法
  return (s || '').replace(/\n/g, ' ').trim()
}

/**
 * 树 → 嵌套 Markdown（markmap Transformer 消费）。
 * 根为 rootTitle；每个节点用标题层级；有详情的节点追加 [ⓘ 详情](#kp:<id>) 哨兵（按 id 联动）。
 */
export function treeToMarkdown(roots: KPNode[], rootTitle: string): string {
  const lines: string[] = [`# ${escapeName(rootTitle)}`]

  const walk = (node: KPNode, depth: number) => {
    const level = Math.min(depth + 1, 6) // #=1 已用于根，子级从 ## 开始
    const marker = '#'.repeat(level)
    const detail = hasDetail(node) ? ` [ⓘ 详情](#kp:${node.id})` : ''
    lines.push('')
    lines.push(`${marker} ${escapeName(node.name)}${detail}`)
    node.children.forEach((c) => walk(c, depth + 1))
  }

  roots.forEach((r) => walk(r, 1))
  return lines.join('\n')
}
