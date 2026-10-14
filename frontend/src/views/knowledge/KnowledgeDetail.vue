<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { knowledgeApi, favoriteApi } from '@/api'
import { useUserStore } from '@/stores/user'
import { usePageMeta } from '@/composables/usePageMeta'
import { renderMd } from '@/utils/markdown'
import { useImagePreview } from '@/composables/useImagePreview'
import type { KnowledgePoint } from '@/types'

interface RelatedQuestion {
  id: number
  title: string
  question_type: number
  year: number
}

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const kp = ref<KnowledgePoint | null>(null)
const isFavorited = ref(false)
const relatedQuestions = ref<RelatedQuestion[]>([])
const { visible, urlList, initialIndex, onContainerClick, close } = useImagePreview()

const typeLabel: Record<number, string> = { 1: '单选', 2: '多选', 3: '案例', 4: '论文' }

onMounted(async () => {
  const id = Number(route.params.id)
  const res: any = await knowledgeApi.getById(id)
  kp.value = res.data
  usePageMeta(`${res.data.name} - 软考知识导图`, res.data.content)
  // 收藏是登录态功能。这页对游客和爬虫开放，未登录时不能去碰它 ——
  // 401 会被拦截器当成登录过期，直接把访客踢去登录页。
  if (userStore.isLoggedIn) {
    try {
      const fRes: any = await favoriteApi.check({ target_type: 3, target_id: id })
      isFavorited.value = fRes.data.exists
    } catch {}
  }
  try {
    const qRes: any = await knowledgeApi.relatedQuestions(id)
    relatedQuestions.value = qRes.data || []
  } catch {}
})

async function toggleFavorite() {
  if (!kp.value) return
  if (!userStore.isLoggedIn) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  if (isFavorited.value) {
    await favoriteApi.remove(3, kp.value.id)
  } else {
    await favoriteApi.add({ target_type: 3, target_id: kp.value.id })
  }
  isFavorited.value = !isFavorited.value
}

// 知识点内容为完整 Markdown（管理端用 FullScreenEditor 编辑），
// 与知识导图 MindmapView 一致走 renderMd 渲染，含标题/加粗/列表/表格/代码块/图片，
// 图片未加载占位与 XSS 净化都在 renderMd 里统一处理。

</script>

<template>
  <div v-if="kp" class="max-w-3xl mx-auto">
    <div class="bg-white p-6 rounded-lg shadow dark:bg-gray-800">
      <div class="flex items-center justify-between mb-4">
        <h2 class="text-2xl font-bold text-gray-900 dark:text-gray-100">{{ kp.name }}</h2>
        <el-button :type="isFavorited ? 'warning' : 'info'" @click="toggleFavorite">
          <el-icon><Star /></el-icon> {{ isFavorited ? '已收藏' : '收藏' }}
        </el-button>
      </div>

      <div v-if="kp.content" class="mb-6">
        <h3 class="font-bold text-lg mb-2 text-gray-900 dark:text-gray-100">内容</h3>
        <div class="md-content text-gray-600 dark:text-gray-300 leading-relaxed" v-html="renderMd(kp.content)" @click="onContainerClick"></div>
      </div>

      <div v-if="kp.key_points" class="mb-6">
        <h3 class="font-bold text-lg mb-2 text-blue-600">考点</h3>
        <div class="md-content text-gray-600 dark:text-gray-300 leading-relaxed" v-html="renderMd(kp.key_points)" @click="onContainerClick"></div>
      </div>

      <div v-if="kp.easy_mistakes" class="mb-6">
        <h3 class="font-bold text-lg mb-2 text-red-600">易错点</h3>
        <div class="md-content text-gray-600 dark:text-gray-300 leading-relaxed" v-html="renderMd(kp.easy_mistakes)" @click="onContainerClick"></div>
      </div>

      <div v-if="relatedQuestions.length" class="mt-8 pt-6 border-t border-gray-100 dark:border-gray-700">
        <h3 class="font-bold text-lg mb-3 text-gray-900 dark:text-gray-100">
          相关真题 <span class="text-sm font-normal text-gray-400">({{ relatedQuestions.length }})</span>
        </h3>
        <div class="space-y-2">
          <router-link
            v-for="q in relatedQuestions"
            :key="q.id"
            :to="`/questions/${q.id}`"
            class="flex items-start gap-2 p-3 rounded-lg bg-gray-50 hover:bg-blue-50 dark:bg-gray-700/50 dark:hover:bg-gray-700 transition"
          >
            <span class="shrink-0 mt-0.5 px-1.5 py-0.5 rounded text-xs bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
              {{ q.year ? q.year + '年' : '' }}{{ typeLabel[q.question_type] || '' }}
            </span>
            <span class="text-sm text-gray-700 dark:text-gray-200 line-clamp-2">{{ q.title }}</span>
          </router-link>
        </div>
      </div>
    </div>

    <div class="mt-4">
      <el-button @click="$router.back()">返回</el-button>
    </div>

    <el-image-viewer
      v-if="visible"
      :url-list="urlList"
      :initial-index="initialIndex"
      :hide-on-click-modal="true"
      @close="close"
    />
  </div>
</template>

<style scoped>
:deep(.md-content img) {
  cursor: zoom-in;
}
</style>
