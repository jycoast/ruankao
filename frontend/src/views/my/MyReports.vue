<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { reportApi } from '@/api'

interface MyReport {
  id: number
  question_id: number
  report_type: number
  description: string
  status: number
  admin_note: string
  created_at: string
  question_preview: string
}

const router = useRouter()
const list = ref<MyReport[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res: any = await reportApi.mine({ page: page.value, page_size: 20 })
    list.value = res.data.items || []
    total.value = res.data.total || 0
  } finally {
    loading.value = false
  }
}

onMounted(load)

const TYPE_LABELS: Record<number, string> = {
  1: '题目有误', 2: '答案有误', 3: '解析有误', 4: '选项有误', 5: '其他',
}

function statusTag(s: number) {
  return { 0: { label: '待处理', type: 'info' }, 1: { label: '已修正', type: 'success' }, 2: { label: '核对无误', type: 'warning' } }[s]
    || { label: '未知', type: '' }
}
</script>

<template>
  <div class="mx-auto max-w-3xl">
    <div class="mb-6 flex items-center gap-3">
      <el-button text @click="router.back()">返回</el-button>
      <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100">我的纠错</h2>
    </div>

    <el-card v-loading="loading">
      <el-empty v-if="!list.length && !loading" description="你还没有提交过纠错" />

      <div v-else class="space-y-4">
        <div v-for="item in list" :key="item.id" class="rounded-lg border p-4 dark:border-gray-700">
          <div class="flex items-start justify-between gap-2">
            <div class="flex-1 text-sm text-gray-500">
              {{ TYPE_LABELS[item.report_type] || '其他' }}
            </div>
            <el-tag :type="(statusTag(item.status) as any).type" size="small">
              {{ statusTag(item.status).label }}
            </el-tag>
          </div>

          <router-link
            v-if="item.question_preview"
            :to="`/questions/${item.question_id}`"
            class="mt-1 block truncate text-sm font-medium text-gray-800 hover:underline dark:text-gray-200"
          >
            {{ item.question_preview }}
          </router-link>

          <div class="mt-2 whitespace-pre-line text-sm text-gray-600 dark:text-gray-400">
            {{ item.description }}
          </div>

          <div v-if="item.admin_note" class="mt-2 rounded bg-gray-50 p-2 text-sm text-gray-600 dark:bg-gray-800 dark:text-gray-400">
            管理员回复：{{ item.admin_note }}
          </div>

          <div class="mt-2 text-xs text-gray-400">提交于 {{ item.created_at?.slice(0, 10) }}</div>
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
