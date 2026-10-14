<script setup lang="ts">
import { computed } from 'vue'
import { isDueReview, questionTypeLabel, summarizeStem } from '@/utils/wrongBook'
import type { WrongQuestion } from '@/types'

const props = defineProps<{ wq: WrongQuestion; active: boolean }>()
const emit = defineEmits<{ select: [] }>()

const q = computed<WrongQuestion['question'] | null>(() => props.wq.question ?? null)
const isMultiBlank = computed(() => (props.wq.question?.options?.length || 0) > 4)
const due = computed(() => isDueReview(props.wq, new Date()))
const dueText = computed(() =>
  due.value ? '待复习' : props.wq.next_review_time?.slice(0, 10) || '未安排',
)
</script>

<template>
  <button
    class="flex w-full flex-wrap items-start gap-x-2 gap-y-1.5 rounded-lg border border-l-[3px] px-3 py-2 text-left transition"
    :class="
      active
        ? 'border-blue-200 border-l-blue-500 bg-blue-50/60 dark:border-blue-500/40 dark:border-l-blue-500 dark:bg-blue-500/10'
        : 'border-gray-200 border-l-transparent hover:border-blue-300 hover:border-l-blue-300 dark:border-gray-700 dark:hover:border-blue-500/40'
    "
    @click="emit('select')"
  >
    <el-tag v-if="q" size="small" effect="plain">{{ questionTypeLabel(q.question_type) }}</el-tag>
    <!-- 窄屏（抽屉 86% 宽）下题干 min-w-full 独占一行，右侧标签自动换到第二行，避免题干被挤没 -->
    <span
      class="line-clamp-2 min-w-full flex-1 text-sm leading-snug text-gray-800 dark:text-gray-200 sm:min-w-0"
    >
      {{ summarizeStem(q) || `题目 #${props.wq.question_id}（题目已删除）` }}
    </span>
    <el-tag v-if="isMultiBlank" size="small" type="warning" effect="plain" round class="shrink-0">多空题</el-tag>
    <el-tag
      size="small"
      class="shrink-0"
      :type="props.wq.wrong_count >= 3 ? 'danger' : 'info'"
      :effect="props.wq.wrong_count >= 3 ? 'light' : 'plain'"
    >
      错 {{ props.wq.wrong_count }} 次
    </el-tag>
    <span
      class="shrink-0 text-xs"
      :class="due ? 'text-red-500' : 'text-gray-500 dark:text-gray-400'"
    >{{ dueText }}</span>
  </button>
</template>