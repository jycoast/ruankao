<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useSubjectStore } from '@/stores/subject'
import SubjectSelectDialog from '@/components/SubjectSelectDialog.vue'

const userStore = useUserStore()
const subjectStore = useSubjectStore()
const router = useRouter()
const route = useRoute()

const MOBILE_BREAKPOINT = 1024
const isMobile = ref(false)
const drawerOpen = ref(false)

function syncViewport() {
  const mobile = window.innerWidth < MOBILE_BREAKPOINT
  isMobile.value = mobile
  if (!mobile) drawerOpen.value = false
}

function openMenu() {
  drawerOpen.value = true
}

function closeMenu() {
  drawerOpen.value = false
}

// 切换科目后整页刷新：监听 currentSubjectId 的实际变化,而不是 dialog 关闭。
// 旧实现监听 dialog 的 visible,在弹窗任意关闭(包括点遮罩/ESC/关闭按钮)时都会 reload,
// 但这些路径 SubjectSelectDialog.pick() 没被调用,store 没变 —— 用户感觉"切换没生效"。
// 改成监听 currentSubjectId:真正变化才 reload,关弹窗不切换不再误触。
//
// 刷新前必须先等科目偏好同步落定:reload 会中断在途的 PUT,服务端 subject_id 仍是旧值,
// 重载后 router.beforeEach → restoreFromServer(旧值) 会把刚切的新科目回滚掉
// (表现为"切完几百毫秒后又跳回旧科目",且是否复现取决于 PUT 有没有赶在导航前发出去)。
// flushPendingSync 自带超时兜底,网络卡住也不会把刷新挂死。
watch(
  () => subjectStore.currentSubjectId,
  async (id, prev) => {
    if (!id || !prev || id === prev) return
    await subjectStore.flushPendingSync()
    window.location.reload()
  },
)
const subjectDialogVisible = ref(false)

const pageTitle = computed(() => {
  const map: Record<string, string> = {
    '/admin': '仪表盘',
    '/admin/questions': '题库管理',
    '/admin/papers': '试卷管理',
    '/admin/knowledge': '知识点管理',
    '/admin/articles': '论文管理',
    '/admin/essay-submissions': '论文投稿审核',
    '/admin/announcements': '公告管理',
    '/admin/exam-schedules': '考试排期',
    '/admin/users': '用户管理',
    '/admin/password-resets': '找回密码申请',
    '/admin/invite-codes': '邀请码管理',
    '/admin/membership-plans': '会员价格',
    '/admin/card-keys': '卡密管理',
    '/admin/orders': '订单管理',
    '/admin/reports': '错误报告',
    '/admin/feedbacks': '意见反馈',
    '/admin/comments': '评论管理',
    '/admin/comment-reports': '举报处理',
    '/admin/ai': 'AI 管理',
  }
  return map[route.path] || '管理后台'
})

watch(
  () => route.path,
  () => {
    if (isMobile.value) closeMenu()
  },
)

onMounted(() => {
  subjectStore.loadSubjects()
  syncViewport()
  window.addEventListener('resize', syncViewport)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', syncViewport)
})
</script>

