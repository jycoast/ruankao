<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { SPRINT_POINTS } from '@/data/sprintPoints'
import { SPRINT_MISTAKES } from '@/data/sprintMistakes'

const route = useRoute()
const tab = ref<'points' | 'mistakes'>(route.query.tab === 'mistakes' ? 'mistakes' : 'points')
const keyword = ref('')

onMounted(() => {
  if (route.query.tab === 'mistakes') tab.value = 'mistakes'
})

function filterGroups<T extends { items: string[] }>(groups: T[], titleKey: keyof T) {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return groups
  return groups
    .map((g) => ({
      ...g,
      items: g.items.filter(
        (it) => it.toLowerCase().includes(k) || String(g[titleKey]).toLowerCase().includes(k),
      ),
    }))
    .filter((g) => g.items.length > 0)
}

const points = computed(() => filterGroups(SPRINT_POINTS, 'chapter'))
const mistakes = computed(() => filterGroups(SPRINT_MISTAKES, 'topic'))
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100">必考速记</h2>
      <div class="flex items-center gap-1 rounded-lg border border-slate-200 bg-white p-0.5 dark:border-slate-700 dark:bg-slate-800">
        <button class="rounded-md px-3 py-1 text-sm font-medium transition" :class="tab === 'points' ? 'bg-blue-600 text-white' : 'text-slate-500 dark:text-slate-400'" @click="tab = 'points'">分章要点</button>
        <button class="rounded-md px-3 py-1 text-sm font-medium transition" :class="tab === 'mistakes' ? 'bg-rose-600 text-white' : 'text-slate-500 dark:text-slate-400'" @click="tab = 'mistakes'">高频易错</button>
      </div>
    </div>

    <el-input v-model="keyword" placeholder="搜索关键词" clearable class="mb-4" style="max-width: 320px">
      <template #prefix><el-icon><Search /></el-icon></template>
    </el-input>

    <div class="grid gap-4 md:grid-cols-2">
      <section
        v-for="g in (tab === 'points' ? points : mistakes)"
        :key="(g as any).chapter || (g as any).topic"
        class="rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <h3 class="mb-3 flex items-center gap-2 font-semibold text-slate-900 dark:text-white">
          <span class="inline-block h-4 w-1 rounded-full" :class="tab === 'points' ? 'bg-blue-500' : 'bg-rose-500'" />
          {{ (g as any).chapter || (g as any).topic }}
        </h3>
        <ul class="space-y-2">
          <li v-for="(it, i) in g.items" :key="i" class="flex items-start gap-2 text-sm leading-relaxed text-slate-600 dark:text-slate-300">
            <span class="mt-1 shrink-0" :class="tab === 'points' ? 'text-blue-400' : 'text-rose-400'">•</span>
            <span>{{ it }}</span>
          </li>
        </ul>
      </section>
    </div>

    <div v-if="(tab === 'points' ? points : mistakes).length === 0" class="py-12 text-center text-sm text-slate-500">
      没有匹配的内容
    </div>
  </div>
</template>
