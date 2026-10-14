import { ref, watch } from 'vue'

export type ThemeColor =
  | 'blue' | 'indigo' | 'purple' | 'fuchsia' | 'pink' | 'red'
  | 'orange' | 'amber' | 'lime' | 'green' | 'teal' | 'cyan'
export type ThemeMode = 'light' | 'dark' | 'system'

// 主题色单一事实源。value 存 localStorage；hex 一律取 Tailwind 500 档，与 applyTwBlue /
// applyElPrimary 的派生基准保持一致（换非 Tailwind 色号会让派生出的色阶跑偏）。
// 数组顺序 = 个人中心色块的排列顺序：按色相走一圈，同色系相邻，默认的蓝色排头。
// 加新颜色只需要在这里加一行 —— 个人中心自动多一个色块。
export const THEME_COLOR_OPTIONS: { value: ThemeColor; label: string; color: string }[] = [
  { value: 'blue', label: '蓝色', color: '#3b82f6' },
  { value: 'indigo', label: '靛蓝', color: '#6366f1' },
  { value: 'purple', label: '紫色', color: '#8b5cf6' },
  { value: 'fuchsia', label: '紫红', color: '#d946ef' },
  { value: 'pink', label: '粉色', color: '#ec4899' },
  { value: 'red', label: '红色', color: '#ef4444' },
  { value: 'orange', label: '橙色', color: '#f97316' },
  { value: 'amber', label: '琥珀', color: '#f59e0b' },
  { value: 'lime', label: '青柠', color: '#84cc16' },
  { value: 'green', label: '绿色', color: '#22c55e' },
  { value: 'teal', label: '青色', color: '#14b8a6' },
  { value: 'cyan', label: '青蓝', color: '#06b6d4' },
]

const THEME_COLORS = Object.fromEntries(
  THEME_COLOR_OPTIONS.map((o) => [o.value, o.color]),
) as Record<ThemeColor, string>

const themeColor = ref<ThemeColor>((localStorage.getItem('theme_color') as ThemeColor) || 'blue')
const themeMode = ref<ThemeMode>((localStorage.getItem('theme_mode') as ThemeMode) || 'light')

// 把主色派生出 Tailwind 的 blue-* 色阶（tailwind.config.js 已把 blue-50..900 接到这些变量）。
// 50..400 向白渐变、500 是主色、600..900 向黑渐变。切换主题色后所有 blue-* 类随之变化。
// ---- 颜色工具：把主题色按 HSL 派生出各档，输出 "r g b" 通道字符串 ----
// 用通道而非整色，是为了配合 tailwind.config.js 里 rgb(var(--tw-blue-500-rgb) / <alpha-value>)，
// 这样 blue-500/15 等透明度修饰符变体也能正常生成并跟随主题色。
function hexToRgb(hex: string) {
  const h = hex.replace('#', '')
  const n = h.length === 3 ? h.split('').map((c) => c + c).join('') : h
  return { r: parseInt(n.slice(0, 2), 16), g: parseInt(n.slice(2, 4), 16), b: parseInt(n.slice(4, 6), 16) }
}
function rgbToHsl({ r, g, b }: { r: number; g: number; b: number }) {
  r /= 255; g /= 255; b /= 255
  const max = Math.max(r, g, b), min = Math.min(r, g, b)
  let h = 0, s = 0
  const l = (max + min) / 2
  if (max !== min) {
    const d = max - min
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min)
    switch (max) {
      case r: h = (g - b) / d + (g < b ? 6 : 0); break
      case g: h = (b - r) / d + 2; break
      default: h = (r - g) / d + 4
    }
    h /= 6
  }
  return { h: h * 360, s: s * 100, l: l * 100 }
}
function hslToRgb(h: number, s: number, l: number) {
  h /= 360; s /= 100; l /= 100
  if (s === 0) { const v = Math.round(l * 255); return { r: v, g: v, b: v } }
  const hue2rgb = (p: number, q: number, t: number) => {
    if (t < 0) t += 1
    if (t > 1) t -= 1
    if (t < 1 / 6) return p + (q - p) * 6 * t
    if (t < 1 / 2) return q
    if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6
    return p
  }
  const q = l < 0.5 ? l * (1 + s) : l + s - l * s
  const p = 2 * l - q
  return {
    r: Math.round(hue2rgb(p, q, h + 1 / 3) * 255),
    g: Math.round(hue2rgb(p, q, h) * 255),
    b: Math.round(hue2rgb(p, q, h - 1 / 3) * 255),
  }
}
// 各档目标 lightness（参考 Tailwind 默认蓝色阶），保持主题色的 hue 与 saturation。
// 各档目标 lightness（参考 Tailwind 默认蓝色阶），500 档直接用主色本身，保持默认主题与原生蓝一致。
const BLUE_LEVELS: Record<number, number> = {
  50: 96, 100: 93, 200: 86, 300: 76, 400: 62, 600: 42, 700: 34, 800: 27, 900: 21,
}
function applyTwBlue(root: HTMLElement, color: string) {
  const base = hexToRgb(color)
  const { h, s } = rgbToHsl(base)
  const clampS = s === 0 ? 0 : Math.max(s, 55) // 浅档避免发灰，深档保留主题饱和度
  root.style.setProperty('--tw-blue-500-rgb', `${base.r} ${base.g} ${base.b}`)
  for (const [n, l] of Object.entries(BLUE_LEVELS)) {
    const c = hslToRgb(h, clampS, l)
    root.style.setProperty(`--tw-blue-${n}-rgb`, `${c.r} ${c.g} ${c.b}`)
  }
}
// 把主色派生出 Element Plus 需要的色阶，让 EP 组件也跟随主题色变化。
// EP 的 primary 色体系是 1 个主色 + light-1..9（向白渐变）+ dark-2..10（向黑渐变），
// 这里只需常用档位。用 color-mix 现算，省掉手写 6 套色阶。
function applyElPrimary(root: HTMLElement, color: string) {
  root.style.setProperty('--el-color-primary', color)
  root.style.setProperty('--el-color-primary-light-3', `color-mix(in srgb, ${color} 70%, white)`)
  root.style.setProperty('--el-color-primary-light-5', `color-mix(in srgb, ${color} 50%, white)`)
  root.style.setProperty('--el-color-primary-light-7', `color-mix(in srgb, ${color} 30%, white)`)
  root.style.setProperty('--el-color-primary-light-8', `color-mix(in srgb, ${color} 20%, white)`)
  root.style.setProperty('--el-color-primary-light-9', `color-mix(in srgb, ${color} 10%, white)`)
  root.style.setProperty('--el-color-primary-dark-2', `color-mix(in srgb, ${color} 80%, black)`)
}

function applyTheme() {
  const root = document.documentElement
  const color = THEME_COLORS[themeColor.value]

  root.style.setProperty('--primary', color)
  applyElPrimary(root, color)
  applyTwBlue(root, color)

  const isDark =
    themeMode.value === 'dark' ||
    (themeMode.value === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)

  root.classList.toggle('dark', isDark)
}

watch(themeColor, (v) => { localStorage.setItem('theme_color', v); applyTheme() })
watch(themeMode, (v) => { localStorage.setItem('theme_mode', v); applyTheme() })

export function useAppearance() {
  return {
    themeColor,
    themeMode,
    colorOptions: THEME_COLOR_OPTIONS,
    setThemeColor(c: ThemeColor) { themeColor.value = c },
    setThemeMode(m: ThemeMode) { themeMode.value = m },
    init() { applyTheme() },
  }
}
