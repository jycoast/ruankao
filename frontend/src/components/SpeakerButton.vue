<script setup lang="ts">
// 喇叭按钮：点击朗读传入的文本。不支持 Web Speech 的浏览器不渲染。
// 用内联 SVG 而不是 @element-plus/icons-vue —— 图标库里没有喇叭形状。
import { useSpeech } from '@/composables/useSpeech'

defineProps<{ text: string }>()

const { isSupported, speak } = useSpeech()
</script>

<template>
  <button
    v-if="isSupported"
    type="button"
    class="inline-flex shrink-0 items-center justify-center rounded-full p-2 text-slate-400 transition hover:bg-slate-100 hover:text-blue-600 dark:hover:bg-slate-800 dark:hover:text-blue-400"
    :aria-label="`播放 ${text} 的读音`"
    :title="`播放 ${text} 的读音`"
    @click.stop="speak(text)"
    @keydown.enter.stop
    @keydown.space.stop
  >
    <svg
      class="h-4 w-4"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path d="M11 5 6 9H2v6h4l5 4V5z" />
      <path d="M15.54 8.46a5 5 0 0 1 0 7.07" />
      <path d="M19.07 4.93a10 10 0 0 1 0 14.14" />
    </svg>
  </button>
</template>
