<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import WrongQuestionListItem from './WrongQuestionListItem.vue'
import type { WrongQuestion } from '@/types'

// 左栏扁平列表：不再按知识点分组，直接展示题干摘要。
// 父组件传入已过滤并排序（最新答错在前）的题目与选中态，本组件只负责渲染与转发事件。
// 宽屏常驻左栏与窄屏抽屉里复用同一个组件（同笔记页 NoteList）。
// 分页：列表自身是滚动容器，底部常驻一个哨兵，IntersectionObserver 看到它就
// emit loadMore 让父级拉下一页（同 NoteList 的无限滚动模式）。
const props = defineProps<{
  items: WrongQuestion[]
  selectedId: number | null
  hasMore: boolean
  loadingMore?: boolean
  loadFailed?: boolean
}>()
const emit = defineEmits<{ select: [id: number]; loadMore: []; retry: [] }>()

const scrollEl = ref<HTMLElement | null>(null)
const sentinel = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

onMounted(() => {
  // root 绑列表自己的滚动容器，不绑 viewport —— 抽屉里复用同一组件时，
  // ref 指向的仍是该实例自己的容器，两种布局都对。
  observer = new IntersectionObserver(
    (entries) => {
      if (entries[0]?.isIntersecting) emit('loadMore')
    },
    { root: scrollEl.value, rootMargin: '200px' },
  )
  // 哨兵常驻，不随 hasMore 卸载：hasMore 变化时要重新 observe 多一套竞态。
  if (sentinel.value) observer.observe(sentinel.value)
})

onUnmounted(() => observer?.disconnect())
</script>

<template>
  <div ref="scrollEl" class="min-h-0 flex-1 space-y-1 overflow-y-auto p-1">
    <WrongQuestionListItem
      v-for="wq in items"
      :key="wq.id"
      :wq="wq"
      :active="selectedId === wq.id"
      @select="emit('select', wq.id)"
    />

    <!-- 分页反馈区 -->
    <div v-if="loadingMore" class="space-y-1 px-0.5 py-1">
      <el-skeleton v-for="i in 2" :key="`sk-${i}`" :rows="1" animated class="rounded-lg border border-gray-200 p-2 dark:border-gray-700" />
    </div>
    <!-- 翻页失败：列表已有内容，只在底部挂一条可重试的提示，不整块替换 -->
    <div v-else-if="loadFailed && items.length" class="py-2 text-center text-xs text-gray-500 dark:text-gray-400">
      加载失败
      <el-button link type="primary" size="small" @click="emit('retry')">重试</el-button>
    </div>
    <div v-else-if="!hasMore && items.length" class="py-2 text-center text-xs text-gray-400 dark:text-gray-500">
      没有更多了
    </div>

    <!-- 无限滚动哨兵：常驻，不要加 v-if -->
    <div ref="sentinel" class="h-px w-full"></div>
  </div>
</template>