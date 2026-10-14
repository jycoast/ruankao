import { onMounted, onUnmounted } from 'vue'

interface QuizShortcutOptions {
  /** 按 A-H 或 1-8 时触发，label 已规范化为大写字母 */
  onSelect?: (label: string) => void
  /** 回车：通常是"提交答案"或"下一题" */
  onEnter?: () => void
  onPrev?: () => void
  onNext?: () => void
  /** 返回 false 时本次按键忽略（例如弹窗打开、题目未加载） */
  enabled?: () => boolean
}

const LETTERS = 'ABCDEFGH'

function isTypingTarget(el: EventTarget | null) {
  if (!(el instanceof HTMLElement)) return false
  const tag = el.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable
}

// 答题页统一快捷键：A-H / 1-8 选项，Enter 提交或下一题，← → 翻题。
export function useQuizShortcuts(options: QuizShortcutOptions) {
  function handler(e: KeyboardEvent) {
    if (e.ctrlKey || e.metaKey || e.altKey) return
    if (isTypingTarget(e.target)) return
    if (options.enabled && !options.enabled()) return

    const key = e.key
    let label = ''
    if (key.length === 1) {
      const upper = key.toUpperCase()
      if (LETTERS.includes(upper)) label = upper
      else if (key >= '1' && key <= '8') label = LETTERS[Number(key) - 1]
    }

    if (label && options.onSelect) {
      e.preventDefault()
      options.onSelect(label)
      return
    }

    if (key === 'Enter' && options.onEnter) {
      e.preventDefault()
      options.onEnter()
    } else if (key === 'ArrowLeft' && options.onPrev) {
      e.preventDefault()
      options.onPrev()
    } else if (key === 'ArrowRight' && options.onNext) {
      e.preventDefault()
      options.onNext()
    }
  }

  onMounted(() => window.addEventListener('keydown', handler))
  onUnmounted(() => window.removeEventListener('keydown', handler))
}
