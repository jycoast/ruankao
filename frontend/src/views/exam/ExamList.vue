<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { examApi } from '@/api'
import { useSubjectStore } from '@/stores/subject'
import { ElMessage } from 'element-plus'
import jsPDF from 'jspdf'
import html2canvas from 'html2canvas'
import { renderMd, renderMdInline } from '@/utils/markdown'
import { requireMember, isRecentPaperYear } from '@/utils/memberGate'
import PaperCard from '@/components/exam/PaperCard.vue'
import type { Paper } from '@/types'

const router = useRouter()
const subjectStore = useSubjectStore()

const papers = ref<Paper[]>([])
const loading = ref(false)
const exportingYear = ref(0)

// Filters
const activeType = ref(0) // 0=全部, 1=综合知识, 2=案例分析, 3=论文
const selectedYear = ref(0) // 0=全部年份

const typeOptions = [
  { label: '全部', value: 0 },
  { label: '综合知识', value: 1 },
  { label: '案例分析', value: 2 },
  { label: '论文', value: 3 },
]

const yearOptions = computed(() => {
  const years = new Set(filteredBySubject.value.map(p => p.year).filter(y => y > 2000))
  return [0, ...Array.from(years).sort((a, b) => b - a)]
})

const filteredBySubject = computed(() => {
  return papers.value.filter(p => p.subject_id === subjectStore.currentSubjectId)
})

const filteredPapers = computed(() => {
  let result = filteredBySubject.value
  if (activeType.value > 0) {
    result = result.filter(p => p.paper_type === activeType.value)
  }
  if (selectedYear.value > 0) {
    result = result.filter(p => p.year === selectedYear.value)
  }
  return result
})

interface BatchSection {
  batch: number
  papers: Paper[]
}

interface YearGroup {
  year: number
  batches: BatchSection[] | null // null=单次年（无小节，直接平铺）；非 null=两考年（按批次分小节）
  papers: Paper[] // 单次年时的卡片列表
}

const groupedPapers = computed<YearGroup[]>(() => {
  const groups: Record<number, Paper[]> = {}
  for (const p of filteredPapers.value) {
    if (!groups[p.year]) groups[p.year] = []
    groups[p.year].push(p)
  }
  const typeOrder: Record<number, number> = { 1: 1, 2: 2, 3: 3 }
  const sortPapers = (items: Paper[]) =>
    [...items].sort(
      (a, b) =>
        (a.batch || 0) - (b.batch || 0) ||
        (typeOrder[a.paper_type] || 99) - (typeOrder[b.paper_type] || 99)
    )
  return Object.entries(groups)
    .sort((a, b) => Number(b[0]) - Number(a[0]))
    .map(([year, items]) => {
      // 两考年判定：该年存在 batch=1(上半年)或 2(下半年)
      const hasSession = items.some(p => (p.batch || 0) === 1 || (p.batch || 0) === 2)
      if (!hasSession) {
        // 单次年：保持现状，年标题下直接平铺
        return { year: Number(year), batches: null, papers: sortPapers(items) }
      }
      // 两考年：按批次分小节（单次 → 上半年 → 下半年，仅列存在的批次）
      const byBatch: Record<number, Paper[]> = {}
      for (const p of items) {
        const b = p.batch || 0
        if (!byBatch[b]) byBatch[b] = []
        byBatch[b].push(p)
      }
      return {
        year: Number(year),
        batches: [0, 1, 2]
          .filter(b => byBatch[b])
          .map(b => ({ batch: b, papers: sortPapers(byBatch[b]) })),
        papers: [],
      }
    })
})

// 年份下试卷总数（两考年按小节求和，单次年直接用列表）
function yearTotal(group: YearGroup) {
  return group.batches ? group.batches.reduce((s, b) => s + b.papers.length, 0) : group.papers.length
}

async function load() {
  loading.value = true
  try {
    const res: any = await examApi.listPapers({ page: 1, page_size: 200 })
    papers.value = res.data.items || []
  } catch {}
  loading.value = false
}

onMounted(load)

watch(() => subjectStore.currentSubjectId, () => {
  selectedYear.value = 0
  activeType.value = 0
})

// 考试批次：0=单次(2023 及以前每年一考) 1=上半年 2=下半年（2024 软考改革后一年两考）
function batchLabel(b?: number) {
  return { 0: '单次', 1: '上半年', 2: '下半年' }[b || 0] || ''
}

