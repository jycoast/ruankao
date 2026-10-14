<script setup lang="ts">
// 登录/注册页共用外壳（Minimalism & Swiss 方向）：
// 浅灰底（#F8FAFC）+ 居中纯白单卡 + 品牌头 + 倒计时脚注。
// 无渐变、无插图、无面板装饰——蓝色只出现在按钮/链接/焦点环（由全站主题色提供）。
// 浅色/暗色双态，单一淡入动效，尊重 prefers-reduced-motion。
import { useExamCountdown } from '@/composables/useExamCountdown'

const { countdown } = useExamCountdown()
</script>

<template>
  <div class="auth-shell flex min-h-screen flex-col items-center justify-center bg-[#f8fafc] px-4 py-10 dark:bg-slate-950">
    <!-- 品牌头：与下方卡片同宽（max-w-sm），左右边缘对齐卡片 -->
    <header class="auth-item flex w-full max-w-sm items-center justify-between gap-2.5">
      <div class="flex items-center gap-2.5">
        <img src="/brand-mark.png" alt="软考星图" class="h-8 w-8" />
        <div class="text-lg font-bold tracking-tight text-slate-950 dark:text-white">软考星图</div>
      </div>
      <span class="text-sm text-slate-500 dark:text-slate-400">软考真题 · 备考练习</span>
    </header>

    <!-- 表单卡（由页面提供，.auth-card） -->
    <main class="mt-7 w-full max-w-sm">
      <slot />
    </main>

    <!-- 倒计时脚注：安静的一行小字 -->
    <p
      v-if="countdown.days > 0"
      class="auth-item mt-6 text-sm text-slate-500 dark:text-slate-400"
      style="animation-delay: 120ms"
    >
      距 {{ countdown.label }}考试还有 {{ countdown.days }} 天
    </p>
  </div>
</template>

<style scoped>
@keyframes auth-fade-in-up {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}
.auth-item { animation: auth-fade-in-up 0.4s ease both; }
main { animation: auth-fade-in-up 0.4s ease 0.08s both; }

@media (prefers-reduced-motion: reduce) {
  .auth-item,
  main { animation: none; }
}
</style>
