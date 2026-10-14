// 前端反调试：提高普通用户用 DevTools 观察请求的门槛。
//
// 重要边界：这只能“劝退”非技术用户，无法真正阻止观察请求——
//   - 抓包工具（Charles / Fiddler / mitmproxy）不经过浏览器控制台
//   - DevTools 里停用断点即可废掉 debugger 陷阱
//   - 爬虫拿到 JWT 后直接 curl，根本不开浏览器
// 真正的防线在服务端限流 / 行为检测（见 backend 反爬中间件）。
//
// 仅在生产构建（import.meta.env.PROD）启用，不影响本地开发调试。

const DEBUGGER_INTERVAL = 1000 // debugger 陷阱触发间隔(ms)
const SIZE_THRESHOLD = 160 // 窗口内外尺寸差阈值，用于粗测 DevTools 是否停靠打开

let started = false

/**
 * 无限 debugger 陷阱：DevTools 打开时命中断点会反复暂停，
 * 关闭时 debugger 是 no-op，不影响正常用户。
 * 用 Function 构造避免被打包工具优化掉。
 */
function debuggerTrap() {
  setInterval(() => {
    // eslint-disable-next-line no-new-func
    ;(function () {}).constructor('debugger')()
  }, DEBUGGER_INTERVAL)
}

/**
 * 尺寸探测：DevTools 停靠（非独立窗口）时，外层窗口与视口尺寸差会变大。
 * 命中后进入“提示遮罩”状态。对独立窗口的 DevTools 无效，仅作补充。
 */
function sizeDetect(onDetected: () => void) {
  const check = () => {
    const widthGap = window.outerWidth - window.innerWidth
    const heightGap = window.outerHeight - window.innerHeight
    if (widthGap > SIZE_THRESHOLD || heightGap > SIZE_THRESHOLD) {
      onDetected()
    }
  }
  setInterval(check, DEBUGGER_INTERVAL)
}

/** 检测到 DevTools 后展示的全屏遮罩，阻断页面浏览。 */
function showBlockOverlay() {
  if (document.getElementById('__anti_debug_mask__')) return
  const mask = document.createElement('div')
  mask.id = '__anti_debug_mask__'
  mask.style.cssText = [
    'position:fixed',
    'inset:0',
    'z-index:2147483647',
    'background:#0b0b0d',
    'color:#e5e7eb',
    'display:flex',
    'flex-direction:column',
    'align-items:center',
    'justify-content:center',
    'gap:16px',
    'font-size:18px',
    'font-family:system-ui,-apple-system,sans-serif',
    'text-align:center',
    'padding:24px',
  ].join(';')
  mask.innerHTML =
    '<div style="font-size:48px">🔒</div>' +
    '<div>检测到开发者工具已开启</div>' +
    '<div style="font-size:14px;color:#9ca3af">为保护题库内容，请关闭开发者工具后刷新页面</div>'
  document.body.appendChild(mask)
}

/** 屏蔽常见 DevTools 入口快捷键，作为轻量补充（可被绕过，仅提高门槛）。
 * 注意：不再禁用右键菜单（contextmenu）。右键屏蔽误伤普通用户最多
 * （无法复制文字、无法用「在新标签打开图片」等），而防爬收益几乎为零
 * ——爬虫不会靠右键扒站。真正拦在 debugger 陷阱 + 尺寸探测遮罩。
 */
function blockShortcuts() {
  document.addEventListener('keydown', (e) => {
    const key = e.key.toUpperCase()
    // F12
    if (e.key === 'F12') e.preventDefault()
    // Ctrl+Shift+I / J / C（DevTools、Console、检查元素）
    if ((e.ctrlKey || e.metaKey) && e.shiftKey && ['I', 'J', 'C'].includes(key)) {
      e.preventDefault()
    }
    // Ctrl+U（查看源码）
    if ((e.ctrlKey || e.metaKey) && key === 'U') e.preventDefault()
  })
}

/**
 * 启用前端反调试。仅生产构建生效，重复调用无副作用。
 */
export function enableAntiDebug() {
  if (started || !import.meta.env.PROD) return
  started = true

  blockShortcuts()
  debuggerTrap()
  sizeDetect(showBlockOverlay)
}
