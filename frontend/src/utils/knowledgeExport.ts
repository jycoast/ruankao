// 知识点导出思维导图长图：整树展开 + 详情全文嵌在节点卡片里，输出【一张】PNG。
// 布局自绘（左→右树、圆角卡片、贝塞尔连线，纯 DOM + SVG），html2canvas 截图。
// 不用 markmap 渲染：其 SVG 依赖 foreignObject，浏览器把含 foreignObject 的 SVG
// 画进 canvas 时渲染不可靠；自绘 DOM/SVG 走 html2canvas 是稳定路径。
//
// 单图约束：浏览器 canvas 单边 ≤ 16384px、总面积有限。整树内容纵向可达数万 px，
// 直接竖排必超限 —— 先把各章节按可用高度切成「段」（超长章节按分支切，续段重复
// 章节根节点），再把段横向排进多列，选可缩放比最大（=最清晰）的列数出一张图。
import html2canvas from 'html2canvas'
import { buildTree, type KPNode } from './knowledgeTree'
import { renderMd } from './markdown'

// 浏览器 canvas 安全上限（单边留余量 < 16384；面积 < 16384²×0.8）
const MAX_SIDE = 15600
const AREA_CAP = 214_000_000
const COL_GAP = 46 // 导图层级列间距（连线空间）
const CARD_GAP_Y = 16 // 同层相邻子树纵向间距
const BAND_GAP = 28 // 同列相邻段间距
const BAND_PAD = 24 // 段容器上下留白（列高统计与段盒高共用）
// 页眉名义高度：只剩两处「预算/阈值」性质的用途 —— planLayout 的列高预算与 chapterBands 的
// 切段阈值。两条渲染路径（打印、位图）都已改成运行时实测（见 measureBlockH），别拿它当布局尺寸。
const HEADER_H = 74
const HEADER_GAP = 12 // 页眉（含其下分隔线）与首张卡片之间的呼吸位
const FOOTER_H = 40 // 页脚高度
const MAX_COLS = 12 // 列数试算上限

// —— 打印导出（一章一页）——
// 打印走浏览器打印管线，不经过 canvas：文字以真矢量写进 PDF（放大不糊），图片按原始
// 分辨率嵌入。所以「清晰不清晰」不再由画布面积决定——位图路线被 214M 面积锁在
// scale 1.09（见 exportKnowledgeImage 注释），打印路线没有这个上限。
// 一章 = 一张纸 = 一列：整章一段不切，章节根卡片因此只出现一次，读者顺着一条主干读到底。
// 纸就按内容的原生尺寸给，不缩不裁（实测见 chapterBands 注释）。
//
// 单张纸的长边上限（CSS px）= 19200px = 14400pt = 200in，PDF 阅读器普遍遵守的页面尺寸
// 实现上限，留出 36% 余量给数据增长（现有最高章 14061px）。超过它的极端章不进打印管线
// 硬缩（缩了会被裁，见 chapterBands 注释），而是切成多张续纸。纸本身能有多大实测过：
// 无头 Chrome 打印 30000px 高的纸依然完整无裁剪，这里卡在 19200 是照顾阅读器而不是浏览器。
const PRINT_PAGE_MAX = 19200
/** 一列的累计高（列高统计与 packColumns 的收列判据共用同一套算法） */
const colHeight = (col: PagePlan[]) =>
  col.reduce((s, b) => s + b.h + BAND_PAD, 0) + BAND_GAP * Math.max(0, col.length - 1)

// 单段高度上限：超过就按分支切段（续段重复章节根）。不切的话超长章节会把整列钉死，
// 各列高低失衡 → scale 被压低，文字和图片一起糊。
//
// 实测（无头 Chrome + 线上 310 条知识点 + 84 张真实源图，源图宽中位数 1022px）：
//   段高上限  段数  列数  scale   正文     图片设备像素中位   不足源图一半的图片
//   不切      15    8     1.057   12.7px   311 (30%)          72 / 84   ← 旧行为
//   8000      16    8     1.019   12.2px   300 (29%)          72
//   5000      24    8     1.146   13.7px   337 (33%)          69
//   3500      32    7     1.201   14.4px   353 (35%)          65        ← 取这档
// 切段本身救不了图片（图片宽度被卡片内容宽钉死），但把列压均衡后 scale 能涨 13%，
// 正好抵掉下面放宽列宽的文字代价。再往下切收益递减、章节也碎得难看。
const BAND_CAP = 3500

// 含图层级卡片宽度的放大倍数（1 = 不放宽）。图片宽被 .kp-md img{max-width:100%}
// 钉在卡片内容宽上，所以「图片清不清晰」≈「含图列有多宽」× scale。放宽到 1.5x 时
// 列数 8→6、总宽几乎不变，scale 只掉 4%，图片设备像素却 +48%；但 1.75x→2x 是断崖
// （列数掉到 5，最高列 15281→21134），scale 从 0.98 崩到 0.73，正文跟着掉到 8.8px。
// 所以停在 1.5x。与 BAND_CAP=3500 合起来：图片 311→496px(+59%)、正文 12.7→13.1px。
const IMAGE_COL_BOOST = 1.5
const CARD_CHROME = 26 // 卡片左右 padding(24) + 边框(2)，用于「撑到刚好装下源图」

// 各层级卡片宽度；更深层级取最后一档。含图层级会被 IMAGE_COL_BOOST 临时撑宽，
// 所以宽度是模块级可变状态：每次导出前必须 resetCardWidths 复位，否则宽度漏到下
// 一次导出（撑宽的列变成常驻，第二次导出的排版就跟第一次不一样了）。
const CARD_W = [250, 300, 320]
let cardW: number[] = [...CARD_W]
const cardWidth = (depth: number) => cardW[Math.min(depth, cardW.length - 1)]
const colX = (depth: number) => {
  let x = 0
  for (let i = 0; i < depth; i++) x += cardWidth(i) + COL_GAP
  return x
}
function resetCardWidths(maxDepth: number) {
  cardW = []
  for (let d = 0; d <= maxDepth; d++) cardW[d] = CARD_W[Math.min(d, CARD_W.length - 1)]
}
/** 树的最大层级（布局前就要知道，用于复位卡片宽度） */
function treeDepth(nodes: KPNode[]): number {
  const walk = (n: KPNode): number => (n.children.length ? 1 + Math.max(...n.children.map(walk)) : 0)
  return nodes.length ? Math.max(...nodes.map(walk)) : 0
}

