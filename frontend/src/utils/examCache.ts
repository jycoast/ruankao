// 未提交的答题进度存 localStorage，刷新/误关页面后可恢复。
export type ExamMode = 'exam' | 'practice'

const CACHE_TTL = 12 * 60 * 60 * 1000 // 12 小时

export interface ExamCache {
  answers: Record<number, any>
  practiceResults: Record<number, { is_correct: boolean | null; correctAnswer: string }>
  currentIndex: number
  timer: number
}

function examCacheKey(paperId: number, mode: ExamMode): string {
  return `exam_session:${paperId}:${mode}`
}

export function loadExamCache(paperId: number, mode: ExamMode): (ExamCache & { ts: number }) | null {
  try {
    const raw = localStorage.getItem(examCacheKey(paperId, mode))
    if (!raw) return null
    const data = JSON.parse(raw)
    if (!data.ts || Date.now() - data.ts > CACHE_TTL) {
      localStorage.removeItem(examCacheKey(paperId, mode))
      return null
    }
    return data
  } catch {
    return null
  }
}

export function saveExamCache(paperId: number, mode: ExamMode, data: ExamCache): void {
  try {
    localStorage.setItem(examCacheKey(paperId, mode), JSON.stringify({ ...data, ts: Date.now() }))
  } catch {}
}

export function clearExamCache(paperId: number, mode: ExamMode): void {
  try {
    localStorage.removeItem(examCacheKey(paperId, mode))
  } catch {}
}

// 只有真正答过题才算「可恢复」，否则进考试页会为一份空进度弹窗。
export function hasResumableExam(paperId: number, mode: ExamMode): boolean {
  const c = loadExamCache(paperId, mode)
  return !!c && !!c.answers && Object.keys(c.answers).length > 0
}
