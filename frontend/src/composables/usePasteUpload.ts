import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { uploadApi } from '@/api'

/**
 * 后台编辑器的剪贴板图片上传。
 *
 * 绑定到 textarea 的 paste 事件：剪贴板里有图片就上传到对象存储，
 * 并在光标处插入 markdown 图片语法；纯文本粘贴不受影响。
 *
 * 之所以自己处理而不是用 el-upload：粘贴场景没有文件选择框，
 * 拿到的是 ClipboardItem，且需要精确控制插入位置。
 */
export function usePasteUpload() {
  const uploading = ref(false)

  /**
   * @param e      textarea 的 paste 事件
   * @param value  当前文本内容
   * @param onUpdate 回写文本（含插入的图片语法）
   */
  async function handlePaste(
    e: ClipboardEvent,
    value: string,
    onUpdate: (next: string) => void,
  ) {
    const items = Array.from(e.clipboardData?.items || [])
    const imageItems = items.filter((it) => it.kind === 'file' && it.type.startsWith('image/'))
    if (!imageItems.length) return // 普通文本粘贴，走浏览器默认行为

    // 复制图片文件（而非截图）时剪贴板会同时带文件名文本，
    // 不拦下来会既插图又插一段路径文字。
    e.preventDefault()

    const el = e.target as HTMLTextAreaElement | HTMLInputElement | null
    // 光标位置必须在 await 之前取：异步期间 textarea 可能已失焦，
    // selectionStart 会变成 0，图片就插到开头去了。
    const start = el?.selectionStart ?? value.length
    const end = el?.selectionEnd ?? value.length

    uploading.value = true
    try {
      const files = imageItems.map((it) => it.getAsFile()).filter((f): f is File => !!f)
      const urls: string[] = []
      for (const f of files) {
        const res: any = await uploadApi.image(f, f.name || 'paste.png')
        if (res?.data?.url) urls.push(res.data.url)
      }
      if (!urls.length) return

      // 前后补换行，避免图片和上下文挤在同一段里
      const snippet = urls.map((u) => `\n![](${u})\n`).join('')
      const next = value.slice(0, start) + snippet + value.slice(end)
      onUpdate(next)

      // 光标移到插入内容之后，方便继续输入
      const caret = start + snippet.length
      requestAnimationFrame(() => {
        el?.focus()
        el?.setSelectionRange?.(caret, caret)
      })
      ElMessage.success(`已插入 ${urls.length} 张图片`)
    } catch {
      // 具体错误信息由 axios 拦截器统一弹出，这里不重复提示
    } finally {
      uploading.value = false
    }
  }

  return { uploading, handlePaste }
}
