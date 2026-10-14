<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ChatLineSquare } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { notificationApi } from '@/api'
import { useUserStore } from '@/stores/user'

interface NotificationItem {
  id: number
  title: string
  content: string
  link: string
  read_at: string | null
  created_at: string
}

const router = useRouter()
const userStore = useUserStore()
const list = ref<NotificationItem[]>([])
const unread = ref(0)
const visible = ref(false)

async function load() {
  if (!userStore.isLoggedIn) {
    list.value = []
    unread.value = 0
    return
  }
  try {
    const res: any = await notificationApi.unreadCount()
    unread.value = res.data?.count || 0
  } catch {
    unread.value = 0
  }
}

async function openPanel() {
  visible.value = true
  try {
    const res: any = await notificationApi.list({ page: 1, page_size: 10 })
    list.value = res.data?.items || []
  } catch {
    list.value = []
  }
}

onMounted(load)
watch(() => userStore.isLoggedIn, load)

async function open(n: NotificationItem) {
  if (!n.read_at) {
    try {
      await notificationApi.markRead(n.id)
      n.read_at = new Date().toISOString()
      unread.value = Math.max(0, unread.value - 1)
    } catch {
      /* 标记失败不该挡住跳转 */
    }
  }
  visible.value = false
  if (n.link) router.push(n.link)
}

async function markAll() {
  try {
    await notificationApi.markAllRead()
    list.value.forEach((n) => (n.read_at = n.read_at || new Date().toISOString()))
    unread.value = 0
  } catch {
    ElMessage.error('操作失败')
  }
}
</script>

<template>
  <template v-if="userStore.isLoggedIn">
    <el-badge :value="unread" :hidden="unread === 0" :max="99" class="mr-1">
      <button
        class="flex h-7 w-7 items-center justify-center rounded-md border border-slate-200 bg-white text-slate-600 transition hover:border-blue-200 hover:text-blue-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
        title="消息"
        @click="openPanel"
      >
        <el-icon :size="16"><ChatLineSquare /></el-icon>
      </button>
    </el-badge>

    <el-dialog v-model="visible" title="我的消息" width="520px" append-to-body>
      <div v-if="list.length === 0" class="py-8 text-center text-gray-400">暂无消息</div>
      <div v-else class="max-h-[60vh] space-y-2 overflow-y-auto">
        <button
          v-for="n in list"
          :key="n.id"
          class="block w-full rounded-lg border p-3 text-left transition hover:border-blue-300"
          :class="n.read_at
            ? 'border-gray-100 dark:border-gray-700'
            : 'border-blue-200 bg-blue-50/60 dark:border-blue-500/30 dark:bg-blue-500/10'"
          @click="open(n)"
        >
          <div class="flex items-start justify-between gap-2">
            <span class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ n.title }}</span>
            <span v-if="!n.read_at" class="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-blue-500" />
          </div>
          <div class="mt-1 line-clamp-2 whitespace-pre-line text-xs text-gray-600 dark:text-gray-300">
            {{ n.content }}
          </div>
          <div class="mt-1 text-xs text-gray-400">{{ n.created_at }}</div>
        </button>
      </div>
      <template #footer>
        <div class="flex justify-between">
          <el-button size="small" :disabled="unread === 0" @click="markAll">全部已读</el-button>
          <el-button size="small" type="primary" plain @click="visible = false; router.push('/my/notifications')">
            查看全部
          </el-button>
        </div>
      </template>
    </el-dialog>
  </template>
</template>
