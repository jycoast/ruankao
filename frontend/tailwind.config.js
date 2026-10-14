/** @type {import('tailwindcss').Config} */
// blue 色阶接入 CSS 变量：所有 blue-* 类跟随主题色变化（含透明度修饰符 blue-500/15 等）。
// 用 RGB 通道形式 rgb(var(--tw-blue-500-rgb) / <alpha-value>)，这样 Tailwind 才能生成
// 透明度变体（rgb(...) / 0.15）。变量由 stores/appearance.ts 按主题色计算填值。
// 语义色 red/green/orange 等不接入，避免错误/成功/警告色被主题色覆盖。
function blueScale() {
  const out = {}
  for (const n of [50, 100, 200, 300, 400, 500, 600, 700, 800, 900]) {
    out[n] = `rgb(var(--tw-blue-${n}-rgb) / <alpha-value>)`
  }
  return out
}

export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        blue: blueScale(),
      },
    },
  },
  plugins: [],
}