<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { articleApi, favoriteApi } from '@/api'
import { useUserStore } from '@/stores/user'
import { usePageMeta } from '@/composables/usePageMeta'
import { renderMd } from '@/utils/markdown'
import { cleanEssayText, parseEssaySections } from '@/utils/essay'
import type { Article } from '@/types'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const article = ref<Article | null>(null)
const isFavorited = ref(false)
const activeId = ref('')

onMounted(async () => {
  const id = Number(route.params.id)
  const res: any = await articleApi.getById(id)
  article.value = res.data
  usePageMeta(`${res.data.title} - 软考论文范文`, res.data.content)
  checkFavorite()
})

const sections = computed(() => {
  if (!article.value?.content) return []
  return parseEssaySections(cleanEssayText(article.value.content))
})

// 收藏是登录态功能。这页对游客和爬虫开放，未登录时不能去碰它 ——
// 401 会被拦截器当成登录过期，直接把访客踢去登录页。
async function checkFavorite() {
  if (!article.value || !userStore.isLoggedIn) return
  try {
    const res: any = await favoriteApi.check({ target_type: 2, target_id: article.value.id })
    isFavorited.value = res.data.exists
  } catch {}
}

async function toggleFavorite() {
  if (!article.value) return
  if (!userStore.isLoggedIn) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  if (isFavorited.value) {
    await favoriteApi.remove(2, article.value.id)
  } else {
    await favoriteApi.add({ target_type: 2, target_id: article.value.id })
  }
  isFavorited.value = !isFavorited.value
}

function scrollTo(id: string) {
  activeId.value = id
  const el = document.getElementById(id)
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function typeLabel(t: number) {
  return { 1: '论文范文', 2: '写作模板', 3: '评分标准', 4: 'AI点评' }[t] || '其他'
}
function typeTag(t: number) {
  return ({ 1: 'success', 2: 'primary', 3: 'warning', 4: 'info' } as const)[t as 1] || 'info'
}
</script>

<template>
  <div v-if="article" class="mx-auto max-w-5xl">
    <!-- 返回条 -->
    <button
      class="mb-4 flex items-center gap-1 text-sm text-slate-500 transition hover:text-blue-600 dark:text-slate-400"
      @click="router.back()"
    >
      <el-icon><ArrowLeft /></el-icon> 返回论文库
    </button>

    <div class="grid gap-6 lg:grid-cols-[200px_1fr]">
      <!-- 目录（桌面 sticky / 移动折叠） -->
      <aside class="lg:sticky lg:top-20 lg:self-start">
        <!-- 移动端下拉 -->
        <details class="rounded-xl border border-slate-200 bg-white p-3 lg:hidden dark:border-slate-800 dark:bg-slate-900">
          <summary class="cursor-pointer text-sm font-medium text-slate-700 dark:text-slate-200">目录（{{ sections.length }} 节）</summary>
          <nav class="mt-2 space-y-1">
            <button
              v-for="s in sections"
              :key="s.id"
              class="block w-full truncate rounded px-2 py-1.5 text-left text-sm text-slate-600 transition hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800"
              @click="scrollTo(s.id)"
            >{{ s.title }}</button>
          </nav>
        </details>
        <!-- 桌面目录 -->
        <nav class="hidden rounded-xl border border-slate-200 bg-white p-3 lg:block dark:border-slate-800 dark:bg-slate-900">
          <div class="mb-2 px-2 text-xs font-semibold uppercase tracking-wide text-slate-400">目录</div>
          <button
            v-for="s in sections"
            :key="s.id"
            class="block w-full truncate rounded-md border-l-2 px-2 py-1.5 text-left text-sm transition"
            :class="activeId === s.id
              ? 'border-blue-500 bg-blue-50 font-medium text-blue-700 dark:bg-blue-500/15 dark:text-blue-300'
              : 'border-transparent text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-800'"
            @click="scrollTo(s.id)"
          >{{ s.title }}</button>
        </nav>
      </aside>

      <!-- 正文 -->
      <article class="min-w-0 rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <header class="border-b border-slate-100 p-5 sm:p-6 dark:border-slate-800">
          <div class="flex flex-wrap items-center gap-2">
            <el-tag :type="typeTag(article.article_type)" size="small">{{ typeLabel(article.article_type) }}</el-tag>
            <span v-if="article.year" class="text-xs text-slate-400">{{ article.year }}年</span>
            <span v-if="article.author" class="text-xs text-slate-400">作者：{{ article.author }}</span>
            <span
              v-if="article.score"
              class="rounded-full bg-orange-50 px-2 py-0.5 text-xs font-medium text-orange-600 dark:bg-orange-500/15 dark:text-orange-300"
            >{{ article.score }} 分</span>
            <button
              class="ml-auto flex items-center gap-1 rounded-lg border px-2.5 py-1 text-xs font-medium transition"
              :class="isFavorited
                ? 'border-amber-300 bg-amber-50 text-amber-600 dark:border-amber-500/40 dark:bg-amber-500/15 dark:text-amber-300'
                : 'border-slate-200 text-slate-500 hover:border-amber-300 hover:text-amber-600 dark:border-slate-700 dark:text-slate-400'"
              @click="toggleFavorite"
            >
              <el-icon><Star /></el-icon>{{ isFavorited ? '已收藏' : '收藏' }}
            </button>
          </div>
          <h1 class="mt-3 text-xl font-bold text-slate-900 sm:text-2xl dark:text-white">{{ article.title }}</h1>
        </header>

        <div class="p-5 sm:p-6">
          <section v-for="s in sections" :key="s.id" :id="s.id" class="scroll-mt-20 mb-8 last:mb-0">
            <h2 class="mb-3 flex items-center gap-2 text-base font-semibold text-slate-900 dark:text-white">
              <span class="inline-block h-4 w-1 rounded-full bg-blue-500" />
              {{ s.title }}
            </h2>
            <div class="essay-prose md-content text-slate-700 dark:text-slate-300" v-html="renderMd(s.body)" />
          </section>

          <div v-if="!sections.length" class="py-12 text-center text-sm text-slate-500">暂无内容</div>
        </div>
      </article>
    </div>
  </div>
</template>
