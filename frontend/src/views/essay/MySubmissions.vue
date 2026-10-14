<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { essaySubmissionApi } from '@/api'
import type { EssaySubmission } from '@/types'

const router = useRouter()
const list = ref<EssaySubmission[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res: any = await essaySubmissionApi.mySubmissions({ page: page.value, page_size: 20 })
    list.value = res.data.items || []
    total.value = res.data.total || 0
  } finally {
    loading.value = false
  }
}

onMounted(load)

function statusTag(s: number) {
  return { 0: { label: '待审核', type: 'info' }, 1: { label: '已通过', type: 'success' }, 2: { label: '已拒绝', type: 'danger' } }[s] || { label: '未知', type: '' }
}
</script>

<template>
  <div class="max-w-3xl mx-auto">
    <div class="flex items-center justify-between mb-6">
      <div class="flex items-center gap-3">
        <el-button @click="router.back()" text>返回</el-button>
        <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100">我的投稿</h2>
      </div>
      <el-button type="primary" @click="router.push('/essays/submit')">提交新论文</el-button>
    </div>

    <el-card v-loading="loading">
      <el-empty v-if="!list.length && !loading" description="暂无投稿记录" />

      <div v-else class="space-y-4">
        <div
          v-for="item in list"
          :key="item.id"
          class="border rounded-lg p-4 dark:border-gray-700"
        >
          <div class="flex items-start justify-between gap-2">
            <div class="font-medium text-gray-800 dark:text-gray-200 flex-1">{{ item.title }}</div>
            <el-tag :type="(statusTag(item.review_status) as any).type" size="small">
              {{ statusTag(item.review_status).label }}
            </el-tag>
          </div>
          <div class="mt-2 text-sm text-gray-500 flex flex-wrap gap-4">
            <span v-if="item.category">分类：{{ item.category }}</span>
            <span v-if="item.year">年份：{{ item.year }}</span>
            <span v-if="item.score">得分：{{ item.score }}</span>
            <span>投稿时间：{{ item.created_at?.slice(0, 10) }}</span>
          </div>
          <div v-if="item.reviewer_note" class="mt-2 text-sm text-gray-600 dark:text-gray-400 bg-gray-50 dark:bg-gray-800 p-2 rounded">
            审核意见：{{ item.reviewer_note }}
          </div>
          <div v-if="item.review_status === 1 && item.article_id" class="mt-2">
            <router-link :to="`/essays/${item.article_id}`" class="text-blue-500 text-sm hover:underline">
              查看已发布范文 →
            </router-link>
          </div>
        </div>
      </div>

      <div class="mt-4 flex justify-center">
        <el-pagination
          v-model:current-page="page"
          :page-size="20"
          :total="total"
          layout="prev, pager, next"
          @current-change="load"
        />
      </div>
    </el-card>
  </div>
</template>