function batchColor(b?: number) {
  return {
    0: 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300',
    1: 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400',
    2: 'bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-400',
  }[b || 0] || ''
}

async function exportYearPdf(year: number) {
  if (!requireMember('pdfExport')) return
  const yearPapers = filteredBySubject.value.filter((p) => p.year === year)
  if (!yearPapers.length) {
    ElMessage.warning('\u672a\u627e\u5230\u8be5\u5e74\u4efd\u8bd5\u5377')
    return
  }
  const hasEssay = subjectStore.hasEssay
  const typeOrder = [1, 2].concat(hasEssay ? [3] : [])
  // sectionLabels 仍按 paper_type 下标索引，供解析区按大类取标签用
  const sectionLabels = ['\u4e00\u3001\u7efc\u5408\u77e5\u8bc6', '\u4e8c\u3001\u6848\u4f8b\u5206\u6790', '\u4e09\u3001\u8bba\u6587']
  const questionsByType: Record<number, any[]> = { 1: [], 2: [], 3: [] }
  try {
    exportingYear.value = year
    const responses = await Promise.all(yearPapers.map((p) => examApi.getPaper(p.id)))
    for (const res of responses) {
      const paper = (res as any).data
      for (const q of paper.questions || []) {
        const t = paper.paper_type as number
        if (t === 1 || t === 2 || t === 3) {
          questionsByType[t].push({ ...q, _type: t, _batch: (paper.batch || 0) })
        }
      }
    }
    const batchName = (b: number) => (b === 1 ? '上半年' : b === 2 ? '下半年' : '')
    const multiBatch = new Set(yearPapers.map((p) => p.batch || 0)).size > 1
    for (const t of typeOrder) questionsByType[t].sort((a: any, b: any) => (a._batch || 0) - (b._batch || 0))
    const safe = (s: string) => (s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    const effLen = (s: string) => { let n = 0; for (const ch of s) n += ch.charCodeAt(0) > 255 ? 2 : 1; return n } // 中文按 2 计宽
    const style = document.createElement('style')
    style.textContent = '.pdf-section h1,.pdf-section h2,.pdf-section h3,.pdf-section h4{margin:8px 0 4px;line-height:1.4}.pdf-section p{margin:6px 0}.pdf-section ul,.pdf-section ol{margin:6px 0 6px 20px}.pdf-section li{margin:3px 0}.pdf-section pre{background:#f6f8fa;border:1px solid #e5e7eb;border-radius:6px;padding:10px 12px;overflow-x:auto;white-space:pre-wrap;word-wrap:break-word;font-size:11px}.pdf-section code{background:#f3f4f6;padding:1px 4px;border-radius:4px;font-size:11px}.pdf-section blockquote{margin:12px 0;padding:6px 12px;border-left:3px solid #d1d5db;color:#374151;background:#fafafa}.pdf-section table{border-collapse:collapse;margin:12px 0;width:100%}.pdf-section th,.pdf-section td{border:1px solid #d1d5db;padding:6px 8px;text-align:left}.pdf-section img{max-width:100%;max-height:520px;object-fit:contain;display:block;margin:12px auto;background:#fff;border:1px solid #eef0f3;border-radius:4px;padding:2px}'
    let html = ''
    html += '<div class="pdf-section-group">'
    html += '<div class="pdf-section" style="text-align:center;margin-bottom:16px;">'
    html += '<div style="font-size:22px;font-weight:700;">' + safe(year + ' \u5e74\u8f6f\u8003\u771f\u9898\u4e0e\u89e3\u6790') + '</div>'
    const coverStats = '\u7efc\u5408\u77e5\u8bc6 ' + questionsByType[1].length + ' \u9898 \u00b7 \u6848\u4f8b\u5206\u6790 ' + questionsByType[2].length + ' \u9898' + (hasEssay ? ' \u00b7 \u8bba\u6587 ' + questionsByType[3].length + ' \u9898' : '')
    html += '<div style="font-size:12px;color:#555;margin-top:6px;">' + coverStats + '</div>'
    html += '</div>'
    html += '</div>'
    // 按批次分块：先第一批次的完整卷（题目+解析），再下一批次；单批次年份只有一块
    const batches = Array.from(new Set(yearPapers.map((p) => p.batch || 0))).sort((a, b) => a - b)
    for (const bt of batches) {
      if (multiBatch) {
        html += '<div class="pdf-section-group">'
        html += '<div class="pdf-section" style="font-size:18px;font-weight:700;color:#2563eb;margin:24px 0 4px;border-bottom:2px solid #2563eb;padding-bottom:4px;">' + safe(year + ' 年' + batchName(bt)) + '</div>'
        html += '</div>'
      }
      for (let i = 0; i < typeOrder.length; i++) {
        const type = typeOrder[i]
        const list = questionsByType[type].filter((q: any) => (q._batch || 0) === bt)
        html += '<div class="pdf-section-group">'
        html += '<div class="pdf-section" style="font-size:16px;font-weight:700;margin:18px 0 8px;">' + safe(sectionLabels[i]) + '</div>'
        if (!list.length) {
          html += '<div class="pdf-section" style="color:#666;margin-bottom:10px;">暂无题目</div>'
          html += '</div>'
          continue
        }
        let qIndex = 0 // 每个大类内独立编号，从 1 开始
        for (const q of list) {
          qIndex += 1
          const scoreText = q.score ? '（' + q.score + ' 分）' : ''
          const stem = q.content || q.title || ''
          const stemHtml = stem ? renderMd(stem) : ''
          const numLabel = '<strong>' + qIndex + '.</strong> '
          // 把序号注入题干首个 <p>，使序号与首行文字同行；无 <p> 打头时才单独占行
          let numberedStem = stemHtml
            ? (/^\s*<p>/i.test(stemHtml)
                ? stemHtml.replace(/^\s*<p>/i, '<p>' + numLabel)
                : '<p style="margin:6px 0;">' + numLabel + '</p>' + stemHtml)
            : '<p style="margin:6px 0;">' + numLabel + '</p>'
          // 分值追加在题干末尾同行，不单独占行
          if (scoreText) {
            const scoreHtml = '<span style="color:#374151;">' + scoreText + '</span>'
            numberedStem = /<\/p>\s*$/i.test(numberedStem)
              ? numberedStem.replace(/<\/p>\s*$/i, scoreHtml + '</p>')
              : numberedStem + scoreHtml
          }
          html += '<div class="pdf-section" style="margin:10px 0 4px;">' + numberedStem + '</div>'
          // 案例分析(2)与论文(3)题干后留答题区：打印后便于手写作答。
          // 案例题题干内含多个小问，无法靠通用 markdown 精确定位每个小问插空，
          // 这里在题干整块下方统一留一块答题区，作为一个 .pdf-section 参与分页，
          // 切片时会被整体下移到下一页，不会被切坏。
          if (type === 2 || type === 3) {
            // 多条灰色虚线作为书写行；高度约容纳 6~8 行手写
            const lines = Array.from({ length: 7 }, () => '<div style="height:26px;border-bottom:1px dashed #cbd5e1;margin:0 4px;"></div>').join('')
            html += '<div class="pdf-section" style="margin:6px 0 12px;">'
            html += '<div style="display:flex;align-items:baseline;gap:6px;margin-bottom:2px;"><span style="font-weight:600;">答：</span><span style="color:#9ca3af;font-size:11px;">在虚线上作答</span></div>'
            html += '<div style="border:1px solid #eef0f3;border-radius:6px;padding:6px 4px;background:#fcfcfd;">' + lines + '</div>'
            html += '</div>'
          }
          if (type !== 3) {
            const options = (q.options || []).slice().sort((a: any, b: any) => (a.sort_order ?? a.id) - (b.sort_order ?? b.id))
            if (options.length) {
              // 短选项同行紧凑排布：按最长选项宽度决定每行列数（含图片/多行时退回每行一个）
              const texts = options.map((o: any) => String(o.content || ''))
              const hasMedia = texts.some((t: string) => t.includes('![') || t.includes('<img') || t.includes('\n'))
              const maxLen = Math.max(...texts.map(effLen))
              const perRow = hasMedia || maxLen > 32 ? 1 : maxLen > 14 ? 2 : 4
              html += '<div class="pdf-section" style="margin:4px 0 8px 18px;">'
              for (let oi = 0; oi < options.length; oi++) {
                const label = options[oi].label || ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H'][oi] || String(oi + 1)
                const body = label + '. ' + renderMdInline(options[oi].content || '')
                html += perRow === 1
                  ? '<div style="margin:4px 0;">' + body + '</div>'
                  : '<span style="display:inline-block;width:' + (100 / perRow) + '%;vertical-align:top;margin:3px 0;box-sizing:border-box;padding-right:8px;">' + body + '</span>'
              }
              html += '</div>'
            }
          }
        }
        html += '</div>'
      }
      // 本批次的解析紧随本批次题目之后
      html += '<div class="pdf-section-group">'
      html += '<div class="pdf-section" style="font-size:16px;font-weight:700;margin:22px 0 8px;">' + safe(hasEssay ? '四、题目解析' : '三、题目解析') + '</div>'
      for (const type of typeOrder) {
        const list = questionsByType[type].filter((q: any) => (q._batch || 0) === bt)
        if (!list.length) continue
        html += '<div class="pdf-section" style="font-size:12px;font-weight:600;color:#2563eb;margin:8px 0 2px;">【' + safe(sectionLabels[type - 1].slice(2)) + '】</div>'
        const answers = list.map((q: any) => type !== 3
          ? (q.options || []).filter((o: any) => o.is_correct === 1).map((o: any) => o.label).join('')
          : '')
        // 综合知识开头放答案速查表，每 5 题一组
        if (type === 1 && answers.some(Boolean)) {
          html += '<div class="pdf-section" style="margin:4px 0 10px;padding:6px 10px;background:#f8fafc;border:1px solid #e5e7eb;border-radius:6px;">'
          for (let s = 0; s < list.length; s += 5) {
            const seg = answers.slice(s, s + 5).map((a: string) => a || '-').join('')
            html += '<span style="display:inline-block;width:20%;margin:2px 0;"><span style="color:#6b7280;">' + (s + 1) + '-' + Math.min(s + 5, list.length) + '</span> <strong>' + safe(seg) + '</strong></span>'
          }
          html += '</div>'
        }
        let aIndex = 0 // 解析区同样按大类独立编号，与题目区一一对应
        for (const q of list) {
          aIndex += 1
          const answer = answers[aIndex - 1]
          const explanation = q.explanation || q.analysis || ''
          if (!explanation) {
            // 无解析文字：合并为一行
            html += '<div class="pdf-section" style="margin:3px 0;"><strong>' + aIndex + '.</strong>' + (answer ? '【答案】' + safe(answer) : ' <span style="color:#666;">暂无解析</span>') + '</div>'
            continue
          }
          html += '<div class="pdf-section" style="margin:6px 0 10px;">'
          html += '<div style="font-weight:600;">' + aIndex + '.' + (answer ? '【答案】' + safe(answer) : '') + '</div>'
          html += '<div style="margin-top:2px;"><strong>【解析】</strong>' + renderMd(explanation) + '</div>'
          html += '</div>'
        }
      }
      html += '</div>'
    }
    const pdf = new jsPDF('p', 'mm', 'a4')
    const pageWidth = pdf.internal.pageSize.getWidth()
    const pageHeight = pdf.internal.pageSize.getHeight()
    const margin = 12
    const contentTop = 16 // \u9876\u90e8\u7559\u767d\uff0c\u653e\u9875\u7709
    const footer = 8 // \u5e95\u90e8\u7559\u767d\uff0c\u653e\u9875\u7801
    const renderWidth = pageWidth - margin * 2
    const contentHeightMM = pageHeight - contentTop - footer
    // \u6574\u7bc7\u4e00\u6b21\u6027\u79bb\u5c4f\u6e32\u67d3\uff0c\u518d\u6309\u6574\u9875\u9ad8\u5ea6\u5207\u7247
    const wrapper = document.createElement('div')
    wrapper.style.position = 'absolute'
    wrapper.style.left = '-9999px'
    wrapper.style.top = '0'
    wrapper.style.width = '880px'
    wrapper.style.padding = '28px 32px'
    wrapper.style.background = '#ffffff'
    wrapper.style.color = '#111111'
    wrapper.style.fontFamily = 'Microsoft YaHei, PingFang SC, Helvetica Neue, Arial, sans-serif'
    wrapper.style.lineHeight = '1.8'
    wrapper.style.fontSize = '12px'
    wrapper.innerHTML = '<style>' + style.textContent + '</style>' + html
    document.body.appendChild(wrapper)
    try {
      // images are now local (/images/...); wait for them so html2canvas captures them
      const _imgs = Array.from(wrapper.querySelectorAll('img')) as HTMLImageElement[]
      await Promise.all(_imgs.map((img) => img.complete && img.naturalWidth > 0
        ? Promise.resolve()
        : new Promise<void>((res) => { img.onload = () => res(); img.onerror = () => res(); setTimeout(() => res(), 6000) })))
      // \u5206\u5757\u6e32\u67d3\uff1a\u6574\u7bc7\u4e00\u6b21\u6027\u6e32\u67d3\u65f6 canvas \u9ad8\u5ea6\u4f1a\u8d85\u51fa\u6d4f\u89c8\u5668\u4e0a\u9650(\u7ea616384px)\u800c\u53d8\u6210\u7a7a\u767d\u9875\uff1b
      // \u800c\u6bcf\u4e2a .pdf-section \u5404\u8c03\u4e00\u6b21 html2canvas \u53c8\u4f1a\u56e0\u6bcf\u6b21\u514b\u9686\u6574\u4e2a\u6587\u6863\u5bfc\u81f4\u6781\u6162(\u5361\u6b7b)\u3002
      // \u6298\u4e2d\uff1a\u6309\u7d2f\u8ba1\u9ad8\u5ea6\u628a\u5757\u5206\u6210\u82e5\u5e72\u5927\u5757\uff0c\u6bcf\u5927\u5757\u53ea\u8c03\u4e00\u6b21 html2canvas\uff0c\u518d\u8fde\u7eed\u6392\u7248\u5206\u9875\u3002
      const styleText = style.textContent || ''
      const blockEls = Array.from(wrapper.querySelectorAll('.pdf-section')) as HTMLElement[]
      const heights = blockEls.map((el) => el.offsetHeight)
      const CAP = 5000 // \u5355\u5757 CSS \u9ad8\u5ea6\u4e0a\u9650\uff0c\u00d73 \u540e\u4ecd\u4f4e\u4e8e\u6d4f\u89c8\u5668 canvas \u4e0a\u9650 16384
      const chunks: HTMLElement[][] = []
      {
        let cur: HTMLElement[] = []
        let curH = 0
        for (let i = 0; i < blockEls.length; i++) {
          const h = heights[i] || 0
          if (cur.length && curH + h > CAP) { chunks.push(cur); cur = []; curH = 0 }
          cur.push(blockEls[i]); curH += h + 20
        }
        if (cur.length) chunks.push(cur)
      }
      // \u628a\u5757\u79fb\u5165\u72ec\u7acb\u7684\u5206\u5757\u5bb9\u5668\uff08\u8131\u79bb\u4e3b\u5bb9\u5668\uff0c\u4f7f\u6bcf\u6b21\u622a\u56fe\u53ea\u514b\u9686\u5f53\u524d\u5206\u5757\uff09
      const chunkWrappers = chunks.map((els) => {
        const cw = document.createElement('div')
        cw.style.cssText = wrapper.style.cssText
        cw.innerHTML = '<style>' + styleText + '</style>'
        for (const el of els) cw.appendChild(el)
        return cw
      })
      const gapMM = 1.2
      let cursorY = 0 // \u5f53\u524d\u9875\u5df2\u5360\u7528\u9ad8\u5ea6(mm)
      // \u5728\u8fd1\u4f3c\u7eaf\u767d\u7684\u6574\u884c\u5904\u5206\u9875\uff0c\u907f\u514d\u7a7f\u8fc7\u6587\u5b57
      const findCleanCut = (cv: HTMLCanvasElement, cctx: CanvasRenderingContext2D, start: number, target: number) => {
        const maxBack = Math.max(1, Math.floor((target - start) * 0.22))
        const bandTop = Math.max(start + 1, target - maxBack)
        const bandH = target - bandTop
        if (bandH <= 0) return target
        const band = cctx.getImageData(0, bandTop, cv.width, bandH).data
        for (let y = target - 1; y >= bandTop; y--) {
          const rowOff = (y - bandTop) * cv.width * 4
          let white = true
          for (let x = 0; x < cv.width; x++) {
            const o = rowOff + x * 4
            if (band[o] < 245 || band[o + 1] < 245 || band[o + 2] < 245) { white = false; break }
          }
          if (white) return y
        }
        return target
      }
      // \u628a\u4e00\u5f20\u6574\u5bbd canvas \u8fde\u7eed\u6392\u5165 PDF\uff0c\u8de8\u9875\u65f6\u4ece\u4e0a\u4e00\u9875\u6e38\u6807\u5904\u8854\u63a5
      const placeFlow = (cv: HTMLCanvasElement) => {
        const cctx = cv.getContext('2d')!
        const pxPerMM = cv.width / renderWidth
        let sy = 0
        while (sy < cv.height) {
          let availPx = Math.floor((contentHeightMM - cursorY) * pxPerMM)
          if (availPx < 60) { pdf.addPage(); cursorY = 0; availPx = Math.floor(contentHeightMM * pxPerMM) }
          let target = Math.min(sy + availPx, cv.height)
          if (target < cv.height) { const c = findCleanCut(cv, cctx, sy, target); if (c > sy) target = c }
          const sliceH = target - sy
          if (sliceH <= 0) { pdf.addPage(); cursorY = 0; continue }
          const pageCanvas = document.createElement('canvas')
          pageCanvas.width = cv.width
          pageCanvas.height = sliceH
          const pctx = pageCanvas.getContext('2d')!
          pctx.fillStyle = '#ffffff'
          pctx.fillRect(0, 0, cv.width, sliceH)
          pctx.drawImage(cv, 0, sy, cv.width, sliceH, 0, 0, cv.width, sliceH)
          const hmm = sliceH / pxPerMM
          pdf.addImage(pageCanvas.toDataURL('image/jpeg', 0.92), 'JPEG', margin, contentTop + cursorY, renderWidth, hmm)
          cursorY += hmm
          sy = target
          if (sy < cv.height) { pdf.addPage(); cursorY = 0 }
        }
      }
      for (const cw of chunkWrappers) {
        document.body.appendChild(cw)
        try {
          const imgs = Array.from(cw.querySelectorAll('img')) as HTMLImageElement[]
          await Promise.all(imgs.map((img) => img.complete && img.naturalWidth > 0
            ? Promise.resolve()
            : new Promise<void>((res) => { img.onload = () => res(); img.onerror = () => res(); setTimeout(() => res(), 6000) })))
          const cv = await html2canvas(cw, { scale: 3, useCORS: true, allowTaint: true, logging: false, backgroundColor: '#ffffff' })
          if (cv.width && cv.height) placeFlow(cv)
          cursorY += gapMM
        } finally {
          document.body.removeChild(cw)
        }
      }
      // \u9875\u7709 + \u9875\u7801\uff1ajsPDF \u5185\u7f6e\u5b57\u4f53\u4e0d\u652f\u6301\u4e2d\u6587\uff0c\u7528 canvas \u6e32\u67d3\u6587\u5b57\u540e\u8d34\u56fe
      const textImage = (text: string, fontPx: number, color: string) => {
        const mc = document.createElement('canvas')
        const font = fontPx + 'px Microsoft YaHei, PingFang SC, Helvetica Neue, Arial, sans-serif'
        let mctx = mc.getContext('2d')!
        mctx.font = font
        const tw = Math.ceil(mctx.measureText(text).width) + 4
        const th = Math.ceil(fontPx * 1.5)
        mc.width = tw * 2
        mc.height = th * 2
        mctx = mc.getContext('2d')!
        mctx.scale(2, 2)
        mctx.font = font
        mctx.fillStyle = color
        mctx.textBaseline = 'middle'
        mctx.fillText(text, 2, th / 2)
        return { url: mc.toDataURL('image/png'), ratio: tw / th, th }
      }
      const totalPages = pdf.getNumberOfPages()
      const headerImg = textImage(year + ' \u5e74\u8f6f\u8003\u771f\u9898\u4e0e\u89e3\u6790', 12, '#9ca3af')
      const headerH = 3.2 // mm
      for (let p = 1; p <= totalPages; p++) {
        pdf.setPage(p)
        if (p > 1) {
          // \u9875\u7709\uff1a\u6807\u9898 + \u5206\u9694\u7ec6\u7ebf\uff08\u9996\u9875\u6709\u5927\u6807\u9898\uff0c\u8df3\u8fc7\uff09
          pdf.addImage(headerImg.url, 'PNG', margin, 5, headerH * headerImg.ratio, headerH)
          pdf.setDrawColor(229, 231, 235)
          pdf.setLineWidth(0.2)
          pdf.line(margin, 10.5, pageWidth - margin, 10.5)
        }
        const footImg = textImage('\u7b2c ' + p + ' \u9875 / \u5171 ' + totalPages + ' \u9875', 12, '#9ca3af')
        const footH = 3 // mm
        const footW = footH * footImg.ratio
        pdf.addImage(footImg.url, 'PNG', (pageWidth - footW) / 2, pageHeight - 6.5, footW, footH)
      }
    } finally {
      document.body.removeChild(wrapper)
    }
    pdf.save(year + '-\u771f\u9898\u89e3\u6790.pdf')
  } catch (e: any) {
    ElMessage.error(e?.message || '\u5bfc\u51faPDF\u5931\u8d25')
  } finally {
    exportingYear.value = 0
  }
}

function goToExam(id: number) {
  router.push(`/exams/${id}`)
}
</script>

<template>
  <div class="max-w-screen-2xl mx-auto px-4 py-6">
    <!-- Header -->
    <div class="mb-8">
      <h1 class="text-3xl font-bold text-gray-900 dark:text-gray-100 mb-2">软考真题</h1>
      <p class="text-gray-500 dark:text-gray-400">历年真题模拟，助你备考无忧</p>
    </div>

    <!-- Filters -->
    <div class="bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700 mb-6">
      <div class="flex flex-wrap gap-4 items-center">
        <!-- Type Tabs -->
        <div class="flex gap-2">
          <button
            v-for="t in typeOptions"
            :key="t.value"
            @click="activeType = t.value"
            class="px-4 py-2 rounded-lg text-sm font-medium transition-all"
            :class="activeType === t.value 
              ? 'bg-blue-600 text-white shadow-md' 
              : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'"
          >
            {{ t.label }}
          </button>
        </div>

        <div class="h-6 w-px bg-gray-200 dark:bg-gray-600"></div>

        <!-- Year Filter -->
        <select 
          v-model="selectedYear" 
          class="px-4 py-2 rounded-lg text-sm border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          <option :value="0">全部年份</option>
          <option v-for="y in yearOptions.slice(1)" :key="y" :value="y">{{ y }}年</option>
        </select>

        <div class="ml-auto text-sm text-gray-500">
          共 {{ filteredPapers.length }} 份试卷
        </div>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="text-center py-20 text-gray-500">
      <div class="animate-spin inline-block w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full mb-4"></div>
      <div>加载中...</div>
    </div>

    <!-- Paper Groups by Year -->
    <div v-else-if="groupedPapers.length > 0" class="space-y-8">
      <div v-for="group in groupedPapers" :key="group.year">
        <!-- Year Header -->
        <div class="flex items-center gap-3 mb-4">
          <div class="text-2xl font-bold text-gray-900 dark:text-gray-100">{{ group.year }}</div>
          <div class="flex-1 h-px bg-gray-200 dark:bg-gray-700"></div>
          <div class="flex items-center gap-2 text-sm text-gray-500">
              {{ yearTotal(group) }} 份试卷
              <button
                class="ml-2 px-3 py-1.5 rounded-lg text-xs font-medium bg-emerald-600 text-white hover:bg-emerald-700 disabled:opacity-60"
                :disabled="exportingYear === group.year"
                @click.stop="exportYearPdf(group.year)"
              >
                <span v-if="exportingYear === group.year">导出中..</span>
                <span v-else>导出 PDF</span>
              </button>
            </div>
        </div>

        <!-- 两考年：按批次分小节（2024 起一年两考；早年部分年份也有双批次数据） -->
        <template v-if="group.batches">
          <div v-for="sec in group.batches" :key="sec.batch" class="mb-6 last:mb-0">
            <!-- 小节标题 -->
            <div class="flex items-center gap-2 mb-3">
              <span class="inline-flex items-center rounded-md px-2.5 py-1 text-sm font-semibold" :class="batchColor(sec.batch)">
                {{ batchLabel(sec.batch) }}
              </span>
              <span class="text-xs text-gray-400">{{ sec.papers.length }} 份</span>
              <div class="flex-1 h-px bg-gray-200 dark:bg-gray-700"></div>
            </div>
            <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4 gap-4">
              <PaperCard v-for="paper in sec.papers" :key="paper.id" :paper="paper" @open="goToExam" />
            </div>
          </div>
        </template>
        <!-- 单次年：直接平铺（无批次小节） -->
        <template v-else>
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4 gap-4">
            <PaperCard v-for="paper in group.papers" :key="paper.id" :paper="paper" @open="goToExam" />
          </div>
        </template>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else class="text-center py-20">
      <div class="text-6xl mb-4">📝</div>
      <div class="text-gray-500 text-lg">暂无试卷</div>
    </div>
  </div>
</template>
