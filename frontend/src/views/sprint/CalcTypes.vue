<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { SPRINT_CALC } from '@/data/sprintCalc'

const route = useRoute()
const tab = ref<'calc' | 'formula'>(route.query.tab === 'formula' ? 'formula' : 'calc')
const keyword = ref('')

const filtered = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return SPRINT_CALC
  return SPRINT_CALC.filter(
    (c) => c.name.toLowerCase().includes(k) || c.category.toLowerCase().includes(k),
  )
})

// 公式速查：按分类聚合
const formulaGroups = computed(() => {
  const map = new Map<string, { name: string; formulas: string[] }[]>()
  for (const c of SPRINT_CALC) {
    if (!map.has(c.category)) map.set(c.category, [])
    map.get(c.category)!.push({ name: c.name, formulas: c.formulas })
  }
  return [...map.entries()].map(([category, items]) => ({ category, items }))
})

onMounted(() => {
  if (route.query.tab === 'formula') tab.value = 'formula'
})
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100">计算题冲刺</h2>
      <div class="flex items-center gap-1 rounded-lg border border-slate-200 bg-white p-0.5 dark:border-slate-700 dark:bg-slate-800">
        <button
          class="rounded-md px-3 py-1 text-sm font-medium transition"
          :class="tab === 'calc' ? 'bg-blue-600 text-white' : 'text-slate-500 dark:text-slate-400'"
          @click="tab = 'calc'"
        >类型汇总</button>
        <button
          class="rounded-md px-3 py-1 text-sm font-medium transition"
          :class="tab === 'formula' ? 'bg-blue-600 text-white' : 'text-slate-500 dark:text-slate-400'"
          @click="tab = 'formula'"
        >公式速查</button>
      </div>
    </div>

    <!-- 类型汇总（手风琴） -->
    <div v-if="tab === 'calc'">
      <el-input v-model="keyword" placeholder="搜索类型 / 分类" clearable class="mb-4" style="max-width: 320px">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>

      <el-collapse>
        <el-collapse-item v-for="c in filtered" :key="c.id" :name="c.id">
          <template #title>
            <div class="flex items-center gap-2">
              <el-tag size="small" type="info">{{ c.category }}</el-tag>
              <span class="font-medium text-slate-800 dark:text-slate-100">{{ c.name }}</span>
            </div>
          </template>

          <div class="space-y-4 px-1 pb-2">
            <div>
              <div class="mb-1 text-sm font-semibold text-blue-600 dark:text-blue-400">公式</div>
              <ul class="space-y-1">
                <li v-for="(f, i) in c.formulas" :key="i" class="rounded bg-slate-50 px-3 py-1.5 font-mono text-sm text-slate-700 dark:bg-slate-800 dark:text-slate-200">{{ f }}</li>
              </ul>
            </div>
            <div>
              <div class="mb-1 text-sm font-semibold text-slate-900 dark:text-slate-100">解题方法</div>
              <p class="text-sm leading-relaxed text-slate-600 dark:text-slate-300">{{ c.method }}</p>
            </div>
            <div>
              <div class="mb-1 text-sm font-semibold text-emerald-600 dark:text-emerald-400">典型例题</div>
              <p class="rounded-lg bg-emerald-50 px-3 py-2 text-sm leading-relaxed text-slate-700 dark:bg-emerald-500/10 dark:text-slate-200">{{ c.example }}</p>
            </div>
            <div>
              <div class="mb-1 text-sm font-semibold text-rose-600 dark:text-rose-400">易错点</div>
              <ul class="list-disc space-y-1 pl-5 text-sm text-slate-600 dark:text-slate-300">
                <li v-for="(p, i) in c.pitfalls" :key="i">{{ p }}</li>
              </ul>
            </div>
          </div>
        </el-collapse-item>
      </el-collapse>
    </div>

    <!-- 公式速查 -->
    <div v-else class="space-y-5">
      <section
        v-for="g in formulaGroups"
        :key="g.category"
        class="rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <h3 class="mb-3 font-semibold text-slate-900 dark:text-white">{{ g.category }}</h3>
        <div class="space-y-3">
          <div v-for="item in g.items" :key="item.name">
            <div class="mb-1 text-sm font-medium text-slate-700 dark:text-slate-200">{{ item.name }}</div>
            <ul class="space-y-1">
              <li v-for="(f, i) in item.formulas" :key="i" class="rounded bg-slate-50 px-3 py-1.5 font-mono text-sm text-slate-700 dark:bg-slate-800 dark:text-slate-200">{{ f }}</li>
            </ul>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
