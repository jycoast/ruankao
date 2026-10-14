<script setup lang="ts">
import { computed, ref } from 'vue'

interface Props {
  total: number
  currentIndex: number
  answeredMap: Record<number, boolean>
  // 判题结果（练习模式/已交卷）：true=正确 false=错误 null=已答未判(主观题)。
  // 不传则为普通作答模式，仅区分已答/未答。
  resultMap?: Record<number, boolean | null>
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'jump', index: number): void
}>()

const collapsed = ref(false)
const hasResult = computed(() => !!props.resultMap)

function getBlockClass(index: number) {
  const base = 'w-8 h-8 rounded-md text-xs font-medium cursor-pointer transition-all flex items-center justify-center shrink-0'
  if (index === props.currentIndex) {
    return `${base} bg-blue-600 text-white shadow-md ring-2 ring-blue-300`
  }
  // 判题结果模式：正确=绿，错误=红，已答未判=琥珀，未作答=灰
  if (props.resultMap) {
    if (index in props.resultMap) {
      const r = props.resultMap[index]
      if (r === true) {
        return `${base} bg-green-100 text-green-700 hover:bg-green-200 dark:bg-green-900/40 dark:text-green-400`
      }
      if (r === false) {
        return `${base} bg-red-100 text-red-700 hover:bg-red-200 dark:bg-red-900/40 dark:text-red-400`
      }
      return `${base} bg-amber-100 text-amber-700 hover:bg-amber-200 dark:bg-amber-900/40 dark:text-amber-400`
    }
    return `${base} bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600`
  }
  // 普通作答模式：已答=绿，未答=灰
  if (props.answeredMap[index]) {
    return `${base} bg-green-100 text-green-700 hover:bg-green-200 dark:bg-green-900/40 dark:text-green-400`
  }
  return `${base} bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600`
}
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700 overflow-hidden">
    <!-- Toggle Header -->
    <button 
      @click="collapsed = !collapsed"
      class="w-full flex items-center justify-between px-4 py-3 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
    >
      <div class="flex items-center gap-2">
        <svg class="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h16"/>
        </svg>
        <span class="font-medium text-gray-700 dark:text-gray-300">答题卡</span>
        <span class="text-sm text-gray-400">({{ Object.keys(answeredMap).length }}/{{ total }})</span>
      </div>
      <svg 
        class="w-4 h-4 text-gray-400 transition-transform" 
        :class="{ 'rotate-180': !collapsed }"
        fill="none" stroke="currentColor" viewBox="0 0 24 24"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
      </svg>
    </button>

    <!-- Answer Grid -->
    <div v-show="!collapsed" class="px-4 pb-4">
      <div class="grid gap-2" style="grid-template-columns: repeat(auto-fill, minmax(2rem, 1fr));">
        <button
          v-for="(_, i) in total"
          :key="i"
          :class="getBlockClass(i)"
          @click="emit('jump', i)"
        >
          {{ i + 1 }}
        </button>
      </div>
      
      <!-- Legend -->
      <div v-if="hasResult" class="flex flex-wrap items-center gap-x-4 gap-y-1.5 mt-3 text-xs text-gray-500 dark:text-gray-400">
        <span class="flex items-center gap-1.5">
          <span class="w-3 h-3 rounded bg-green-100 dark:bg-green-900/40 border border-green-300 dark:border-green-700"></span>
          正确
        </span>
        <span class="flex items-center gap-1.5">
          <span class="w-3 h-3 rounded bg-red-100 dark:bg-red-900/40 border border-red-300 dark:border-red-700"></span>
          错误
        </span>
        <span class="flex items-center gap-1.5">
          <span class="w-3 h-3 rounded bg-amber-100 dark:bg-amber-900/40 border border-amber-300 dark:border-amber-700"></span>
          待判
        </span>
        <span class="flex items-center gap-1.5">
          <span class="w-3 h-3 rounded bg-gray-100 dark:bg-gray-700 border border-gray-300 dark:border-gray-600"></span>
          未答
        </span>
        <span class="flex items-center gap-1.5">
          <span class="w-3 h-3 rounded bg-blue-600"></span>
          当前
        </span>
      </div>
      <div v-else class="flex items-center gap-4 mt-3 text-xs text-gray-500 dark:text-gray-400">
        <span class="flex items-center gap-1.5">
          <span class="w-3 h-3 rounded bg-gray-100 dark:bg-gray-700 border border-gray-300 dark:border-gray-600"></span>
          未答
        </span>
        <span class="flex items-center gap-1.5">
          <span class="w-3 h-3 rounded bg-green-100 dark:bg-green-900/40 border border-green-300 dark:border-green-700"></span>
          已答
        </span>
        <span class="flex items-center gap-1.5">
          <span class="w-3 h-3 rounded bg-blue-600"></span>
          当前
        </span>
      </div>
    </div>
  </div>
</template>