const EXPORT_CSS = `
* { box-sizing: border-box; margin: 0; padding: 0; }
.kp-export, .kp-measure {
  font-family: 'Microsoft YaHei', 'PingFang SC', 'Hiragino Sans GB', 'Segoe UI', Arial, sans-serif;
  color: #111827;
}
.kp-page { position: relative; background: #ffffff; overflow: hidden; }
.kp-page-inner { position: absolute; left: 0; top: 0; }
.kp-card {
  position: absolute; border: 1px solid #dbe2ea; border-radius: 10px;
  background: #ffffff; padding: 9px 12px;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.06);
}
.kp-card-title { font-weight: 600; color: #0f172a; line-height: 1.45; word-break: break-word; }
.kp-card.d0 { border-left: 4px solid #2563eb; background: #f8fafc; }
.kp-card.d0 .kp-card-title { font-size: 16px; color: #1d4ed8; }
.kp-card.d1 .kp-card-title { font-size: 14px; }
.kp-card.d2 .kp-card-title, .kp-card.d3 .kp-card-title { font-size: 13px; color: #334155; }
.kp-md { color: #1f2937; word-break: break-word; font-size: 12px; line-height: 1.65; }
.kp-md p { margin: 0.25em 0; }
.kp-md h1, .kp-md h2, .kp-md h3, .kp-md h4 { font-weight: 600; margin: 0.5em 0 0.25em; font-size: 1.05em; }
.kp-md ul, .kp-md ol { margin: 0.25em 0; padding-left: 1.4em; }
.kp-md ul { list-style: disc; }
.kp-md ol { list-style: decimal; }
.kp-md li { margin: 0.1em 0; }
.kp-md table { border-collapse: collapse; margin: 0.4em 0; width: 100%; font-size: 11px; }
.kp-md th, .kp-md td { border: 1px solid #d1d5db; padding: 3px 6px; text-align: left; }
.kp-md th { background: #f3f4f6; }
.kp-md img { max-width: 100%; height: auto; margin: 0.3em 0; border-radius: 4px; }
.kp-md code { background: #f3f4f6; padding: 1px 4px; border-radius: 4px; font-size: 0.9em; }
.kp-md pre { background: #f6f8fa; border: 1px solid #e5e7eb; border-radius: 6px; padding: 8px 10px; overflow-x: auto; margin: 0.4em 0; }
.kp-md pre code { background: none; padding: 0; }
.kp-md blockquote { margin: 0.4em 0; padding: 3px 10px; border-left: 3px solid #93c5fd; background: #f8fafc; color: #475569; }
.kp-links { position: absolute; left: 0; top: 0; }
/* 连线是纯色细线，位图路径下被缩放抗锯齿冲淡成灰雾：加粗 + 加深补偿，
   不含图、不占画布面积（体积/清晰度零代价）。 */
.kp-links path { fill: none; stroke: #64748b; stroke-width: 2.6; stroke-linejoin: round; }
/* padding 从各处内联 style 挪到这里：探针量高要带同一份盒模型，否则量出来的高度少了
   上下 padding，内容又会顶进页眉。整块居中只在打印路径加（章名是每页的标题）：
   位图长图是另一张脸面，动它要单独说。 */
.kp-page-header {
  position: absolute; left: 0; top: 0; width: 100%;
  padding: 22px 0 12px;
}
.kp-doc-title { font-size: 20px; font-weight: 700; color: #0f172a; }
.kp-doc-meta { margin-top: 4px; font-size: 12px; color: #94a3b8; }
.kp-page-footer {
  position: absolute; left: 0; bottom: 0; width: 100%;
  border-top: 1px solid #e5e7eb; padding: 6px 4px 0;
  font-size: 11px; color: #9ca3af; display: flex; justify-content: space-between;
}
`

