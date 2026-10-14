<script setup lang="ts">
import { isRecentPaperYear } from '@/utils/memberGate'
import type { Paper } from '@/types'

defineProps<{ paper: Paper }>()
const emit = defineEmits<{ (e: 'open', id: number): void }>()

function typeLabel(t: number) {
  return { 1: '综合知识', 2: '案例分析', 3: '论文' }[t] || '未知'
}

function typeColor(t: number) {
  return { 1: 'bg-blue-100 text-blue-700', 2: 'bg-green-100 text-green-700', 3: 'bg-purple-100 text-purple-700' }[t] || 'bg-gray-100 text-gray-700'
}
</script>

<template>
  <div
    class="group bg-white dark:bg-gray-800 rounded-xl p-5 shadow-sm border border-gray-100 dark:border-gray-700 hover:shadow-lg hover:border-blue-200 dark:hover:border-blue-700 transition-all cursor-pointer"
    @click="emit('open', paper.id)"
  >
    <!-- Type Badge -->
    <div class="flex items-center justify-between mb-3">
      <div class="flex items-center gap-1.5">
        <span
          class="px-2.5 py-1 rounded-md text-xs font-medium"
          :class="typeColor(paper.paper_type)"
        >
          {{ typeLabel(paper.paper_type) }}
        </span>
      </div>
      <!-- 近三年卷为会员专属：低调描边小标放在年份旁，不抢年份焦点 -->
      <div class="flex items-center gap-1.5">
        <span
          v-if="isRecentPaperYear(paper.year)"
          class="px-1.5 py-0.5 rounded border border-amber-300 dark:border-amber-700 text-xs text-amber-600 dark:text-amber-400"
        >会员专属</span>
        <span class="text-xs text-gray-400">{{ paper.year }}年</span>
      </div>
    </div>

    <!-- Title -->
    <h3 class="font-semibold text-gray-800 dark:text-gray-200 mb-3 line-clamp-2 group-hover:text-blue-600 dark:group-hover:text-blue-400 transition-colors">
      {{ paper.title }}
    </h3>

    <!-- Meta Info -->
    <div class="flex flex-wrap gap-3 text-xs text-gray-500 dark:text-gray-400">
      <span class="flex items-center gap-1">
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
        </svg>
        {{ paper.duration }}分钟
      </span>
      <span class="flex items-center gap-1">
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
        </svg>
        {{ paper.total_score }}分
      </span>
    </div>

    <!-- Action -->
    <div class="mt-4 pt-3 border-t border-gray-100 dark:border-gray-700">
      <span class="text-sm font-medium text-blue-600 dark:text-blue-400 group-hover:translate-x-1 inline-flex items-center transition-transform">
        开始考试
        <svg class="w-4 h-4 ml-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
        </svg>
      </span>
    </div>
  </div>
</template>
