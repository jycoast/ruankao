// 一次性脚本:从 public/brand-mark.png 生成 PWA 图标 PNG。
// sharp 仅临时安装,不写入 package.json:npm install --no-save sharp && node scripts/gen-pwa-icons.mjs
// 来源从 brand-mark.svg 换成 brand-mark.png(白底「软考星图」字标, 400x400)。
import sharp from 'sharp'
import { readFileSync } from 'node:fs'

const src = readFileSync('public/brand-mark.png')

// 常规图标(192 清单 / 512 清单 / 180 apple-touch):全幅白底字标直接栅格化。
for (const size of [192, 512, 180]) {
  await sharp(src).resize(size, size).png().toFile(`public/pwa-icon-${size}.png`)
}

// maskable:字标是宽幅(印墨约 90%x70% 的 400 画布),缩到 70% 居中才保证全部
// 印墨落在系统安全圈(半径 = 512*40% = 205px)内,80% 会切到红色底杠两端。
// 铺白底(与字标自身底色、manifest background_color 一致)。
const inner = Math.round(512 * 0.7)
const logo = await sharp(src).resize(inner, inner).png().toBuffer()
await sharp({
  create: { width: 512, height: 512, channels: 4, background: { r: 255, g: 255, b: 255, alpha: 1 } },
})
  .composite([{ input: logo, gravity: 'centre' }])
  .png()
  .toFile('public/pwa-icon-512-maskable.png')

console.log('generated pwa icons (192 / 512 / 512-maskable / 180)')