<template>
  <div class="admin-shell flex h-screen overflow-hidden bg-gray-50 text-gray-900 dark:bg-gray-950 dark:text-gray-100">
    <!-- Desktop sidebar -->
    <aside
      class="admin-sidebar hidden h-full w-[220px] shrink-0 flex-col overflow-y-auto border-r border-gray-800 bg-gray-900 text-white lg:flex"
    >
      <div class="border-b border-gray-700 p-4 text-center text-xl font-bold">
        管理后台
      </div>
      <el-menu
        :default-active="route.path"
        router
        :unique-opened="true"
        background-color="#111827"
        text-color="#9ca3af"
        active-text-color="#60a5fa"
        class="admin-menu border-r-0"
      >
        <el-menu-item index="/admin">
          <el-icon><DataBoard /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>
        <el-sub-menu index="content">
          <template #title>
            <el-icon><Collection /></el-icon>
            <span>内容管理</span>
          </template>
          <el-menu-item index="/admin/questions"><span>题库管理</span></el-menu-item>
          <el-menu-item index="/admin/papers"><span>试卷管理</span></el-menu-item>
          <el-menu-item index="/admin/knowledge"><span>知识点管理</span></el-menu-item>
          <el-menu-item index="/admin/articles"><span>论文管理</span></el-menu-item>
          <el-menu-item index="/admin/essay-submissions"><span>论文投稿审核</span></el-menu-item>
          <el-menu-item index="/admin/announcements"><span>公告管理</span></el-menu-item>
          <el-menu-item index="/admin/exam-schedules"><span>考试排期</span></el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="users">
          <template #title>
            <el-icon><User /></el-icon>
            <span>用户管理</span>
          </template>
          <el-menu-item index="/admin/users"><span>用户管理</span></el-menu-item>
          <el-menu-item index="/admin/password-resets"><span>找回密码申请</span></el-menu-item>
          <el-menu-item index="/admin/invite-codes"><span>邀请码管理</span></el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="membership">
          <template #title>
            <el-icon><GoldMedal /></el-icon>
            <span>会员与订单</span>
          </template>
          <el-menu-item index="/admin/membership-plans"><span>会员价格</span></el-menu-item>
          <el-menu-item index="/admin/card-keys"><span>卡密管理</span></el-menu-item>
          <el-menu-item index="/admin/orders"><span>订单管理</span></el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="feedback">
          <template #title>
            <el-icon><Warning /></el-icon>
            <span>反馈与纠错</span>
          </template>
          <el-menu-item index="/admin/reports"><span>错误报告</span></el-menu-item>
          <el-menu-item index="/admin/feedbacks"><span>意见反馈</span></el-menu-item>
          <el-menu-item index="/admin/comments"><span>评论管理</span></el-menu-item>
          <el-menu-item index="/admin/comment-reports"><span>举报处理</span></el-menu-item>
        </el-sub-menu>
        <el-menu-item index="/admin/ai">
          <el-icon><MagicStick /></el-icon>
          <span>AI 管理</span>
        </el-menu-item>
      </el-menu>
    </aside>

    <!-- Mobile drawer sidebar -->
    <el-drawer
      v-model="drawerOpen"
      direction="ltr"
      size="78%"
      :with-header="false"
      class="admin-drawer"
      append-to-body
    >
      <div class="flex h-full flex-col bg-gray-900 text-white">
        <div class="flex items-center justify-between border-b border-gray-700 px-4 py-3">
          <div class="text-lg font-bold">管理后台</div>
          <el-button text class="!text-gray-300" @click="closeMenu">
            <el-icon :size="18"><Close /></el-icon>
          </el-button>
        </div>
        <el-menu
          :default-active="route.path"
          router
          :unique-opened="true"
          background-color="#111827"
          text-color="#9ca3af"
          active-text-color="#60a5fa"
          class="admin-menu flex-1 overflow-y-auto border-r-0"
          @select="closeMenu"
        >
          <el-menu-item index="/admin">
            <el-icon><DataBoard /></el-icon>
            <span>仪表盘</span>
          </el-menu-item>
          <el-sub-menu index="content-m">
            <template #title>
              <el-icon><Collection /></el-icon>
              <span>内容管理</span>
            </template>
            <el-menu-item index="/admin/questions"><span>题库管理</span></el-menu-item>
            <el-menu-item index="/admin/papers"><span>试卷管理</span></el-menu-item>
            <el-menu-item index="/admin/knowledge"><span>知识点管理</span></el-menu-item>
            <el-menu-item index="/admin/articles"><span>论文管理</span></el-menu-item>
            <el-menu-item index="/admin/essay-submissions"><span>论文投稿审核</span></el-menu-item>
            <el-menu-item index="/admin/announcements"><span>公告管理</span></el-menu-item>
            <el-menu-item index="/admin/exam-schedules"><span>考试排期</span></el-menu-item>
          </el-sub-menu>
          <el-sub-menu index="users-m">
            <template #title>
              <el-icon><User /></el-icon>
              <span>用户管理</span>
            </template>
            <el-menu-item index="/admin/users"><span>用户管理</span></el-menu-item>
            <el-menu-item index="/admin/password-resets"><span>找回密码申请</span></el-menu-item>
            <el-menu-item index="/admin/invite-codes"><span>邀请码管理</span></el-menu-item>
          </el-sub-menu>
          <el-sub-menu index="membership-m">
            <template #title>
              <el-icon><GoldMedal /></el-icon>
              <span>会员与订单</span>
            </template>
            <el-menu-item index="/admin/membership-plans"><span>会员价格</span></el-menu-item>
            <el-menu-item index="/admin/card-keys"><span>卡密管理</span></el-menu-item>
            <el-menu-item index="/admin/orders"><span>订单管理</span></el-menu-item>
          </el-sub-menu>
          <el-sub-menu index="feedback-m">
            <template #title>
              <el-icon><Warning /></el-icon>
              <span>反馈与纠错</span>
            </template>
            <el-menu-item index="/admin/reports"><span>错误报告</span></el-menu-item>
            <el-menu-item index="/admin/feedbacks"><span>意见反馈</span></el-menu-item>
            <el-menu-item index="/admin/comments"><span>评论管理</span></el-menu-item>
            <el-menu-item index="/admin/comment-reports"><span>举报处理</span></el-menu-item>
          </el-sub-menu>
          <el-menu-item index="/admin/ai">
            <el-icon><MagicStick /></el-icon>
            <span>AI 管理</span>
          </el-menu-item>
        </el-menu>
      </div>
    </el-drawer>

    <div class="flex min-w-0 flex-1 flex-col">
      <header class="admin-header sticky top-0 z-30 border-b border-gray-200 bg-white/95 backdrop-blur pt-[env(safe-area-inset-top)] dark:border-gray-700 dark:bg-gray-800/95">
        <div class="flex items-center gap-2 px-3 py-2 sm:px-4 sm:py-3">
          <el-button class="lg:hidden" text @click="openMenu">
            <el-icon :size="20"><List /></el-icon>
          </el-button>
          <div class="min-w-0 flex-1">
            <div class="truncate text-base font-semibold text-gray-900 dark:text-gray-100 sm:text-lg">
              {{ pageTitle }}
            </div>
            <div class="hidden text-xs text-gray-400 sm:block">软考星图管理后台</div>
          </div>
          <div class="hidden items-center gap-2 md:flex">
            <span class="text-sm text-gray-500 dark:text-gray-400">科目</span>
            <button
              class="flex items-center gap-1 rounded-md border border-gray-200 bg-white px-2.5 py-1 text-sm text-gray-700 transition hover:border-blue-200 hover:text-blue-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
              @click="subjectDialogVisible = true"
            >
              <span>{{ subjectStore.currentSubject()?.name || '选择科目' }}</span>
              <el-icon :size="12"><ArrowDown /></el-icon>
            </button>
          </div>
          <el-button class="hidden sm:inline-flex" @click="router.push('/')">返回前台</el-button>
          <el-button class="hidden sm:inline-flex" @click="userStore.logout()">退出</el-button>
          <el-dropdown class="sm:hidden" trigger="click">
            <el-button text>
              <el-icon :size="18"><User /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="router.push('/')">返回前台</el-dropdown-item>
                <el-dropdown-item divided @click="userStore.logout()">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
        <div class="border-t border-gray-100 px-3 py-2 dark:border-gray-700 md:hidden">
          <button
            class="flex w-full items-center justify-between gap-2 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm font-medium text-gray-700 transition hover:border-blue-200 hover:text-blue-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
            @click="subjectDialogVisible = true"
          >
            <span class="flex items-center gap-2">
              <el-icon :size="14"><Reading /></el-icon>
              <span class="text-gray-400 dark:text-gray-500">科目</span>
            </span>
            <span class="flex items-center gap-1 truncate">
              <span class="truncate">{{ subjectStore.currentSubject()?.name || '选择科目' }}</span>
              <el-icon :size="12"><ArrowDown /></el-icon>
            </span>
          </button>
        </div>
      </header>

      <SubjectSelectDialog v-model:visible="subjectDialogVisible" />

      <main class="admin-main min-h-0 flex-1 overflow-auto p-3 sm:p-4 lg:p-6">
        <div class="admin-page mx-auto w-full max-w-[1400px]">
          <router-view />
        </div>
      </main>
    </div>
  </div>
</template>

<style scoped>
.admin-menu {
  border-right: none !important;
}
.admin-menu:not(.el-menu--collapse) {
  width: 100%;
}
</style>
