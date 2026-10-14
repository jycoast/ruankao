<script setup lang="ts">
import { onBeforeUnmount, onMounted } from "vue"
import { ElConfigProvider } from "element-plus"
import zhCn from "element-plus/es/locale/lang/zh-cn"
import dayjs from "dayjs"
import "dayjs/locale/zh-cn"
import { startPwaUpdate, stopPwaUpdate } from "@/composables/usePwaUpdate"

// 全局中文:el-date-picker 等组件、dayjs 均使用中文区域。
dayjs.locale("zh-cn")
const locale = zhCn

// PWA:注册 Service Worker + SW 驱动的更新检测(替代原 version.json 轮询,见 composables/usePwaUpdate.ts)。
onMounted(startPwaUpdate)
onBeforeUnmount(stopPwaUpdate)
</script>

<template>
  <el-config-provider :locale="locale">
    <router-view />
  </el-config-provider>
</template>
