<script setup lang="ts">
import { onMounted, computed, ref } from "vue";
import { useUserStore } from "@/stores/user";
import { useSubjectStore } from "@/stores/subject";
import { useAppearance } from "@/stores/appearance";
import { useExamCountdown } from "@/composables/useExamCountdown";
import { useRoute, useRouter } from "vue-router";
import AnnouncementBell from "@/components/AnnouncementBell.vue";
import NotificationBell from "@/components/NotificationBell.vue";
import AIAssistant from "@/components/AIAssistant.vue";
import MembershipExpiryBanner from "@/components/MembershipExpiryBanner.vue";
import SubjectSelectDialog from "@/components/SubjectSelectDialog.vue";

const userStore = useUserStore();
const subjectStore = useSubjectStore();
const appearance = useAppearance();
const { countdown } = useExamCountdown();
const router = useRouter();

const route = useRoute();

const subjectDialogVisible = ref(false);

onMounted(() => {
  subjectStore.loadSubjects();
  appearance.init();
});

// 主导航：核心学习入口
const allNavItems = [
  { path: "/exams", label: "真题", icon: "Tickets" },
  { path: "/practice", label: "刷题", icon: "EditPen" },
  { path: "/questions", label: "搜题", icon: "Search" },
  { path: "/knowledge", label: "知识导图", icon: "Collection" },
  { path: "/essays", label: "论文", icon: "Document" },
  { path: "/sprint", label: "冲刺", icon: "Timer", accent: "orange" },
];

// 论文 / 冲刺仅对相应科目展示（见 subject store 的 hasEssay / hasSprint）。
const navItems = computed(() =>
  allNavItems.filter((item) => {
    if (item.path === "/essays") return subjectStore.hasEssay
    if (item.path === "/sprint") return subjectStore.hasSprint
    return true
  })
);

// 游客态：无个人中心;导航=只读内容+冲刺(按科目能力);「我的」工具页对游客放开(点开为空态)
const isGuest = computed(() => userStore.isGuest);
const guestNavItems = computed(() =>
  allNavItems.filter((item) => {
    if (["/exams", "/questions", "/knowledge"].includes(item.path)) return true;
    if (item.path === "/essays") return subjectStore.hasEssay;
    if (item.path === "/sprint") return subjectStore.hasSprint;
    return false;
  }),
);
const navForDisplay = computed(() =>
  isGuest.value ? guestNavItems.value : navItems.value,
);

// 「我的」下拉：个人学习工具
const myItems = [
  { path: "/report", label: "学习报告", icon: "TrendCharts" },
  { path: "/wrong", label: "错题本", icon: "Warning" },
  { path: "/review", label: "Anki 背题", icon: "Reading" },
  { path: "/favorites", label: "收藏夹", icon: "Star" },
  { path: "/my/notes", label: "笔记", icon: "EditPen" },
  { path: "/ai/my-config", label: "AI 配置", icon: "MagicStick" },
  { path: "/my/diagnosis", label: "AI 诊断报告", icon: "Aim" },
  { path: "/feedback", label: "意见反馈", icon: "ChatLineSquare" },
];

const mobileItems = computed(() => {
  if (isGuest.value) {
    return [
      ...guestNavItems.value.map(({ path, label, icon }) => ({ path, label, icon })),
      ...myItems.map(({ path, label, icon }) => ({ path, label, icon })),
    ];
  }
  return [
    { path: "/", label: "首页", icon: "HomeFilled", exact: true },
    ...navItems.value,
    ...myItems.map(({ path, label, icon }) => ({ path, label, icon })),
  ];
});

function isActive(item: { path: string; exact?: boolean }) {
  if (item.exact) return route.path === item.path;
  return route.path === item.path || route.path.startsWith(`${item.path}/`);
}

// 导航项样式：普通项蓝色，accent 项（冲刺=橙 / AI=紫）高亮
function navClass(item: { path: string; exact?: boolean; accent?: string }) {
  const active = isActive(item);
  if (item.accent === "violet") {
    return active
      ? "bg-violet-600 text-white shadow-sm"
      : "text-violet-600 hover:bg-violet-50 dark:text-violet-300 dark:hover:bg-violet-500/15";
  }
  if (item.accent === "orange") {
    return active
      ? "bg-orange-500 text-white shadow-sm"
      : "text-orange-600 hover:bg-orange-50 dark:text-orange-300 dark:hover:bg-orange-500/15";
  }
  return active
    ? "bg-blue-50 text-blue-700 dark:bg-blue-500/15 dark:text-blue-300"
    : "text-slate-500 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white";
}

