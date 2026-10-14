<script setup lang="ts">
import { useRouter, useRoute } from 'vue-router'
import { HomeFilled, ArrowLeft } from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()
</script>

<template>
  <div class="flex min-h-screen flex-col items-center justify-center gap-6 bg-slate-50 px-4 text-center dark:bg-slate-950">
    <!-- 404 主视觉：渐变数字 + 底部投影 -->
    <div class="relative select-none">
      <div class="bg-gradient-to-br from-blue-500 to-indigo-600 bg-clip-text text-[8rem] font-black leading-none text-transparent dark:from-blue-400 dark:to-indigo-500">
        404
      </div>
      <div class="absolute inset-x-0 bottom-1 mx-auto h-3 w-4/5 rounded-full bg-slate-300/70 blur-md dark:bg-slate-800"></div>
    </div>

    <div class="space-y-2">
      <h1 class="text-2xl font-bold text-slate-800 dark:text-slate-100">页面走丢了</h1>
      <p class="text-slate-500 dark:text-slate-400">
        你访问的
        <span class="font-mono text-blue-500 dark:text-blue-400">{{ route.fullPath }}</span>
        不存在，可能已被移动或删除。
      </p>
    </div>

    <div class="flex flex-wrap justify-center gap-3">
      <el-button size="large" @click="router.back()">
        <el-icon class="mr-1"><ArrowLeft /></el-icon>返回上一页
      </el-button>
      <el-button size="large" type="primary" @click="router.push('/')">
        <el-icon class="mr-1"><HomeFilled /></el-icon>回到首页
      </el-button>
    </div>
  </div>
</template>
