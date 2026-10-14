<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { favoriteApi } from '@/api'
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { loginGuestRoute } from '@/utils/guestGate'

const favorites = ref<any[]>([])
const activeTab = ref('1')
const page = ref(1)
const total = ref(0)
const pageSize = 20

const router = useRouter()
const userStore = useUserStore()
const isGuest = computed(() => userStore.isGuest)

async function loadFavorites(type: number, p = 1) {
  const res: any = await favoriteApi.list({ target_type: type, page: p, page_size: pageSize })
  favorites.value = res.data?.items || []
  total.value = Number(res.data?.total || 0)
  page.value = p
}

onMounted(() => loadFavorites(1))

function handleChange(tab: string) {
  // 切 tab 从第一页重新拉
  loadFavorites(Number(tab), 1)
}

function onPageChange(p: number) {
  loadFavorites(Number(activeTab.value), p)
}

async function removeFav(targetType: number, targetId: number) {
  await favoriteApi.remove(targetType, targetId)
  // 删完当页可能少于 pageSize，若还有后页且当前页已空则回退一页
  const p = favorites.value.length <= 1 && page.value > 1 ? page.value - 1 : page.value
  loadFavorites(Number(activeTab.value), p)
}
</script>

<template>
  <div>
    <h2 class="text-xl font-bold mb-4">我的收藏</h2>
    <el-tabs v-model="activeTab" @tab-change="handleChange">
      <el-tab-pane label="题目" name="1" />
      <el-tab-pane label="论文" name="2" />
      <el-tab-pane label="知识点" name="3" />
    </el-tabs>

    <div v-if="favorites.length === 0" class="py-12 text-center">
      <template v-if="isGuest">
        <p class="text-gray-500">登录后同步你的收藏</p>
        <el-button class="mt-3" type="primary" plain size="small" @click="router.push(loginGuestRoute())">去登录</el-button>
      </template>
      <p v-else class="text-gray-500">暂无收藏</p>
    </div>

    <div class="space-y-3">
      <div v-for="fav in favorites" :key="fav.id" class="bg-white p-4 rounded-lg shadow flex items-center justify-between">
        <div>
          <router-link
            v-if="Number(activeTab) === 1"
            :to="`/questions/${fav.id}`"
            class="text-gray-800 hover:text-blue-500"
          >{{ fav.title }}</router-link>
          <router-link
            v-else-if="Number(activeTab) === 2"
            :to="`/essays/${fav.id}`"
            class="text-gray-800 hover:text-blue-500"
          >{{ fav.title }}</router-link>
          <router-link
            v-else
            :to="`/knowledge/${fav.id}`"
            class="text-gray-800 hover:text-blue-500"
          >{{ fav.name }}</router-link>
        </div>
        <el-button size="small" type="danger" @click="removeFav(Number(activeTab), fav.id)">取消收藏</el-button>
      </div>
    </div>

    <div v-if="total > pageSize" class="mt-6 flex justify-center">
      <el-pagination
        background
        layout="prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="onPageChange"
      />
    </div>
  </div>
</template>
