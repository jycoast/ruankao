import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import { fileURLToPath, URL } from 'node:url'
import { execSync } from 'node:child_process'
import { writeFileSync, mkdirSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import type { Plugin } from 'vite'
import { resolveApiBaseUrl } from './build/apiBaseUrl'

// 构建版本串:git 短 hash + 时间戳。SW 缓存名与 version 绑定,发版即换缓存名,
// activate 时清掉旧 `ruankao-*` 缓存,不会残留上一版本的 hashed 资源。
function getBuildVersion(): string {
  let hash = ''
  try {
    hash = execSync('git rev-parse --short HEAD', {
      encoding: 'utf8',
      cwd: process.cwd(),
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim()
  } catch {
    // 非 git 环境(如 CI 未 checkout 元数据),退回纯时间戳
  }
  return `${hash || 'build'}-${Date.now()}`
}

// PWA:构建结束时生成 dist/manifest.webmanifest + dist/sw.js。
// sw.js 内联 precache 清单:index.html + dist/assets/ 全部 hashed 文件 + 图标。
// 真题图(public/images/)、API 请求一律不入缓存(壳离线边界)。
// 放 closeBundle:CI(无论 npm run build 还是裸 vite build)都会生成。
function pwaPlugin(): Plugin {
  return {
    name: 'pwa',
    apply: 'build',
    closeBundle() {
      const version = getBuildVersion()
      let assetFiles: string[] = []
      try {
        assetFiles = readdirSync(join('dist', 'assets')).filter((f) => !f.endsWith('.map'))
      } catch {
        // dist/assets 不存在(极端空构建)则只 precache 壳入口
      }
      const assetPaths = assetFiles.map((f) => `/assets/${f}`)
      const precache = [
        '/index.html',
        ...assetPaths,
        '/pwa-icon-192.png',
        '/pwa-icon-512.png',
        '/pwa-icon-512-maskable.png',
        '/pwa-icon-180.png',
        '/favicon.png',
        '/manifest.webmanifest',
      ]

      const manifest = {
        name: '软考星图',
        short_name: '软考星图',
        description: '软考题库、历年真题与知识点学习',
        start_url: './',
        scope: './',
        display: 'standalone',
        orientation: 'portrait',
        theme_color: '#3b82f6',
        background_color: '#ffffff',
        icons: [
          { src: '/pwa-icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: '/pwa-icon-512.png', sizes: '512x512', type: 'image/png' },
          {
            src: '/pwa-icon-512-maskable.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable',
          },
        ],
      }

      // sw.js 由本插件生成。install 不自动 skipWaiting(保留用户「稍后」选择权),
      // 仅收到 SKIP_WAITING 消息才跳过等待(见 Task 3 的 usePwaUpdate)。
      const sw = `/* 构建时由 pwaPlugin 生成,勿手改。PWA 壳离线 Service Worker。 */
const VERSION = '${version}'
const CACHE_NAME = 'ruankao-' + VERSION
const PRECACHE = ${JSON.stringify(precache)}

self.addEventListener('install', (event) => {
  // 只 precache,不自动 skipWaiting:是否接管由页面「立即刷新」决定。
  // 若 install 失败(个别资源 404),本次不激活,下次访问自动重试。
  event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.addAll(PRECACHE)))
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  )
})

self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting()
  }
})

self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') return
  const url = new URL(event.request.url)
  // 跨域(API 在别的域名、外部图床)与同域 /api 一律放行,不拦截不缓存。
  if (url.origin !== location.origin) return
  if (url.pathname.startsWith('/api/')) return
  // 导航请求:network-first,断网回退缓存 index.html(壳离线核心)。
  // 只把「应用壳」写进 index.html 缓存键 —— 构建期预渲染出的静态内容页也是 200 的
  // text/html,若无条件覆盖,用户离线兜底壳会变成某个知识点页面,应用直接坏掉。
  // 认壳靠 index.html 上 <html data-app-shell> 这个标记(预渲染页没有)。
  if (event.request.mode === 'navigate') {
    event.respondWith(
      fetch(event.request)
        .then(async (res) => {
          if (!res.ok) return res
          // 先读文本判断是否壳:res.clone() 保证 res 本体仍能原样返回给页面。
          // 不能先 clone 再 text() 后把那个克隆体 put 进缓存 —— text() 已把它消耗掉,
          // 所以用读到的 body 重建一个 Response 来写缓存。
          const body = await res.clone().text()
          if (body.includes('data-app-shell')) {
            const copy = new Response(body, {
              status: res.status,
              statusText: res.statusText,
              headers: res.headers,
            })
            // waitUntil 包住写入:respondWith 已随 return res 提前兑现,不包的话页面可用后
            // SW 可能被挂起,离线回退用的 index.html 副本会写丢
            event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.put('/index.html', copy)).catch(() => {}))
          }
          return res
        })
        .catch(() => caches.match('/index.html')),
    )
    return
  }
  // precache 命中的 hashed 静态资源:cache-first。
  if (PRECACHE.includes(url.pathname)) {
    event.respondWith(caches.match(event.request).then((hit) => hit || fetch(event.request)))
    return
  }
  // 其余(真题图 images/** 等)放行。
})
`
      mkdirSync('dist', { recursive: true })
      writeFileSync(join('dist', 'manifest.webmanifest'), JSON.stringify(manifest, null, 2) + '\n')
      writeFileSync(join('dist', 'sw.js'), sw)
      console.log(`[pwa] precache ${precache.length} files`)
    },
  }
}

export default defineConfig(({ command, mode }) => {
  const env = loadEnv(mode, process.cwd(), '')

  // 构建期强制校验：生产构建缺 VITE_API_BASE_URL 会在这里直接中止，
  // 不再让它静默兜底成相对路径 /api（理由见 build/apiBaseUrl.ts）。
  const apiBaseUrl = resolveApiBaseUrl(command, env.VITE_API_BASE_URL)

  // Parse API target from apiBaseUrl for dev proxy
  let apiTarget = 'http://localhost:8080'
  if (!apiBaseUrl.startsWith('/')) {
    try {
      const url = new URL(apiBaseUrl)
      apiTarget = url.origin
    } catch {}
  }

  return {
    plugins: [
      vue(),
      Components({
        resolvers: [ElementPlusResolver()],
        dts: 'src/components.d.ts',
      }),
      pwaPlugin(),
    ],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      host: '0.0.0.0',
      port: 3000,
      proxy: {
        '/api': {
          target: apiTarget,
          changeOrigin: true,
        },
      },
    },
    build: {
      // 只把框架层拆出来做长效缓存。jspdf / markmap / tiptap 已经随
      // ExamList、KnowledgeList、QuestionNoteEditor 等懒加载路由天然分包,
      // 再手工列进 manualChunks 反而会被提升成入口的 modulepreload,首屏变重。
      rollupOptions: {
        output: {
          manualChunks: {
            'vendor-vue': ['vue', 'vue-router', 'pinia'],
          },
        },
      },
      chunkSizeWarningLimit: 800,
    },
  }
})
