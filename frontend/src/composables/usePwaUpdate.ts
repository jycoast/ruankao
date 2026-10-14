// PWA 运行时:注册 Service Worker + SW 驱动的更新检测。
// 替代旧 versionCheck.ts 的 version.json 轮询:新版本由 SW 的 updatefound 事件感知,
// 弹非阻断通知(稍后/立即刷新),点「立即刷新」postMessage(SKIP_WAITING) → SW 跳过等待
// → controllerchange → 整页重载。首次安装静默,不弹不自动重载。
import { ElNotification, ElButton } from 'element-plus'
import { h } from 'vue'

// 与旧 versionCheck 一致:太频繁打扰用户,太稀疏提示滞后。
const CHECK_INTERVAL = 5 * 60 * 1000

let timer: number | null = null
let disposed = false
// 仅当用户点过「立即刷新」才在 controllerchange 时重载,避免首次安装 claim 触发意外重载。
let reloadPending = false

// 有新版本:弹非阻断通知。文案/交互与旧 versionCheck 保持一致,只是触发源换成 SW 更新。
// 首次安装(navigator.serviceWorker.controller 为 null)不弹。
function notifyUpdate(registration: ServiceWorkerRegistration): void {
  if (!navigator.serviceWorker.controller) return
  let closeNotify: () => void = () => {}
  const notify = ElNotification({
    title: '发现新版本',
    message: () =>
      h('div', { style: 'display:flex;flex-direction:column;gap:10px;' }, [
        h('span', '新版本已就绪，点击「立即刷新」即可使用。'),
        h(
          'div',
          { style: 'display:flex;justify-content:flex-end;gap:8px;' },
          [
            h(ElButton, { onClick: () => closeNotify() }, () => '稍后'),
            h(
              ElButton,
              { type: 'primary', onClick: () => void applyUpdate(registration) },
              () => '立即刷新',
            ),
          ],
        ),
      ]),
    type: 'info',
    duration: 10000,
  })
  closeNotify = () => notify.close()
}

// 通知等待中的新 SW 接管。controllerchange 触发后由 startPwaUpdate 里的监听完成 reload。
function applyUpdate(registration: ServiceWorkerRegistration): void {
  reloadPending = true
  // 另一标签页可能已让 waiting 接管（此时 waiting 为空），postMessage 无人接收、点了没反应，
  // 兜底直接整页重载拉取新版本。
  if (registration.waiting) {
    registration.waiting.postMessage({ type: 'SKIP_WAITING' })
  } else {
    window.location.reload()
  }
}

export function startPwaUpdate(): void {
  // 开发模式不注册 SW(无产物可服务);无 SW 环境(隐私模式/老浏览器)静默降级。
  if (import.meta.env.DEV) return
  if (!('serviceWorker' in navigator)) return
  disposed = false
  reloadPending = false
  navigator.serviceWorker
    .register(import.meta.env.BASE_URL + 'sw.js')
    .then((registration) => {
      if (disposed) return
      // 用户此前点过「稍后」（或通知超时关闭）：waiting 版本会一直留在 registration 上，
      // 之后 update() 拉到的 sw.js 与 waiting 版字节相同，不会再触发 updatefound，
      // 该版本的更新提示就永远丢了 —— 注册完成后要主动补一次提示。
      // 首次安装时 controller 为空，notifyUpdate 内部会拦住，不会误弹。
      if (registration.waiting) notifyUpdate(registration)
      // 新 SW 安装完成 → 有新版本待接管
      registration.addEventListener('updatefound', () => {
        const newWorker = registration.installing
        if (!newWorker) return
        newWorker.addEventListener('statechange', () => {
          if (newWorker.state === 'installed') notifyUpdate(registration)
        })
      })
      // 新 SW 接管页面 → 仅用户主动刷新时整页重载
      navigator.serviceWorker.addEventListener('controllerchange', () => {
        if (reloadPending) {
          reloadPending = false
          window.location.reload()
        }
      })
      // 挂载后立即查一次,之后每 5 分钟(替代旧 version.json 轮询节奏)
      const tick = (): void => {
        void registration.update()
      }
      tick()
      timer = window.setInterval(tick, CHECK_INTERVAL)
    })
    // sw.js 拉取失败（部署异常/被拦截）静默降级：只少离线能力与更新提示，不影响正常使用
    .catch(() => {})
}

export function stopPwaUpdate(): void {
  disposed = true
  if (timer !== null) {
    window.clearInterval(timer)
    timer = null
  }
}
