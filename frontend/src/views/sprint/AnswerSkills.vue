<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { CASE_SKILLS, ESSAY_TEMPLATE } from '@/data/sprintSkills'

const router = useRouter()
const tab = ref<'case' | 'essay'>('case')
</script>

<template>
  <div class="mx-auto max-w-3xl">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100">答题套路</h2>
      <div class="flex items-center gap-1 rounded-lg border border-slate-200 bg-white p-0.5 dark:border-slate-700 dark:bg-slate-800">
        <button class="rounded-md px-3 py-1 text-sm font-medium transition" :class="tab === 'case' ? 'bg-blue-600 text-white' : 'text-slate-500 dark:text-slate-400'" @click="tab = 'case'">案例分析</button>
        <button class="rounded-md px-3 py-1 text-sm font-medium transition" :class="tab === 'essay' ? 'bg-blue-600 text-white' : 'text-slate-500 dark:text-slate-400'" @click="tab = 'essay'">论文写作</button>
      </div>
    </div>

    <div class="space-y-4">
      <section
        v-for="s in (tab === 'case' ? CASE_SKILLS : ESSAY_TEMPLATE)"
        :key="s.title"
        class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      >
        <h3 class="font-semibold text-slate-900 dark:text-white">{{ s.title }}</h3>
        <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ s.intro }}</p>
        <ol class="mt-3 space-y-2">
          <li v-for="(step, i) in s.steps" :key="i" class="flex items-start gap-2.5 text-sm leading-relaxed text-slate-700 dark:text-slate-200">
            <span class="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-blue-50 text-xs font-medium text-blue-600 dark:bg-blue-500/15 dark:text-blue-300">{{ i + 1 }}</span>
            <span>{{ step }}</span>
          </li>
        </ol>
      </section>
    </div>

    <div class="mt-6 flex flex-wrap gap-3">
      <el-button type="primary" @click="router.push('/essays')">查看论文范文库</el-button>
      <el-button @click="router.push('/essays/submit')">提交我的论文</el-button>
    </div>
  </div>
</template>
