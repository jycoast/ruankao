<script setup lang="ts">
import { computed } from 'vue'
import { useSubjectStore } from '@/stores/subject'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ (e: 'update:visible', v: boolean): void }>()

const subjectStore = useSubjectStore()

const open = computed({
  get: () => props.visible,
  set: (v: boolean) => emit('update:visible', v),
})

function pick(id: number) {
  subjectStore.setSubject(id)
  open.value = false
}
</script>

<template>
  <el-dialog
    v-model="open"
    title="选择考试科目"
    width="calc(100vw - 32px)"
    :style="{ maxWidth: '760px' }"
    append-to-body
  >
    <div v-if="!subjectStore.subjects.length" v-loading="true" class="h-40"></div>
    <div v-else class="max-h-[65vh] space-y-6 overflow-y-auto">
      <div v-for="group in subjectStore.subjectsByLevel" :key="group.level">
        <div class="mb-3 text-xs font-semibold tracking-wide text-slate-400">{{ group.name }}</div>
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
          <button
            v-for="s in group.items"
            :key="s.id"
            class="flex items-center justify-between gap-2 rounded-lg border px-4 py-2.5 text-left text-sm transition"
            :class="
              s.id === subjectStore.currentSubjectId
                ? 'border-blue-300 bg-blue-50 text-blue-700 dark:border-blue-500/40 dark:bg-blue-500/10 dark:text-blue-300'
                : 'border-slate-200 text-slate-700 hover:border-blue-200 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800'
            "
            @click="pick(s.id)"
          >
            <span class="truncate font-medium">{{ s.name }}</span>
            <el-icon v-if="s.id === subjectStore.currentSubjectId" class="shrink-0 text-blue-600 dark:text-blue-400"><Check /></el-icon>
          </button>
        </div>
      </div>
    </div>
  </el-dialog>
</template>