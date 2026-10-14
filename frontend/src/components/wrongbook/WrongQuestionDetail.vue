<script setup lang="ts">
import { computed } from 'vue'
import { isDueReview, questionTypeLabel } from '@/utils/wrongBook'
import { renderMd, renderMdInline } from '@/utils/markdown'
import { renderWrongStem, stemCoversTitle } from '@/utils/blanks'
import type { WrongQuestion } from '@/types'

const props = defineProps<{ wq: WrongQuestion }>()
const emit = defineEmits<{ remove: [] }>()

const q = computed<WrongQuestion['question'] | null>(() => props.wq.question ?? null)
const isMultiBlank = computed(() => (props.wq.question?.options?.length || 0) > 4)
const due = computed(() => isDueReview(props.wq, new Date()))
const dueText = computed(() =>
  due.value ? '待复习' : props.wq.next_review_time?.slice(0, 10) || '未安排',
)
</script>

<template>
  <div class="flex h-full flex-col p-4 sm:p-6">
    <template v-if="q">
      <div class="mb-4 flex flex-wrap items-center gap-2">
        <el-tag size="small" effect="plain">{{ questionTypeLabel(q.question_type) }}</el-tag>
        <el-tag v-if="isMultiBlank" size="small" type="warning" effect="plain" round>多空题</el-tag>
        <el-tag
          size="small"
          :type="props.wq.wrong_count >= 3 ? 'danger' : 'info'"
          :effect="props.wq.wrong_count >= 3 ? 'light' : 'plain'"
        >错 {{ props.wq.wrong_count }} 次</el-tag>
        <span class="text-xs" :class="due ? 'text-red-500' : 'text-gray-500 dark:text-gray-400'">{{ dueText }}</span>
      </div>

      <div v-if="!stemCoversTitle(q.title, q.content)" class="mb-2 whitespace-pre-wrap font-medium text-gray-900 dark:text-gray-100">{{ q.title }}</div>
      <div v-if="q.content" class="mb-3 md-content leading-relaxed text-gray-700 dark:text-gray-300" v-html="renderWrongStem(q)"></div>

      <div v-if="q.options?.length" class="mb-3 space-y-1.5">
        <div
          v-for="opt in q.options"
          :key="opt.id"
          class="flex gap-2 rounded-lg border p-2 text-sm"
          :class="opt.is_correct === 1
            ? 'border-green-500 bg-green-50 text-green-700 dark:border-green-600 dark:bg-green-900/30 dark:text-green-300'
            : 'border-gray-200 text-gray-700 dark:border-gray-600 dark:text-gray-300'"
        >
          <span class="font-medium shrink-0">{{ opt.label }}.</span>
          <span v-html="renderMdInline(opt.content)"></span>
        </div>
      </div>

      <template v-if="q.explanation || q.analysis">
        <div v-if="q.explanation" class="mb-3 rounded bg-gray-50 p-3 text-gray-600 dark:bg-gray-700/50 dark:text-gray-400">
          <div class="font-medium">解析：</div>
          <div class="md-content" v-html="renderMd(q.explanation)"></div>
        </div>
        <div v-if="q.analysis" class="mb-3 rounded bg-gray-50 p-3 text-gray-600 dark:bg-gray-700/50 dark:text-gray-400">
          <div class="font-medium">分析：</div>
          <div class="md-content" v-html="renderMd(q.analysis)"></div>
        </div>
      </template>

      <div class="mt-auto flex items-center gap-3 border-t border-gray-100 pt-4 dark:border-gray-700">
        <router-link
          :to="`/questions/${props.wq.question_id}`"
          class="text-sm text-blue-600 hover:underline dark:text-blue-400"
        >查看题目详情 →</router-link>
        <span class="flex-1"></span>
        <el-button size="small" type="danger" plain @click="emit('remove')">移出错题本</el-button>
      </div>
    </template>
    <div v-else class="flex items-center justify-between text-gray-400 dark:text-gray-500">
      <span>题目 #{{ props.wq.question_id }}（题目已删除或无法加载）</span>
      <el-button size="small" type="danger" plain @click="emit('remove')">移出</el-button>
    </div>
  </div>
</template>