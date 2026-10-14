import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'node',
    // 只收集 src 下的单测，外加 build/ 下的构建期工具单测
    // （vite.config.ts 直接 import 的那几个纯函数，见 build/apiBaseUrl.ts）；
    // scripts/prerender/render.test.mjs 是 node:test 套件，
    // 独立于 vitest 运行（node --test），不纳入 vitest。
    include: ['src/**/*.{test,spec}.ts', 'build/**/*.{test,spec}.ts'],
  },
})