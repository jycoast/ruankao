<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { notificationApi } from '@/api'

interface NotificationItem {
  id: number
  title: string
  content: string
  link: string
  read_at: string | null
  created_at: string
}

const router = useRouter()
const list = ref<NotificationItem[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res: any = await notificationApi.list({ page: page.value, page_size: 20 })
    list.value = res.data.items || []
    total.value = res.data.total || 0
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function open(n: NotificationItem) {
  if (!n.read_at) {
    try {
      await notificationApi.markRead(n.id)
      n.read_at = new Date().toISOString()
    } catch {
      /* 标记失败不该挡住跳转 */
    }
  }
  if (n.link) router.push(n.link)
}

async function markAll() {
  try {
    await notificationApi.markAllRead()
    list.value.forEach((n) => (n.read_at = n.read_at || new Date().toISOString()))
  } catch {
    ElMessage.error('操作失败')
  }
}
</script>

<template>
  <div class="mx-auto max-w-3xl">
    <div class="mb-6 flex items-center justify-between">
      <div class="flex items-center gap-3">
        <el-button text @click="router.back()">返回</el-button>
        <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100">我的消息</h2>
      </div>
      <el-button @click="markAll">全部已读</el-button>
    </div>

    <el-card v-loading="loading">
      <el-empty v-if="!list.length && !loading" description="暂无消息" />

      <div v-else class="space-y-3">
        <button
          v-for="n in list"
          :key="n.id"
          class="block w-full rounded-lg border p-4 text-left transition hover:border-blue-300"
          :class="n.read_at
            ? 'border-gray-100 dark:border-gray-700'
            : 'border-blue-200 bg-blue-50/60 dark:border-blue-500/30 dark:bg-blue-500/10'"
          @click="open(n)"
        >
          <div class="flex items-start justify-between gap-2">
            <span class="font-medium text-gray-900 dark:text-gray-100">{{ n.title }}</span>
            <span v-if="!n.read_at" class="mt-2 h-2 w-2 shrink-0 rounded-full bg-blue-500" />
          </div>
          <div class="mt-1 whitespace-pre-line text-sm text-gray-600 dark:text-gray-300">{{ n.content }}</div>
          <div class="mt-2 text-xs text-gray-400">{{ n.created_at }}</div>
        </button>
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
