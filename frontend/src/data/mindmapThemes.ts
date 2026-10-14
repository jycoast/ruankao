// 知识导图画布背景色预设。
//
// 背景作用于整张 .mm-card——头部操作区（标题/提示/适应屏幕·全屏按钮）与导图画布区
// 同底色，点 ⓘ 弹出的详情抽屉也一样，整体统一；仅「背景」选择浮层保留其默认面板。
// CSS 变量（--mm-bg/--mm-ink/--mm-link/--mm-warn/--mm-muted/--mm-line）设在
// .mm-card 根上，头部/画布/抽屉都是其后代，可共用。
// key 为 'default'（跟随主题）的档位不涂底、不覆写取色，行为与改造前完全一致；
// 其余档位同时给出文字色 ink、ⓘ 链接色 link、易错警示色 warn，以及 markmap 取色板 accents。
export interface MindmapBgPreset {
  key: string
  name: string
  /** 画布背景色；缺省 = 跟随全局明暗 */
  bg?: string
  /** 节点文字色；缺省时沿用 .mm-svg 的 text-slate-700 / dark:text-slate-200 */
  ink?: string
  /** ⓘ 详情链接色；缺省时沿用现有 light/dark 两态规则 */
  link?: string
  /** 警示色（详情「易错点」标题）；随底色深/浅给出不同明度，保证可读 */
  warn?: string
  /** markmap color() 圆点/连线取色板；空数组 = 交给 markmap 默认 */
  accents: string[]
}

/** localStorage 键：选中档位持久化到本机（与 appearance store 的 theme_color 同一套模式） */
export const MINDMAP_BG_KEY = 'mindmap_bg'

export const MINDMAP_BG_PRESETS: MindmapBgPreset[] = [
  { key: 'default', name: '跟随主题', accents: [] },
  {
    key: 'ink',
    name: '纯黑',
    bg: '#000000',
    ink: '#e5e7eb', // gray-200
    link: '#60a5fa', // blue-400
    warn: '#fb7185', // rose-400
    accents: ['#60a5fa', '#34d399', '#fbbf24', '#f472b6', '#a78bfa', '#fb923c'],
  },
  {
    key: 'navy',
    name: '墨蓝夜读',
    bg: '#0d1526',
    ink: '#dbe2ef',
    link: '#7cb8ff',
    warn: '#fb7185', // rose-400
    accents: ['#60a5fa', '#34d399', '#fbbf24', '#f472b6', '#a78bfa', '#fb923c'],
  },
  {
    key: 'cream',
    name: '米白护眼',
    bg: '#f8f2e5',
    ink: '#4b4234',
    link: '#2563eb',
    warn: '#e11d48', // rose-600
    accents: ['#2563eb', '#0d9488', '#d97706', '#dc2626', '#7c3aed', '#db2777'],
  },
  {
    key: 'gray',
    name: '浅灰',
    bg: '#eef1f5',
    ink: '#343a43',
    link: '#2563eb',
    warn: '#e11d48', // rose-600
    accents: ['#2563eb', '#0d9488', '#d97706', '#dc2626', '#7c3aed', '#db2777'],
  },
]

/** 按 key 取预设；未知/空 key 回落默认「跟随主题」 */
export function findPreset(key: string | null | undefined): MindmapBgPreset {
  return MINDMAP_BG_PRESETS.find((p) => p.key === key) ?? MINDMAP_BG_PRESETS[0]
}

/**
 * 从主分支索引取色：取 node.state.path 首段序号（形如 "0.1.2" 的第一个数字），
 * 每条根级分支整体锁定一色。深色底上比 markmap 默认逐层取色更整体、更好读。
 */
export function branchColor(accents: string[], path?: string | null): string {
  const len = accents.length
  if (!len) return ''
  const idx = Number(String(path ?? '').split('.')[0])
  const safe = Number.isFinite(idx) ? idx : 0
  return accents[((safe % len) + len) % len]
}
