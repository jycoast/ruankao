// SSE 流式读取工具。axios 不支持流式，改用 fetch + ReadableStream 解析 OpenAI 兼容 SSE。
// 后端每块以 `data: {json}\n\n` 发送，json 形如 {delta} / {error} / {done}。

const BASE = import.meta.env.VITE_API_BASE_URL || '/api'

export interface StreamHandlers {
  onDelta?: (text: string) => void
  onError?: (message: string) => void
  /**
   * 流结束时调用，且只调用一次。
   * truncated=true 表示正文不完整：撞上了模型的输出上限（后端透传 finish_reason=length），
   * 或者连接被中途切断。这两种情况原本都表现为「输出到一半直接停下」且没有任何提示。
   */
  onDone?: (truncated: boolean) => void
}

/**
 * 向流式端点 POST 并逐块回调。返回一个 abort 函数用于中止。
 */
export function streamPost(path: string, body: unknown, handlers: StreamHandlers): () => void {
  const controller = new AbortController()
  const token = localStorage.getItem('token') || ''

  ;(async () => {
    try {
      const resp = await fetch(BASE + path, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
          Accept: 'text/event-stream',
        },
        body: JSON.stringify(body),
        signal: controller.signal,
      })

      if (!resp.ok || !resp.body) {
        handlers.onError?.(`请求失败（${resp.status}）`)
        return
      }

      const reader = resp.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      // 服务端是否已给出明确结局（done / error）。没有的话说明流是被切断的。
      let settled = false

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        // 按 SSE 事件分隔（\n\n）
        const parts = buffer.split('\n\n')
        buffer = parts.pop() || ''
        for (const part of parts) {
          const line = part.trim()
          if (!line.startsWith('data:')) continue
          const payload = line.slice(5).trim()
          if (!payload) continue
          try {
            const obj = JSON.parse(payload)
            if (obj.delta) handlers.onDelta?.(obj.delta)
            else if (obj.error) {
              settled = true
              handlers.onError?.(obj.error)
            } else if (obj.done) {
              settled = true
              handlers.onDone?.(obj.truncated === true)
            }
          } catch {
            // 忽略无法解析的行
          }
        }
      }
      // 读到流末尾却没收到结局帧：多为连接被中途掐断，已收到的内容是不完整的
      if (!settled) handlers.onDone?.(true)
    } catch (e: any) {
      if (e?.name !== 'AbortError') handlers.onError?.(e?.message || '网络错误')
    }
  })()

  return () => controller.abort()
}
