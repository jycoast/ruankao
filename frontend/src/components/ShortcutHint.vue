<script setup lang="ts">
withDefaults(defineProps<{ enterLabel?: string; showEnter?: boolean }>(), {
  enterLabel: '提交 / 下一题',
  showEnter: true,
})
</script>

<template>
  <div class="mt-3 hidden items-center gap-3 text-xs text-gray-400 sm:flex dark:text-gray-500">
    <span class="flex items-center gap-1">
      <kbd class="rounded border border-gray-200 px-1.5 py-0.5 font-mono dark:border-gray-600">A</kbd>
      <span>~</span>
      <kbd class="rounded border border-gray-200 px-1.5 py-0.5 font-mono dark:border-gray-600">D</kbd>
      选项
    </span>
    <span v-if="showEnter" class="flex items-center gap-1">
      <kbd class="rounded border border-gray-200 px-1.5 py-0.5 font-mono dark:border-gray-600">Enter</kbd>
      {{ enterLabel }}
    </span>
    <span class="flex items-center gap-1">
      <kbd class="rounded border border-gray-200 px-1.5 py-0.5 font-mono dark:border-gray-600">←</kbd>
      <kbd class="rounded border border-gray-200 px-1.5 py-0.5 font-mono dark:border-gray-600">→</kbd>
      翻题
    </span>
  </div>
</template>
