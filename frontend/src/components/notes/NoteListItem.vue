<script setup lang="ts">
import { computed } from 'vue'
import { Notebook } from '@element-plus/icons-vue'
import type { NoteWithQuestion } from '@/types'
import { plainNoteText } from '@/utils/noteText'

const props = defineProps<{ note: NoteWithQuestion; active: boolean }>()

const excerpt = computed(() => plainNoteText(props.note.content))
const title = computed(() => props.note.question_title || `题目 ${props.note.question_id}`)

function typeLabel(t: number) {
  return { 1: '单选', 2: '多选', 3: '案例分析', 4: '论文' }[t] || ''
}
</script>

<template>
  <router-link
    :to="`/my/notes/${note.question_id}`"
    class="block rounded-lg border border-l-[3px] p-3 transition"
    :class="
      active
        ? 'border-blue-200 border-l-blue-500 bg-blue-50/60 dark:border-blue-500/40 dark:border-l-blue-500 dark:bg-blue-500/10'
        : 'border-gray-200 border-l-transparent hover:border-blue-300 hover:border-l-blue-300 dark:border-gray-700 dark:hover:border-blue-500/40'
    "
  >
    <div class="mb-1 flex items-center gap-2">
      <el-tag v-if="typeLabel(note.question_type)" size="small" effect="plain">
        {{ typeLabel(note.question_type) }}
      </el-tag>
      <el-tag v-if="note.paper_title" size="small" type="info" effect="plain" class="min-w-0 truncate">
        {{ note.paper_title }}
      </el-tag>
      <span class="ml-auto whitespace-nowrap text-xs text-gray-500 dark:text-gray-400">
        {{ (note.updated_at || '').replace('T', ' ').slice(0, 16) }}
      </span>
    </div>

    <div class="line-clamp-1 text-sm font-medium text-gray-900 dark:text-gray-100">{{ title }}</div>

    <div class="mt-1 line-clamp-2 text-xs leading-relaxed text-gray-600 dark:text-gray-400">
      <el-icon class="mr-1 align-text-bottom"><Notebook /></el-icon>{{ excerpt || '（空白笔记）' }}
    </div>
  </router-link>
</template>
