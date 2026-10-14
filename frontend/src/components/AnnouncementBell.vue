<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { Bell } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { announcementApi } from '@/api'
import { useUserStore } from '@/stores/user'
import { renderMd } from '@/utils/markdown'
import type { Announcement } from '@/types'

const userStore = useUserStore()
const list = ref<Announcement[]>([])
const dialogVisible = ref(false)
const autoPopped = ref(false)

async function load() {
  if (!userStore.isLoggedIn) {
    list.value = []
    return
  }
  try {
    const res: any = await announcementApi.listUnread()
    list.value = res.data || []
    // 首次拉到未读时自动弹一次
    if (list.value.length > 0 && !autoPopped.value) {
      autoPopped.value = true
      dialogVisible.value = true
    }
  } catch {
    list.value = []
  }
}

onMounted(load)
// 登录状态变化时（登录/退出）重新拉取
watch(() => userStore.isLoggedIn, () => {
  autoPopped.value = false
  load()
})

async function markRead(a: Announcement) {
  try {
    await announcementApi.markRead(a.id)
    list.value = list.value.filter((x) => x.id !== a.id)
    if (list.value.length === 0) dialogVisible.value = false
  } catch {
    ElMessage.error('操作失败')
  }
}
</script>

<template>
  <template v-if="userStore.isLoggedIn">
    <el-badge :value="list.length" :hidden="list.length === 0" :max="99" class="mr-1">
      <button
        class="flex h-7 w-7 items-center justify-center rounded-md border border-slate-200 bg-white text-slate-600 transition hover:border-blue-200 hover:text-blue-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
        title="公告"
        @click="dialogVisible = true"
      >
        <el-icon :size="16"><Bell /></el-icon>
      </button>
    </el-badge>

    <el-dialog v-model="dialogVisible" title="公告" width="560px" append-to-body>
      <div v-if="list.length === 0" class="py-8 text-center text-gray-400">
        暂无未读公告
      </div>
      <div v-else class="space-y-4 max-h-[60vh] overflow-y-auto">
        <div
          v-for="a in list"
          :key="a.id"
          class="rounded-lg border border-gray-100 dark:border-gray-700 p-4"
        >
          <div class="flex items-start justify-between gap-3 mb-2">
            <div class="text-base font-semibold text-gray-900 dark:text-gray-100">{{ a.title }}</div>
            <el-button size="small" type="primary" plain @click="markRead(a)">我已阅读</el-button>
          </div>
          <div class="text-xs text-gray-400 mb-2">{{ a.created_at }}</div>
          <div class="md-content text-sm text-gray-600 dark:text-gray-300" v-html="renderMd(a.content)"></div>
        </div>
      </div>
    </el-dialog>
  </template>
</template>