function esc(s: string): string {
  return (s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

// —— 详情标签（内容/考点/易错点）做成 SVG 图片 ——
// html2canvas 用自身 fillText 基线换算画普通文字，对中文字体的字形度量有系统性
// 偏差，小色块标签里的文字始终对不齐（调 CSS 无法根治）。SVG 标签走浏览器 SVG
// 渲染器，文字精确居中。但【内联】<svg> 不行：html2canvas 的 DocumentCloner 会把
// SVG 子树各元素的计算样式整份复制成内联 style，其中 x:0px/y:0px 等 CSS 几何属性
// 会覆盖 <text> 的 x/y 属性，序列化出的图片文字位置全错。所以包成 <img
// src="data:image/svg+xml,...">：SVG 内容在 URL 里，克隆器碰不到，按原始属性渲染。
const LABEL_FONT = `'Microsoft YaHei', 'PingFang SC', 'Hiragino Sans GB', 'Segoe UI', Arial, sans-serif`
const LABEL_H = 20
let labelMeasureCtx: CanvasRenderingContext2D | null = null
function labelChip(label: string): string {
  if (!labelMeasureCtx) labelMeasureCtx = document.createElement('canvas').getContext('2d')!
  labelMeasureCtx.font = `11px ${LABEL_FONT}`
  const w = Math.ceil(labelMeasureCtx.measureText(label).width) + 12
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${LABEL_H}">` +
    `<rect x="0.5" y="0.5" width="${w - 1}" height="${LABEL_H - 1}" rx="4" fill="#eff6ff" stroke="#bfdbfe"/>` +
    `<text x="${w / 2}" y="${LABEL_H / 2}" text-anchor="middle" dominant-baseline="central" ` +
    `font-family="${LABEL_FONT.replace(/'/g, '')}" font-size="11" fill="#2563eb">${esc(label)}</text>` +
    `</svg>`
  return (
    `<img src="data:image/svg+xml,${encodeURIComponent(svg)}" alt="${esc(label)}" ` +
    `width="${w}" height="${LABEL_H}" style="display:block;margin:6px 0 2px;">`
  )
}

/** 节点卡片 HTML：标题 + 非空详情三段 */
function cardHTML(node: KPNode, depth: number): string {
  const section = (label: string, md?: string) =>
    md && md.trim()
      ? labelChip(label) + `<div class="kp-md">${renderMd(md)}</div>`
      : ''
  const details =
    section('内容', node.content) + section('考点', node.key_points) + section('易错点', node.easy_mistakes)
  return (
    `<div class="kp-card d${Math.min(depth, 3)}" style="width:${cardWidth(depth)}px">` +
    `<div class="kp-card-title">${esc(node.name)}</div>${details}</div>`
  )
}

/** 等待容器内全部图片加载完（成功/失败都放行，6s 兜底）——卡片高度依赖图片尺寸 */
async function waitForImages(scope: HTMLElement) {
  const imgs = Array.from(scope.querySelectorAll('img')) as HTMLImageElement[]
  await Promise.all(
    imgs.map((img) =>
      img.complete && img.naturalWidth > 0
        ? Promise.resolve()
        : new Promise<void>((res) => {
            img.onload = () => res()
            img.onerror = () => res()
            setTimeout(() => res(), 6000)
          }),
    ),
  )
}

/**
 * 等容器内图片真正解码完成（img 加载完只保证拿到数据，不保证可绘制）。
 * 打印前必须过一遍：否则浏览器可能在图片解码完成前就把页面交给打印管线，PDF 里留白框。
 */
async function decodeImages(scope: HTMLElement) {
  const imgs = Array.from(scope.querySelectorAll('img')) as HTMLImageElement[]
  await Promise.all(
    imgs.map((img) => (typeof img.decode === 'function' ? img.decode().catch(() => {}) : Promise.resolve())),
  )
}

/** 已布局节点：卡片 + 以「子树顶部为原点」的局部坐标 */
interface LN {
  id: number // 稳定编号：卡片与克隆都带 data-kp-id，段内自检靠它认归属
  card: HTMLElement
  x: number // 列内全局 x（按层级固定）
  y: number // 卡片顶边（局部 y）
  w: number
  h: number // 卡片高
  totalH: number // 整棵子树高
  kids: { node: LN; dy: number }[] // 子树相对本子树顶部的纵向偏移
  edges: [number, number, number, number][] // 连线 [x1,y1,x2,y2]，y 为局部坐标
}

/**
 * 第一步：递归建卡片 DOM（不量高）。高度必须等图片加载完再量，
 * 拆成两步就是为了在 createCards 与 finalizeLayout 之间插入 waitForImages。
 */
function createCards(node: KPNode, depth: number, measure: HTMLElement, seq: { n: number }): LN {
  const holder = document.createElement('div')
  holder.innerHTML = cardHTML(node, depth)
  const card = holder.firstElementChild as HTMLElement
  const id = seq.n++
  card.dataset.kpId = String(id) // 克隆会带走这个属性，段内自检才能认出「这是哪个节点的卡片」
  measure.appendChild(card)
  const kids = node.children.map((c) => createCards(c, depth + 1, measure, seq))
  return {
    id,
    card,
    x: colX(depth),
    y: 0,
    w: cardWidth(depth),
    h: 0,
    totalH: 0,
    kids: kids.map((k) => ({ node: k, dy: 0 })),
    edges: [],
  }
}

/**
 * 第二步（图片加载完之后调用）：整洁树布局。子树占位高度 = max(卡片高, 子树列总高)，
 * 父卡片在子树跨度内垂直居中；坐标均以子树顶部为原点。
 * 必须先量后布：卡片高度依赖图片尺寸，图片没加载完就量会偏低，
 * 底部内容会被固定高的段容器 + 页面 overflow:hidden 裁掉。
 */
function finalizeLayout(n: LN, depth: number) {
  for (const k of n.kids) finalizeLayout(k.node, depth + 1)
  const childrenH = n.kids.length
    ? n.kids.reduce((s, k) => s + k.node.totalH, 0) + CARD_GAP_Y * (n.kids.length - 1)
    : 0
  n.h = n.card.offsetHeight
  n.totalH = Math.max(n.h, childrenH)
  n.y = childrenH > n.h ? (childrenH - n.h) / 2 : 0

  const edges: [number, number, number, number][] = []
  const x1 = colX(depth) + cardWidth(depth)
  const y1 = n.y + n.h / 2
  let cursor = 0
  for (const k of n.kids) {
    const x2 = colX(depth + 1)
    const y2 = cursor + k.node.y + k.node.h / 2
    edges.push([x1, y1, x2, y2])
    // 后代连线整体平移到本子树坐标系
    for (const e of k.node.edges) edges.push([e[0], e[1] + cursor, e[2], e[3] + cursor])
    cursor += k.node.totalH + CARD_GAP_Y
  }
  n.edges = edges
  n.kids = n.kids.map((k, i) => ({
    node: k.node,
    dy: n.kids.slice(0, i).reduce((s, p) => s + p.node.totalH, 0) + CARD_GAP_Y * i,
  }))
}

/**
 * 把「含图的层级」的卡片撑宽，让图片在导出图里拿到更多设备像素。
 * 必须夹在 waitForImages 与 finalizeLayout 之间：源图宽度要等图片加载完才知道，
 * 而卡片高度随宽度变（换行位置变了），所以撑宽得赶在量高之前。
 * 按层级统一撑宽（不是逐卡片）：同层卡片共用一个列 x，单独撑某一张会让连线对不齐。
 * 上限取「刚好装下该层最宽的源图」，避免为小图平白撑开列宽、白丢 scale。
 */
function widenImageColumns(roots: LN[], boost: number) {
  if (boost <= 1) return
  const widest = new Map<number, number>()
  const scan = (n: LN, depth: number) => {
    for (const im of Array.from(n.card.querySelectorAll('.kp-md img')) as HTMLImageElement[]) {
      if (im.naturalWidth > (widest.get(depth) ?? 0)) widest.set(depth, im.naturalWidth)
    }
    n.kids.forEach((k) => scan(k.node, depth + 1))
  }
  roots.forEach((r) => scan(r, 0))
  for (const [depth, natural] of widest) {
    const base = cardW[depth]
    const want = Math.min(Math.round(natural + CARD_CHROME), Math.round(base * boost))
    if (want > base) cardW[depth] = want
  }
  const apply = (n: LN, depth: number) => {
    const w = cardWidth(depth)
    n.card.style.width = w + 'px' // 覆盖 cardHTML 里的内联宽度
    n.w = w
    n.x = colX(depth)
    n.kids.forEach((k) => apply(k.node, depth + 1))
  }
  roots.forEach((r) => apply(r, 0))
}

/** 把一棵（或一段）已布局子树画进页面容器：卡片 + 平移后的连线 */
function renderSubtree(parent: HTMLElement, n: LN, dy: number) {
  const c = n.card // appendChild 即移动，图片加载状态保留
  c.style.left = n.x + 'px'
  c.style.top = dy + n.y + 'px'
  parent.appendChild(c)
  for (const k of n.kids) renderSubtree(parent, k.node, dy + k.dy)
}

/**
 * 画段首的「章节根卡片」：只放它自己，顶到段首（不按子树居中），不递归。
 *
 * 为什么不能递归：段会把章节根重复一遍，而卡片 DOM 全局只有一份、appendChild 是移动。
 * 续段的根若递归，就会把前面段已经摆好的卡片整片搬过来，堆在「章节绝对坐标」上——
 * 实测 310 条数据下 63 对卡片重叠、单段内容溢出段盒 10698px。分支卡片由同段其它
 * placement 各自递归画，各就各位。根卡片顶到段首也是刻意的：段里它就是「本段表头」。
 *
 * 为什么必须用【克隆】而不是原卡片：同一张卡片被多个段引用时，appendChild 会把原卡片
 * 从上一段搬到这一段——于是它只剩在【最后画的那一段】里。实测第 7 章（3 段）的章节根
 * 因此跑到了第 3 列，而第 1、2 列的根位空着、只剩一排凭空发散的连线；第 1、2、6、7、9
 * 章都有这个毛病（段内自检直接报 7 处）。克隆让每一段都在自己最左有自己的根卡片。
 * 克隆宽度是内联样式、内容一致，所以高度与原卡片一致，连线锚点
 * (root.x + root.w, dy + root.h/2) 依然对得上；图片 src 相同，打印前的 decodeImages 一并解码。
 */
function renderBandRoot(parent: HTMLElement, n: LN, dy: number) {
  const c = n.card.cloneNode(true) as HTMLElement
  c.style.left = n.x + 'px'
  c.style.top = dy + 'px'
  parent.appendChild(c)
}

/** 收集一棵子树全部连线（平移 dy），返回 SVG path 字符串 */
function subtreePaths(n: LN, dy: number): string {
  let d = ''
  for (const [ax, ay, bx, by] of n.edges) {
    const x1 = ax
    const y1 = dy + ay
    const x2 = bx
    const y2 = dy + by
    const mx = (x1 + x2) / 2
    d += `M ${x1} ${y1} C ${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2} `
  }
  for (const k of n.kids) d += subtreePaths(k.node, dy + k.dy)
  return d
}

/**
 * 段首章节根的连线：只画「本段真的放了的那几个分支」到根的连线（不递归）。
 * 分支 subtree 内部的连线由分支自己的 placement 负责，见 subtreePaths。
 */
function bandRootPaths(root: LN, placements: Placement[], dy: number): string {
  const dyOf = new Map<LN, number>()
  for (const p of placements) if (!p.rootOnly) dyOf.set(p.node, p.dy)
  const x1 = root.x + root.w
  const y1 = dy + root.h / 2 // 根卡片顶到段首，纵向中点即 h/2（不能按子树居中）
  let d = ''
  for (const kid of root.kids) {
    const kp = dyOf.get(kid.node)
    if (kp === undefined) continue // 这条分支不在本段，不画线
    const x2 = kid.node.x
    const y2 = kp + kid.node.y + kid.node.h / 2
    const mx = (x1 + x2) / 2
    d += `M ${x1} ${y1} C ${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2} `
  }
  return d
}

/** 一段上放置的子树：{子树根, 纵向平移}；h 为该段内容净高 */
interface Placement {
  node: LN
  dy: number
  /** 段首重复的章节根：只画这张卡片与它到本段分支的连线，不画整棵子树 */
  rootOnly?: boolean
}
interface PagePlan {
  placements: Placement[]
  h: number
}

/**
 * 把一棵超页高的子树按分支切页：每页重复根卡片 + 若干完整子分支；
 * 子分支自身仍超页高时递归同规则处理。返回页面列表（无空页）。
 */
function splitByBranches(root: LN, usableH: number): PagePlan[] {
  if (root.totalH <= usableH) return [{ placements: [{ node: root, dy: 0 }], h: root.totalH }]
  const out: PagePlan[] = []
  const rootPad = root.h + 24
  let page: Placement[] = []
  let y = 0
  const closePage = () => {
    if (page.length > 1) out.push({ placements: page, h: y }) // 只有重复的根、没放任何分支的空页丢弃
  }
  const newPage = () => {
    closePage()
    page = [{ node: root, dy: 0, rootOnly: true }] // 段首只放根卡片，分支各自递归
    y = rootPad
  }
  newPage()
  for (const k of root.kids) {
    if (y + k.node.totalH > usableH && page.length > 1) newPage()
    if (y + k.node.totalH > usableH) {
      // 该分支单独都放不下：递归切它，其页面独立成页（自带该分支的根卡片）
      closePage()
      out.push(...splitByBranches(k.node, usableH))
      page = [{ node: root, dy: 0, rootOnly: true }]
      y = rootPad
      continue
    }
    page.push({ node: k.node, dy: y })
    y += k.node.totalH + CARD_GAP_Y
  }
  closePage()
  return out
}

/**
 * 把段落尽量均衡地装进 count 列，返回各列的段列表；装不下（有段超过列高上限
 * 或列数不够）返回 null。列累计高度达到 target 即收列，保证各列均衡。
 */
function packColumns(bands: PagePlan[], count: number, cap: number): PagePlan[][] | null {
  const colH = colHeight // 收列判据与列高统计必须同源，否则选出的列高对不上
  const total = bands.reduce((s, b) => s + b.h + BAND_PAD, 0) + BAND_GAP * Math.max(0, bands.length - 1)
  const target = total / count
  const cols: PagePlan[][] = []
  let cur: PagePlan[] = []
  for (const b of bands) {
    if (b.h + BAND_PAD > cap) return null
    const would = colH(cur) + (cur.length ? BAND_GAP : 0) + b.h + BAND_PAD
    if (cur.length && (would > cap || colH(cur) >= target)) {
      cols.push(cur)
      if (cols.length === count) return null
      cur = []
    }
    cur.push(b)
  }
  if (cur.length) cols.push(cur)
  return cols
}

/** 一章的已布局根（打印按章分页要用章节名；位图路径只用到 node） */
interface ChapterNode {
  name: string
  node: LN
}

/** 布局产物：导出图片 / 导出 PDF 共用同一套排版，只有纸张的组织方式不同（图片多列 / PDF 一章一列） */
interface LayoutPlan {
  wrapper: HTMLElement // 已挂在 body 上，调用方负责移除
  chapters: ChapterNode[] // 按顶层章节分组：打印路径一章一页，位图路径再摊平
  pageW: number
  colCap: number
}

interface LayoutOpts {
  /** 含图列宽的放大倍数上限；1 = 不放宽（打印走 1，理由见 prepareKnowledgePdf） */
  imageColBoost?: number
}

/**
 * 建卡片 → 等图片全部加载 → 按需撑宽含图列 → 量高做布局（顺序不能反，见
 * finalizeLayout 注释）。返回按章分组的已布局树，纸张/画布的尺寸决策交给调用方。
 */
async function planLayout(items: any[], opts: LayoutOpts = {}): Promise<LayoutPlan> {
  const roots = buildTree(items)
  resetCardWidths(treeDepth(roots)) // 复位上一次导出可能残留的撑宽

  const wrapper = document.createElement('div')
  wrapper.className = 'kp-export'
  wrapper.style.cssText = `position:absolute;left:-9999px;top:0;`
  wrapper.innerHTML = `<style>${EXPORT_CSS}</style>`
  // 隐形测量容器：真实宽度布局下量卡片高（图片加载后高度才准）
  const measure = document.createElement('div')
  measure.className = 'kp-measure'
  measure.style.cssText = `position:absolute;left:0;top:0;visibility:hidden;`
  wrapper.appendChild(measure)
  document.body.appendChild(wrapper)

  try {
    const seq = { n: 0 }
    const laidRoots = roots.map((r) => createCards(r, 0, measure, seq))
    await waitForImages(wrapper)
    // 撑宽必须排在量高之前：这一步只改宽度，高度交给 finalizeLayout 按新宽度重量
    widenImageColumns(laidRoots, opts.imageColBoost ?? 1)
    laidRoots.forEach((r) => finalizeLayout(r, 0))

    // 全局最大深度决定列宽（各列统一宽，观感一致）
    const maxDepth = treeDepth(roots)
    const pageW = colX(maxDepth) + cardWidth(maxDepth) + 8
    const colCap = MAX_SIDE - HEADER_H - FOOTER_H

    return {
      wrapper,
      chapters: roots.map((r, i) => ({ name: r.name, node: laidRoots[i] })),
      pageW,
      colCap,
    }
  } catch (e) {
    wrapper.remove()
    throw e
  }
}

/**
 * 整章怎么切段 = 占几张纸：正常【一段】（一张纸一个整段），就是这个功能的全部：不切段，
 * 章节根卡片只出现一次（重复根是切段时给续纸当表头用的），读者顺着一条主干读到底。
 * 纸高 = 该章内容净高 + 页眉页脚，就是用户要的「一张超高纸」：文字与图片全是原生尺寸。
 *
 * 实测（线上 310 条知识点 + 84 张真实源图）：最高的章 = 软件工程基础知识，
 * 1702×14061px（53 张卡片），即 1276×10546pt 的纸——阅读器要一路缩小才看得全，
 * 放大看仍是矢量。这是「一章一张纸」的固有代价，也是用户明确要的形态。
 *
 * 为什么不像早先那样「超过上限就整体等比缩」：**缩了会被裁**。实测：纸 727×6000、
 * 内容 1702×14061 按 0.4267 缩进纸里时，53 张卡片只画出了上面 37 张——纸面 y ≥ 4042
 * （内容的下 1/3）那 16 张连文字都没进 PDF，整篇也搜不到。极简复现（同样几何的标记块）
 * 同样丢尾部，且换成不缩放的纸就完整；30000px 高的纸也完整（无头 Chrome 打印，无裁剪）。
 * 所以「缩」不是安全阀而是丢内容的来源，只能靠纸把内容装下。
 *
 * 超过 PRINT_PAGE_MAX 的极端章（现有数据跑不到：14061 < 19200）退化成按顶层分支切段、
 * 一段一张续纸、每张重复章节根当表头——多几张纸可以接受，丢内容不行。
 *
 * 这里用名义页眉高 HEADER_H 判「装得下装不下」，实际纸张用的是实测页眉高（差 20px 量级）：
 * 切段是先于量宽走的（页码总数要用段数），拿不到实测值。误差方向只会让纸略高一点，
 * 对 19200 的上限和 36% 余量无所谓；真正要防的是「整章被裁」，那条由纸随内容走保证。
 */
function chapterBands(chNode: LN): PagePlan[] {
  const H = chNode.totalH + BAND_PAD + HEADER_H + FOOTER_H
  if (H <= PRINT_PAGE_MAX) return [{ placements: [{ node: chNode, dy: 0 }], h: chNode.totalH }]
  const bands = splitByBranches(chNode, PRINT_PAGE_MAX - BAND_PAD - HEADER_H - FOOTER_H)
  // 根卡片自己就比一张纸还高（子分支一个都放不下）时 splitByBranches 返回空：宁可让这张
  // 卡片溢出它那张纸，也不能整章不画。除这种畸形数据外走不到这里。
  return bands.length ? bands : [{ placements: [{ node: chNode, dy: 0 }], h: chNode.totalH }]
}

/**
 * 该章内容在纸宽方向的实际右边界（列内 x 是绝对坐标）。纸宽必须按章量：
 * 拿「全局最深列的列宽」当纸宽，浅章会剩下整片右侧空白——阅读器「适应页面」时
 * 把整章缩得更小，正是要避免的那种糊。实测第 1 章内容只占纸宽 35%。
 */
function chapterContentW(n: LN): number {
  let right = n.x + n.w
  for (const k of n.kids) right = Math.max(right, chapterContentW(k.node))
  return right
}

/**
 * 量一段文案的自然宽度（nowrap，含 padding）：页眉页脚是整幅纸宽呈现的固定文案，
 * 纸窄于它们会被挤成多行、压到内容上。用真实 class 量，字体字号随 CSS 走不另抄一份。
 * html 只传「内容」：探针自己就带 cls，再套一层同名 div 会让探针量出 0——
 * cls 对应的元素是 absolute 的，absolute 子元素不参与父盒的尺寸计算。
 */
function measureNowrap(wrapper: HTMLElement, cls: string, html: string): number {
  const probe = document.createElement('div')
  probe.className = cls
  probe.style.cssText = 'position:absolute;left:0;top:0;bottom:auto;width:auto;white-space:nowrap;visibility:hidden;'
  probe.innerHTML = html
  wrapper.appendChild(probe)
  const w = probe.offsetWidth
  probe.remove()
  return w
}

/**
 * 量一段 HTML 在给定宽度下占的自然高度。页眉写死高度不行：它是「章名 + 副标题 + 分隔线」的
 * 自然高，随字体度量变（Windows 上实测 91px，原常量 74px ⇒ 首张卡片顶部 17px 压在页眉里，
 * 页眉文字和卡片标题叠在一起）。所以内容顶偏移由实测值决定，换个字体/平台也不会再压上。
 * 宽度按该纸的 pageW 量（页眉是 width:100% 呈现，纸宽就是这个宽度）；探针强制 static，
 * 否则带 cls 的盒自己是 absolute，量不到高度。html 同样只传内容（见 measureNowrap）。
 */
function measureBlockH(wrapper: HTMLElement, cls: string, html: string, width: number): number {
  const probe = document.createElement('div')
  probe.className = cls
  probe.style.cssText = `position:static;width:${width}px;visibility:hidden;`
  probe.innerHTML = html
  wrapper.appendChild(probe)
  const h = probe.offsetHeight
  probe.remove()
  return h
}

/**
 * 把选定的列方案渲染成一页 DOM（页眉 + 各列段 + 页脚）并挂进 wrapper，返回该页。
 * 纸盒尺寸 = 内容尺寸（W×H），不做等比缩放：纸要多大就给多大（见 chapterBands 注释）。
 * contentTop = 内容顶从这个 y 开始（页眉占掉的高度，打印路径传实测值，位图路径传名义常量）。
 */
function renderPage(
  wrapper: HTMLElement,
  pageW: number,
  colBands: PagePlan[][],
  W: number,
  H: number,
  headerHtml: string,
  footerHtml: string,
  contentTop: number,
): HTMLElement {
  const colH = Math.max(...colBands.map(colHeight))
  const page = document.createElement('div')
  page.className = 'kp-page'
  page.style.cssText += `width:${Math.ceil(W)}px;height:${Math.ceil(H)}px;`
  const inner = document.createElement('div')
  inner.className = 'kp-page-inner'
  inner.style.width = W + 'px'
  inner.style.height = H + 'px'
  page.appendChild(inner)
  inner.insertAdjacentHTML('beforeend', headerHtml)
  // 段盒先攒着，等所有段都画完再核对（搬卡片的是「后画的段」，边画边查查不到）
  const drawn: [HTMLElement, PagePlan][] = []
  // 各列：段内相对定位渲染（卡片 + 连线），列间加浅色分隔线
  for (let c = 0; c < colBands.length; c++) {
    if (c > 0) {
      inner.insertAdjacentHTML(
        'beforeend',
        `<div style="position:absolute;left:${c * pageW - 4}px;top:${contentTop}px;width:1px;height:${colH}px;background:#eef1f5;"></div>`,
      )
    }
    const colDiv = document.createElement('div')
    colDiv.style.cssText = `position:absolute;left:${c * pageW}px;top:${contentTop}px;width:${pageW}px;`
    let y = 0
    for (const band of colBands[c]) {
      const bandBoxH = band.h + BAND_PAD
      const bandDiv = document.createElement('div')
      bandDiv.style.cssText = `position:absolute;left:0;top:${y}px;width:${pageW}px;height:${bandBoxH}px;`
      // 段首的章节根（rootOnly）只画自己的卡片与到本段分支的连线，其余 placement 整棵递归：
      // 两边必须同规则，否则要么丢卡片/丢线，要么把整章拖进这一段（见 renderBandRoot 注释）
      let pathD = ''
      for (const p of band.placements) {
        pathD += p.rootOnly ? bandRootPaths(p.node, band.placements, p.dy) : subtreePaths(p.node, p.dy)
      }
      bandDiv.insertAdjacentHTML(
        'beforeend',
        `<svg class="kp-links" width="${pageW}" height="${bandBoxH}"><path d="${pathD}"></path></svg>`,
      )
      for (const p of band.placements) {
        if (p.rootOnly) renderBandRoot(bandDiv, p.node, p.dy)
        else renderSubtree(bandDiv, p.node, p.dy)
      }
      colDiv.appendChild(bandDiv)
      drawn.push([bandDiv, band])
      y += bandBoxH + BAND_GAP
    }
    inner.appendChild(colDiv)
  }
  inner.insertAdjacentHTML('beforeend', footerHtml)
  wrapper.appendChild(page)
  for (const [bandDiv, band] of drawn) auditBand(bandDiv, band)
  return page
}

/** 一个段该画出来的卡片号计数：rootOnly 只画根一张（段首表头），其余画整棵子树（与 renderSubtree 同规则） */
function countPlacedIds(n: LN, rootOnly: boolean | undefined, out: Map<number, number>) {
  out.set(n.id, (out.get(n.id) || 0) + 1)
  if (!rootOnly) for (const k of n.kids) countPlacedIds(k.node, false, out)
}

/**
 * 段内自检：这个段盒里的卡片，必须恰好是它 placement 该画的那一批。
 *
 * 为什么必须查：renderSubtree / renderBandRoot 靠 appendChild 搬 DOM 节点（卡片 DOM
 * 全局只有一份，重建会丢图片加载状态），所以「同一张卡片被两个段引用」这种错误不会
 * 报错，只会把卡片悄悄搬到后画的那个段里——段首根于是跑到最后一列、前面各列的根位
 * 空着、连线从空白处发散（实测第 7 章的根跑到了第 3 列）。按段核对，按页核对抓不到它。
 *
 * 只 console.error，不 throw：误报的代价（用户导不出文件）比漏报更大。
 */
function auditBand(bandDiv: HTMLElement, band: PagePlan) {
  const expect = new Map<number, number>()
  for (const p of band.placements) countPlacedIds(p.node, p.rootOnly, expect)
  const got = new Map<number, number>()
  for (const c of Array.from(bandDiv.querySelectorAll('.kp-card')) as HTMLElement[]) {
    const id = Number((c as HTMLElement).dataset.kpId)
    got.set(id, (got.get(id) || 0) + 1)
  }
  const diff: string[] = []
  for (const [id, n] of expect) {
    const g = got.get(id) || 0
    if (g !== n) diff.push(`#${id} 期望 ${n} 张、段内实际 ${g} 张`)
  }
  for (const [id, g] of got) if (!expect.has(id)) diff.push(`#${id} 多出 ${g} 张（不属于本段）`)
  if (diff.length) {
    console.error(
      '[知识导图导出] 段内自检未通过：段首根/分支的卡片不在它自己的段里（会出现根节点跑位、连线凭空发散）',
      diff.slice(0, 8),
    )
  }
}

/**
 * 打印样式：一张纸 = 一章。命名页（@page 名字 + page: 名字）让同一份文档里每页纸张
 * 尺寸各不相同——小章用小纸，大章用大纸，阅读器「适应页面」时才不会把章节缩成一粒。
 * 兜底 @page 取所有页里最大的那张：万一浏览器不支持命名页（Chromium 之外未验证），
 * 各章仍各自占一页、内容也不会被裁，只是纸张统一偏大。
 */
function printCss(pages: { PW: number; PH: number }[]): string {
  const fallback = pages.length ? Math.max(...pages.map((p) => Math.max(p.PW, p.PH))) : PRINT_PAGE_MAX
  const named = pages
    .map((p, i) => `  @page kpch${i} { size: ${p.PW}px ${p.PH}px; margin: 0; }\n  .kp-pg${i} { page: kpch${i}; }`)
    .join('\n')
  // 第 1 页不强制分页（否则首张前面会多出一张空白纸）
  const breaks = pages
    .map((_, i) => (i ? `  .kp-pg${i} { break-before: page; }` : ''))
    .filter(Boolean)
    .join('\n')
  return `@media print {
  @page { size: ${fallback}px ${fallback}px; margin: 0; }
${named}
${breaks}
  html, body { margin: 0 !important; padding: 0 !important; background: #fff !important; overflow: visible !important; }
  body > *:not(.kp-print-root) { display: none !important; }
  .kp-print-root { position: static !important; left: auto !important; top: auto !important; }
  /* 页盒已按内容算准，break-inside 兜住浮点取整带来的 ±1px 溢出 */
  .kp-page { break-inside: avoid; overflow: hidden; }
  * { -webkit-print-color-adjust: exact !important; print-color-adjust: exact !important; }
}`
}

/**
 * 排好版并装上打印样式，返回 cleanup。与 printKnowledgePdf 分开是为了能在无头
 * Chrome 里验证：`--print-to-pdf` 自己会打印，不需要走 window.print()。
 */
export async function prepareKnowledgePdf(
  items: any[],
  subjectName: string,
  onProgress?: (stage: 'layout' | 'print') => void,
): Promise<{ cleanup: () => void; pages: { PW: number; PH: number }[] }> {
  onProgress?.('layout')
  // 打印不放宽含图列：图片由打印管线按原始分辨率嵌入，撑宽只会把纸撑大，不增清晰度
  const plan = await planLayout(items)
  const { wrapper } = plan
  const total = plan.chapters.length
  const pages: { PW: number; PH: number }[] = []
  let styleEl: HTMLStyleElement | null = null
  let cleaned = false
  const cleanup = () => {
    if (cleaned) return
    cleaned = true
    styleEl?.remove()
    wrapper.remove()
  }

  try {
    // 每章要几张纸：正常一张（整章一段），超过纸张上限的极端章多张（切段）。张数只由树高
    // 决定，与纸宽无关；页眉文案要写总张数，所以先切段数张数，渲染时再用同一批段——不重算。
    const jobs = plan.chapters.map((ch) => ({ ch, bands: chapterBands(ch.node) }))
    const totalPages = jobs.reduce((s, j) => s + j.bands.length, 0)
    let pi = 0 // 纸的连续序号：@page kpch{pi} 每张纸要有唯一名字，续纸不能跟上一张同名
    for (const [ci, { ch, bands }] of jobs.entries()) {
      // 页眉的页码是「这一张纸」的号，不是「该章第一张」的号：切段时同章多张纸各报各的
      // （踩过：按章算一次，同章 9 张纸全写成「第 11/36 页」）。
      const metaOf = (no: number) =>
        `软考星图 · ${esc(subjectName)} 知识导图 · 共 ${items.length} 个知识点 · 第 ${no + 1}/${totalPages} 页`
      // 两段分开：*Inner 是盒里的内容（量尺寸用），*Html 是带 class 的整盒（渲染用）。
      // 居中只在打印路径写这条内联样式：章名是这一页的标题，靠左会和首张卡片挤在同一个角上；
      // 位图长图是另一张脸面（那儿的页眉是整个导图的名字，不是章名），要动得单独说。
      const headerInner = (no: number) =>
        `<div class="kp-doc-title">${esc(ch.name)}</div>` +
        `<div class="kp-doc-meta">${metaOf(no)}</div>` +
        `<div style="height:1px;background:#e5e7eb;margin-top:10px;"></div>`
      const footerInner =
        `<span>软考星图 · ${esc(subjectName)} 知识导图</span>` +
        `<span>章节 ${ci + 1}/${total} · 共 ${items.length} 个知识点</span>`
      const headerOf = (no: number) =>
        `<div class="kp-page-header" style="text-align:center;">${headerInner(no)}</div>`
      const footerHtml = `<div class="kp-page-footer">${footerInner}</div>`
      // 纸宽 = 该章内容右边界，但不得窄于页眉页脚（它们整幅纸宽呈现且不换行，窄了会压到内容上）。
      // 页眉量宽用该章最后一张的页码（位数最多 ⇒ 同章各纸等宽）；一章一张时它就是你看到的那张。
      const pageW = Math.max(
        chapterContentW(ch.node) + 8,
        measureNowrap(wrapper, 'kp-doc-title', esc(ch.name)) + 8,
        measureNowrap(wrapper, 'kp-doc-meta', metaOf(pi + bands.length - 1)) + 8,
        measureNowrap(wrapper, 'kp-page-footer', footerInner),
      )
      // 内容顶偏移 = 页眉实测高 + 呼吸位：页眉是自然高（章名/副标题/分隔线随字体度量），
      // 写死常量会让首张卡片顶进页眉（见 measureBlockH 注释）。必须在 pageW 之后量。
      const contentTop = measureBlockH(wrapper, 'kp-page-header', headerInner(pi), pageW) + HEADER_GAP
      for (const band of bands) {
        const H = band.h + BAND_PAD + contentTop + FOOTER_H
        const page = renderPage(wrapper, pageW, [[band]], pageW, H, headerOf(pi), footerHtml, contentTop)
        page.classList.add(`kp-pg${pi}`)
        pages.push({ PW: Math.ceil(pageW), PH: Math.ceil(H) })
        pi++
      }
    }

    styleEl = document.createElement('style')
    styleEl.textContent = printCss(pages)
    document.head.appendChild(styleEl)
    wrapper.classList.add('kp-print-root')

    // 打印前必须等图片真正解码完，否则 PDF 里会出现空白图框
    await decodeImages(wrapper)
    onProgress?.('print')
    return { cleanup, pages }
  } catch (e) {
    cleanup()
    throw e
  }
}

/**
 * 浏览器打印导出【一章一页】的矢量 PDF：同一套排版渲染成 DOM，每章一张命名纸，
 * 交给浏览器打印管线（用户在打印对话框选「另存为 PDF」）。
 *
 * 为什么这条路清晰而位图不行：打印管线不经过 canvas —— 文字以真矢量写入 PDF，
 * 任意放大不糊；图片按原始分辨率嵌入（源图多少像素就嵌多少像素）。位图路线受
 * 画布面积上限锁死，310 卡片规模下 scale 只有 1.09，图片只剩源图 49%。
 *
 * 为什么不重编码成 JPEG（f4af2a4 时代为 17MB→10MB 加的）：用户明确不追求体积，
 * 无损嵌入 = 图片最清晰。要压体积就把内容图重编码为 JPEG 再打印，其余不动。
 */
export async function printKnowledgePdf(
  items: any[],
  subjectName: string,
  onProgress?: (stage: 'layout' | 'print') => void,
): Promise<void> {
  const { cleanup } = await prepareKnowledgePdf(items, subjectName, onProgress)
  try {
    window.addEventListener('afterprint', cleanup, { once: true })
    window.print()
    // window.print() 在 Chrome/Safari 上阻塞到对话框关闭，到这里即可清理；
    // 异步实现由 afterprint 兜住，再加一个超时防止 DOM 永久残留。
    cleanup()
    setTimeout(cleanup, 60_000)
  } catch (e) {
    cleanup()
    throw e
  }
}

/**
 * 导出整树为一张 PNG：先把各章节切成段，再均衡排进多列，
 * 在「单边 ≤ MAX_SIDE、面积 ≤ AREA_CAP」内选缩放比最大（最清晰）的列数方案。
 * 画布像素总量是定死的（面积 ≤ 214M），所以清晰度全看像素怎么分配：
 * 段高上限（BAND_CAP）让列更均衡抬 scale，含图列宽（IMAGE_COL_BOOST）把像素挪给图片。
 * 单张图看不到「完整又清晰」的份上，要清晰得走上面的 PDF 路线。
 */
export async function exportKnowledgeImage(
  items: any[],
  subjectName: string,
  onProgress?: (done: number, total: number) => void,
): Promise<string> {
  const plan = await planLayout(items, { imageColBoost: IMAGE_COL_BOOST })
  const { wrapper, pageW, colCap } = plan

  try {
    // —— 1. 切段 + 多列编排：在画布上限内选缩放比最大的方案 ——
    // 每章先切段：整章装得下一列就整章一段；装不下按分支切，续段重复章节根。
    // 段高上限取 BAND_CAP 而非 colCap：切碎让各列更均衡，scale 才上得去。
    const bandCap = Math.min(BAND_CAP, colCap)
    const bands: PagePlan[] = []
    for (const ch of plan.chapters) bands.push(...splitByBranches(ch.node, bandCap))

    // 页眉内容先备好、先量：它整幅纸宽一行呈现，「章名 + 副标题 + 分隔线」的自然高实测 91px，
    // 而名义常量只有 74px —— 差的 17px 就是首张卡片顶部压在页眉里的那截。内容顶偏移得用实测值，
    // 所以量这一步要放在算 H 之前（H 里必须留出真实偏移，否则内容被推出纸底）。
    // 量宽用 pageW：C 越大纸越宽，而一行文案在 W ≥ pageW 的纸上高度相同；只有「页眉文案比整列
    // 还宽」时才会量出多一行 —— 那个方向是纸偏高、留白多一点，不会裁内容。
    const headerInner =
      `<div class="kp-doc-title">软考星图 · ${esc(subjectName)} 知识导图</div>` +
      `<div class="kp-doc-meta">共 ${items.length} 个知识点 · 生成于 ${new Date().toLocaleDateString('zh-CN')} · 会员专属导出</div>` +
      `<div style="height:1px;background:#e5e7eb;margin-top:10px;"></div>`
    // 居中与打印路径同一处理：页眉是这张图的「标题」，靠左会和首张卡片挤在同一个角上。
    const headerHtml = `<div class="kp-page-header" style="text-align:center;">${headerInner}</div>`
    const contentTop = measureBlockH(wrapper, 'kp-page-header', headerInner, pageW) + HEADER_GAP

    const maxCols = Math.max(1, Math.floor(MAX_SIDE / pageW))
    let best: { colBands: PagePlan[][]; W: number; H: number; scale: number } | null = null
    for (let C = 1; C <= Math.min(MAX_COLS, maxCols); C++) {
      const colBands = packColumns(bands, C, colCap)
      if (!colBands) continue
      const W = C * pageW
      const H = Math.max(...colBands.map(colHeight)) + contentTop + FOOTER_H
      if (H > MAX_SIDE) continue
      const scale = Math.min(2, MAX_SIDE / H, MAX_SIDE / W, Math.sqrt(AREA_CAP / (W * H)))
      if (!best || scale > best.scale) best = { colBands, W, H, scale }
      if (best.scale >= 1.9) break
    }
    if (!best) {
      // 兜底：内容多到列数上限仍装不下时，允许 scale < 1 强缩进画布上限
      const C = Math.min(MAX_COLS, maxCols)
      const colBands = packColumns(bands, C, Number.POSITIVE_INFINITY) || [[]]
      const W = C * pageW
      const H = Math.max(...colBands.map(colHeight)) + contentTop + FOOTER_H
      best = { colBands, W, H, scale: Math.min(2, MAX_SIDE / H, MAX_SIDE / W, Math.sqrt(AREA_CAP / (W * H))) }
    }

    // —— 2. 渲染单页并截图 ——
    const { colBands, W, H, scale } = best
    const page = renderPage(
      wrapper,
      pageW,
      colBands,
      W,
      H,
      headerHtml,
      `<div class="kp-page-footer"><span>软考星图 · ${esc(subjectName)} 知识导图</span><span>共 ${items.length} 个知识点</span></div>`,
      contentTop, // 实测页眉高 + 呼吸位（见上面 headerInner/measureBlockH 的注释）
    )

    onProgress?.(0, 1)
    const cv = await html2canvas(page, {
      scale,
      useCORS: true,
      allowTaint: true,
      logging: false,
      backgroundColor: '#ffffff',
    })
    onProgress?.(1, 1)
    return cv.toDataURL('image/png')
  } finally {
    wrapper.remove()
  }
}
