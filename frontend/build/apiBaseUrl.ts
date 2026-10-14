/**
 * 构建期解析前端 API 基地址。
 *
 * 为什么要有这个断言：客户端三处消费点都是
 * `import.meta.env.VITE_API_BASE_URL || '/api'`（api/index.ts、utils/aiStream.ts、
 * views/auth/Login.vue）。构建时若该变量为空，兜底会静默生效成**相对路径 /api**，
 * 产物一旦部署到静态托管（Cloudflare Pages 等），所有 /api/* 请求都会打到前端源上、
 * 被 SPA fallback 用 index.html 应答 —— 整页跳转拿到的是「不存在」页，XHR 拿到的是
 * HTML 而不是 JSON。构建日志里没有任何异常，只有上线后才会暴露。
 *
 * 2026-10 线上出过一次：`frontend/.env.production` 被移出版本控制后，新构建直接
 * 内联了兜底值。故在此把「静默降级」改成「响亮的构建失败」。
 */

/** serve（dev）下缺失时的兜底：交给 vite 的 `/api` 代理转发到后端。 */
const DEV_FALLBACK = '/api'

export function resolveApiBaseUrl(command: 'build' | 'serve', rawValue?: string): string {
  const value = (rawValue ?? '').trim()
  if (value) return value

  if (command === 'build') {
    throw new Error(
      '[vite] 构建中止：缺少 VITE_API_BASE_URL。\n' +
        '构建产物必须知道后端地址。缺了它，代码里的 `|| "/api"` 兜底会让所有 API 请求\n' +
        '指向前端自己的源，上线后整站接口全废（且构建日志毫无异常）。\n' +
        '提供方式（任选其一）：\n' +
        '  1. 环境变量 VITE_API_BASE_URL=https://your-api.example.com/api\n' +
        '     （Cloudflare Pages：Settings → Environment variables → Production）\n' +
        '  2. frontend/.env.production 写一行 VITE_API_BASE_URL=...\n' +
        '     模板见 frontend/.env.example。',
    )
  }

  return DEV_FALLBACK
}