const myActive = () => myItems.some((i) => isActive(i));
</script>

<template>
  <div
    class="min-h-screen bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-100 transition-colors"
  >
    <header
      class="sticky top-0 z-50 border-b border-slate-200/80 bg-white/95 backdrop-blur pt-[env(safe-area-inset-top)] dark:border-slate-800 dark:bg-slate-900/95"
    >
      <div class="mx-auto max-w-screen-2xl px-4 sm:px-6 lg:px-8 lg:w-[86%]">
        <div class="flex h-14 items-center justify-between gap-3 min-w-0">
          <div class="flex min-w-0 items-center gap-4">
            <button
              class="flex items-center gap-2 text-left"
              @click="router.push('/')"
            >
              <img
                src="/brand-mark.png"
                alt="软考星图"
                class="h-8 w-8 rounded-lg shadow-sm"
              />
              <span class="hidden sm:flex flex-col leading-tight">
                <span
                  class="text-sm font-semibold text-slate-900 dark:text-white"
                  >软考星图</span
                >
              </span>
            </button>

            <div
              class="hidden h-5 w-px bg-slate-200 dark:bg-slate-700 xl:block"
            />

            <nav class="hidden min-w-0 items-center gap-1 overflow-hidden xl:flex">
              <router-link
                v-for="item in navForDisplay"
                :key="item.path"
                :to="item.path"
                class="flex shrink-0 items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium transition"
                :class="navClass(item)"
              >
                <el-icon :size="15"><component :is="item.icon" /></el-icon>
                <span>{{ item.label }}</span>
              </router-link>

              <el-dropdown trigger="hover" placement="bottom-start">
                <button
                  class="flex shrink-0 items-center gap-1 rounded-lg px-3 py-1.5 text-sm font-medium transition"
                  :class="
                    myActive()
                      ? 'bg-blue-50 text-blue-700 dark:bg-blue-500/15 dark:text-blue-300'
                      : 'text-slate-500 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white'
                  "
                >
                  <el-icon :size="15"><Files /></el-icon>
                  <span>我的</span>
                  <el-icon :size="12"><ArrowDown /></el-icon>
                </button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item
                      v-for="item in myItems"
                      :key="item.path"
                      @click="router.push(item.path)"
                    >
                      <el-icon><component :is="item.icon" /></el-icon>
                      <span class="ml-1">{{ item.label }}</span>
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </nav>
          </div>

          <div class="flex items-center gap-2">
            <!-- 距考倒计时冲刺 chip -->
            <button
              v-if="countdown.days > 0"
              class="hidden shrink-0 items-center gap-1.5 rounded-full border border-orange-200 bg-orange-50 px-2.5 py-1 text-xs font-medium text-orange-700 transition hover:bg-orange-100 lg:flex dark:border-orange-500/30 dark:bg-orange-500/10 dark:text-orange-300"
              @click="router.push('/sprint')"
            >
              <el-icon :size="13"><Timer /></el-icon>
              <span>距考 {{ countdown.days }} 天</span>
            </button>

            <!-- 响应式 class 必须挂在 div 上：element-plus 的 .el-select{display:inline-block}
                 与 tailwind 的 .hidden 同特异性，且其样式表是后注入的懒加载 chunk，
                 直接写在 el-select 上会被覆盖，桌面/移动两个下拉会同时出现。 -->
            <div class="hidden sm:block">
              <button
                class="flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm font-medium text-slate-500 transition hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white"
                @click="subjectDialogVisible = true"
              >
                <span>{{ subjectStore.currentSubject()?.name || "选择科目" }}</span>
                <el-icon :size="12"><ArrowDown /></el-icon>
              </button>
            </div>

            <AnnouncementBell />
            <NotificationBell />

            <template v-if="isGuest">
              <span
                class="flex h-6 items-center rounded-full bg-slate-100 px-2 text-xs text-slate-500 dark:bg-slate-800 dark:text-slate-400"
                >游客</span
              >
              <el-button size="small" type="primary" plain @click="router.push('/login')"
                >去登录</el-button
              >
            </template>
            <el-dropdown v-else-if="userStore.isLoggedIn">
              <button
                class="flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs text-slate-700 transition hover:border-blue-200 hover:text-blue-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
              >
                <span
                  class="flex h-6 w-6 items-center justify-center rounded-full bg-slate-900 text-[10px] font-medium text-white dark:bg-blue-600"
                >
                  {{
                    (userStore.user?.nickname ||
                      userStore.user?.username ||
                      "U")[0]
                  }}
                </span>
                <span class="hidden max-w-20 truncate lg:inline">{{
                  userStore.user?.nickname || userStore.user?.username
                }}</span>
                <el-icon :size="12"><ArrowDown /></el-icon>
              </button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="router.push('/profile')"
                    >个人中心</el-dropdown-item
                  >
                  <el-dropdown-item @click="router.push('/membership')">{{
                    userStore.isMember ? "我的会员" : "开通会员"
                  }}</el-dropdown-item>
                  <el-dropdown-item @click="router.push('/membership/orders')"
                    >我的订单</el-dropdown-item
                  >
                  <el-dropdown-item divided @click="router.push('/my/notifications')"
                    >我的消息</el-dropdown-item
                  >
                  <el-dropdown-item @click="router.push('/my/reports')"
                    >我的纠错</el-dropdown-item
                  >
                  <el-dropdown-item @click="router.push('/my/feedbacks')"
                    >我的反馈</el-dropdown-item
                  >
                  <el-dropdown-item @click="router.push('/essays/my-submissions')"
                    >我的投稿</el-dropdown-item
                  >
                  <el-dropdown-item
                    v-if="userStore.user?.role === 1"
                    @click="router.push('/admin')"
                    >后台管理</el-dropdown-item
                  >
                  <el-dropdown-item divided @click="userStore.logout()"
                    >退出登录</el-dropdown-item
                  >
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <div v-else class="flex items-center gap-1.5">
              <el-button size="small" @click="router.push('/login')"
                >登录</el-button
              >
              <el-button
                size="small"
                type="primary"
                plain
                @click="router.push('/register')"
                >注册</el-button
              >
            </div>
          </div>
        </div>

        <div class="mb-1 block sm:hidden">
          <button
            class="flex w-full items-center justify-between gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-700 transition hover:border-blue-200 hover:text-blue-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:border-blue-500/40"
            @click="subjectDialogVisible = true"
          >
            <span class="flex items-center gap-2">
              <el-icon :size="14"><Reading /></el-icon>
              <span class="text-slate-400 dark:text-slate-500">科目</span>
            </span>
            <span class="flex items-center gap-1 truncate">
              <span class="truncate">{{ subjectStore.currentSubject()?.name || "选择科目" }}</span>
              <el-icon :size="12"><ArrowDown /></el-icon>
            </span>
          </button>
        </div>

        <nav class="no-scrollbar flex gap-1 overflow-x-auto pb-2 xl:hidden">
          <router-link
            v-for="item in mobileItems"
            :key="item.path"
            :to="item.path"
            class="flex shrink-0 items-center gap-1 rounded-md px-2 py-1 text-xs font-medium transition"
            :class="navClass(item)"
          >
            <el-icon :size="14"><component :is="item.icon" /></el-icon>
            <span>{{ item.label }}</span>
          </router-link>
        </nav>
      </div>
    </header>

    <div
      v-if="isGuest"
      class="border-b border-blue-100 bg-blue-50 px-4 py-1.5 text-center text-xs text-blue-700 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-300"
    >
      游客模式：可浏览真题、搜题、知识导图、论文与冲刺，可查看学习报告、错题本、收藏与笔记；登录后才能刷题、作答与提交。
    </div>

    <MembershipExpiryBanner v-if="!isGuest" />

    <main class="mx-auto w-full max-w-screen-2xl px-4 py-6 sm:px-6 lg:px-8 lg:w-[86%]">
      <router-view />
    </main>

    <AIAssistant v-if="!isGuest" />
    <SubjectSelectDialog v-model:visible="subjectDialogVisible" />
  </div>
</template>
