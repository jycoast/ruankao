<script setup lang="ts">
import { computed, ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useUserStore } from "@/stores/user";
import { useSubjectStore } from "@/stores/subject";
import { useExamCountdown } from "@/composables/useExamCountdown";
import { practiceApi, userApi } from "@/api";
import { SPRINT_POINTS } from "@/data/sprintPoints";
import type { ChipGroup } from "@/data/sprintPoints";

const router = useRouter();
const userStore = useUserStore();
const subjectStore = useSubjectStore();
const { countdown } = useExamCountdown();

const stats = ref<any>({});
const progress = ref<any[]>([]);
const dueCount = ref(0);
const wrongCount = ref(0);
const loading = ref(true);
const firstLoaded = ref(false);

async function loadData() {
  loading.value = true;
  try {
    if (userStore.isLoggedIn) {
      // 四个接口互不依赖，必须并发发出。原来逐个 await 会让首屏耗时变成
      // 四次往返之和，而首页最慢的一步本就受网络与 DB 影响。
      // 用 allSettled 而非 all：任一接口失败（限流/维护/后端抖动）不能让
      // 其余三块数据一起消失，保持原先四个独立 try/catch 的降级行为。
      const [statsRes, reviewRes, wrongRes, progressRes] =
        await Promise.allSettled([
          userApi.getStats(),
          practiceApi.getDueReview({ silent: true, page: 1, page_size: 1 }),
          practiceApi.getWrongQuestions({ status: 1, page: 1, page_size: 1 }),
          practiceApi.getCategoryProgress(),
        ]);
      if (statsRes.status === "fulfilled")
        stats.value = (statsRes.value as any).data || {};
      if (reviewRes.status === "fulfilled")
        dueCount.value = Number((reviewRes.value as any).data?.total || 0);
      if (wrongRes.status === "fulfilled")
        wrongCount.value = Number((wrongRes.value as any).data?.total || 0);
      if (progressRes.status === "fulfilled")
        progress.value = (progressRes.value as any).data || [];
    }
  } finally {
    loading.value = false;
    firstLoaded.value = true;
  }
}

onMounted(loadData);
// 以上四个接口都不按科目过滤（user/stats、practice/review、practice/wrong、
// practice/category-progress 均无 subject 参数），切换科目重刷只会拿到同一份数据。
// 原先 watch(currentSubjectId) 因此是纯浪费，还会在科目晚于首页挂载恢复时再全量刷一遍。

const greeting = computed(() => {
  const h = new Date().getHours();
  if (h < 6) return "夜深了";
  if (h < 12) return "早上好";
  if (h < 14) return "中午好";
  if (h < 18) return "下午好";
  return "晚上好";
});

const displayName = computed(
  () => userStore.user?.nickname || userStore.user?.username || "同学",
);

// 正确率由后端按统一口径下发（每题只算最近一次、只算客观题）。
// 前端不再用 total_correct/total_practice 自己算 —— 那是累加值，
// 反复刷同一题能刷到接近 100%，与学习报告页对不上。
const accuracy = computed(() => Number(stats.value.accuracy || 0));

// 学习时长自适应展示（total_time 单位为秒）
const studyTime = computed(() => {
  const sec = Number(stats.value.total_time || 0);
  if (sec >= 3600) return { value: (sec / 3600).toFixed(1), suffix: "小时" };
  return { value: Math.round(sec / 60), suffix: "分钟" };
});

const statCards = computed(() => [
  {
    label: "总练习",
    value: stats.value.total_practice || 0,
    suffix: "题",
    icon: "EditPen",
    color: "text-blue-600",
    bg: "bg-blue-50 dark:bg-blue-500/15",
  },
  {
    label: "正确率",
    value: accuracy.value,
    suffix: "%",
    icon: "TrendCharts",
    color: "text-emerald-600",
    bg: "bg-emerald-50 dark:bg-emerald-500/15",
  },
  {
    label: "学习时长",
    value: studyTime.value.value,
    suffix: studyTime.value.suffix,
    icon: "Clock",
    color: "text-orange-600",
    bg: "bg-orange-50 dark:bg-orange-500/15",
  },
  {
    label: "连续学习",
    value: stats.value.streak_days || 0,
    suffix: "天",
    icon: "Calendar",
    color: "text-blue-600",
    bg: "bg-blue-50 dark:bg-blue-500/15",
  },
]);

// 今日学习任务清单
const tasks = computed(() => [
  {
    key: "review",
    label: "今日待复习",
    desc:
      dueCount.value > 0
        ? `有 ${dueCount.value} 道题到期，按遗忘曲线复习`
        : "暂无到期复习，继续保持",
    count: dueCount.value,
    done: dueCount.value === 0,
    path: "/review",
    action: "去复习",
    icon: "Reading",
    tone: "text-orange-600 bg-orange-50 dark:bg-orange-500/15",
  },
  {
    key: "wrong",
    label: "错题待攻克",
    desc:
      wrongCount.value > 0
        ? `错题本累计 ${wrongCount.value} 道，逐个消灭`
        : "错题本已清空，很棒",
    count: wrongCount.value,
    done: wrongCount.value === 0,
    path: "/wrong",
    action: "去攻克",
    icon: "Warning",
    tone: "text-rose-600 bg-rose-50 dark:bg-rose-500/15",
  },
  {
    key: "practice",
    label: "分类刷题",
    desc: "按知识点推进，稳步提升覆盖率",
    count: null,
    done: false,
    path: "/practice",
    action: "去刷题",
    icon: "EditPen",
    tone: "text-blue-600 bg-blue-50 dark:bg-blue-500/15",
  },
]);

// 知识点掌握度：优先展示已练过的，按覆盖率排序
const progressBars = computed(() => {
  return [...progress.value]
    .map((p: any) => {
      const total = Number(p.total || 0);
      const done = Number(p.done || 0);
      const correct = Number(p.correct || 0);
      return {
        name: p.knowledge_point,
        total,
        done,
        coverage: total ? Math.round((done / total) * 100) : 0,
        accuracy: done ? Math.round((correct / done) * 100) : 0,
      };
    })
    .sort((a, b) => b.done - a.done || b.total - a.total)
    .slice(0, 6);
});

function barColor(pct: number) {
  if (pct >= 80) return "bg-emerald-500";
  if (pct >= 40) return "bg-blue-500";
  return "bg-orange-400";
}

function scrollToDaily() {
  document
    .getElementById("daily")
    ?.scrollIntoView({ behavior: "smooth", block: "start" });
}

const allResourceLinks = [
  { path: "/exams", label: "真题试卷", icon: "Tickets" },
  { path: "/report", label: "学习报告", icon: "TrendCharts" },
  { path: "/sprint", label: "考前冲刺", icon: "Timer" },
  { path: "/knowledge", label: "知识点库", icon: "Collection" },
  { path: "/essays", label: "论文库", icon: "Document" },
  { path: "/wrong", label: "错题本", icon: "Warning" },
  { path: "/my/diagnosis", label: "AI 诊断报告", icon: "Aim" },
];
const resourceLinks = computed(() =>
  allResourceLinks.filter((it) => {
    if (it.path === "/sprint") return subjectStore.hasSprint;
    if (it.path === "/essays") return subjectStore.hasEssay;
    return true;
  })
);

// 首页「必考速记」卡片：数据统一收在 sprintPoints.ts（冲刺必考速记同源），
// 带 homeChips 的冲刺章节才有首页卡片形态 —— 改考点只动 sprintPoints.ts 一处。
const showcaseTopics: { topic: string; groups: ChipGroup[] }[] = [];
for (const g of SPRINT_POINTS) {
  if (g.homeChips?.length) showcaseTopics.push({ topic: g.chapter, groups: g.homeChips });
}

const noteIndex = ref(0);
const currentNote = computed(() => showcaseTopics[noteIndex.value] ?? showcaseTopics[0] ?? { topic: "", groups: [] });

function shuffleNote() {
  if (showcaseTopics.length <= 1) return;
  let next = noteIndex.value;
  while (next === noteIndex.value) {
    next = Math.floor(Math.random() * showcaseTopics.length);
  }
  noteIndex.value = next;
}
</script>

<template>
  <div class="space-y-6">
    <!-- 焦点区：问候 + 主次 CTA + 学习指标条（呼应登录页深蓝面板） -->
    <section class="grid gap-6 lg:grid-cols-[1fr_340px]">
      <div
        class="anim-item relative overflow-hidden rounded-2xl bg-gradient-to-br from-blue-600 via-blue-700 to-indigo-900 p-6 text-white shadow-sm lg:p-7"
      >
        <!-- 网格纹路 -->
        <div
          class="pointer-events-none absolute inset-0 [background-image:linear-gradient(rgba(255,255,255,.05)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,.05)_1px,transparent_1px)] [background-size:26px_26px]"
        />
        <div class="relative z-10">
          <h1 class="text-xl font-bold lg:text-2xl">
            {{ greeting }}，{{ displayName }}
          </h1>
          <p class="mt-1.5 text-sm text-blue-100">
            今天也从一次正确作答开始，稳步向通过线靠近
          </p>
          <div class="mt-5 flex flex-wrap gap-2.5">
            <button
              class="rounded-lg bg-white px-4 py-2 text-sm font-medium text-blue-700 shadow-sm transition hover:bg-blue-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white"
              @click="router.push('/practice')"
            >
              继续刷题
            </button>
            <button
              class="rounded-lg bg-white/10 px-4 py-2 text-sm font-medium text-white ring-1 ring-white/40 transition hover:bg-white/20 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white"
              @click="router.push('/review')"
            >
              今日复习{{ dueCount > 0 ? ` ${dueCount}` : "" }}
            </button>
            <button
              class="rounded-lg bg-white/10 px-4 py-2 text-sm font-medium text-white ring-1 ring-white/40 transition hover:bg-white/20 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white"
              @click="scrollToDaily"
            >
              每日一练
            </button>
          </div>
          <!-- 学习指标条（原 4 张统计卡合并入 hero，消除数据重复展示） -->
          <div
            class="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-4"
            aria-label="学习数据概览"
          >
            <template v-if="!firstLoaded">
              <div
                v-for="i in 4"
                :key="'stat-sk-' + i"
                class="rounded-lg bg-white/10 px-3.5 py-2.5 ring-1 ring-white/15"
              >
                <el-skeleton animated :rows="1" />
              </div>
            </template>
            <template v-else>
              <div
                v-for="card in statCards"
                :key="card.label"
                class="rounded-lg bg-white/10 px-3.5 py-2.5 ring-1 ring-white/15 backdrop-blur"
              >
                <div class="text-xs text-blue-100">{{ card.label }}</div>
                <div class="mt-0.5 text-xl font-semibold">
                  {{ card.value
                  }}<span class="ml-0.5 text-xs font-normal text-blue-200">{{
                    card.suffix
                  }}</span>
                </div>
              </div>
            </template>
          </div>
        </div>
        <!-- 装饰性插画 -->
        <el-icon
          class="pointer-events-none absolute -right-4 -bottom-4 text-white/10"
          :size="150"
          ><Reading
        /></el-icon>
      </div>

      <div
        class="anim-item flex cursor-pointer flex-col justify-center rounded-2xl border border-orange-200 bg-orange-50 p-5 text-center transition hover:border-orange-300 hover:bg-orange-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-orange-500 dark:border-orange-500/30 dark:bg-orange-500/10 dark:hover:bg-orange-500/20"
        role="button"
        tabindex="0"
        aria-label="距考试倒计时，点击进入考前冲刺"
        @click="router.push('/sprint')"
        @keydown.enter.prevent="router.push('/sprint')"
        @keydown.space.prevent="router.push('/sprint')"
      >
        <div class="text-xs font-medium text-orange-600 dark:text-orange-300">
          距 {{ countdown.label }}
        </div>
        <div
          class="mt-1 flex items-end justify-center gap-1 text-orange-600 dark:text-orange-300"
        >
          <span class="text-5xl font-bold leading-none">{{
            countdown.days
          }}</span>
          <span class="mb-1 text-sm font-medium">天</span>
        </div>
        <div
          class="mt-2 inline-flex items-center justify-center gap-1 text-xs text-orange-500/90 dark:text-orange-300/80"
        >
          <el-icon :size="13"><Calendar /></el-icon>
          <span>{{ countdown.date || "暂无排期" }}</span>
        </div>
        <div
          class="mt-3 flex items-center justify-center gap-1.5 rounded-lg bg-white/70 py-1.5 text-xs font-medium text-orange-700 dark:bg-slate-900/30 dark:text-orange-300"
        >
          <el-icon :size="13"><Histogram /></el-icon>
          <span>已连续学习 {{ stats.streak_days || 0 }} 天</span>
        </div>
      </div>
    </section>

    <!-- 每日一练入口卡片 -->
    <section
      id="daily"
      class="anim-item flex flex-col gap-4 rounded-xl border border-orange-200 bg-gradient-to-br from-orange-50 to-amber-50 p-5 shadow-sm sm:flex-row sm:items-center sm:justify-between dark:border-orange-500/30 dark:from-orange-500/10 dark:to-amber-500/10"
      style="animation-delay: 90ms"
    >
      <div class="flex items-center gap-4">
        <div
          class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-orange-500 text-white shadow-sm"
        >
          <el-icon :size="24"><Calendar /></el-icon>
        </div>
        <div>
          <h2 class="font-semibold text-slate-950 dark:text-white">每日一练</h2>
          <p class="mt-0.5 text-sm text-slate-500 dark:text-slate-400">
            每天精选题，坚持打卡稳步提升 · 已连续学习
            <span class="font-semibold text-orange-600 dark:text-orange-300">{{
              stats.streak_days || 0
            }}</span>
            天
          </p>
        </div>
      </div>
      <button
        class="shrink-0 rounded-lg bg-orange-500 px-5 py-2.5 text-sm font-medium text-white shadow-sm transition hover:bg-orange-600"
        @click="router.push('/daily')"
      >
        去打卡
      </button>
    </section>

    <!-- 4 sections 直接作为 2 列 grid 子元素，同行自动等高 -->
    <div class="grid gap-6 lg:grid-cols-[1fr_340px]">
      <!-- 今日学习任务 (row 1, col 1) -->
      <section
        class="anim-item rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"
        style="animation-delay: 150ms"
      >
        <div
          class="border-b border-slate-100 px-5 py-4 dark:border-slate-800"
        >
          <h2 class="font-semibold text-slate-950 dark:text-white">
            今日学习任务
          </h2>
          <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
            完成这些，稳步向通过率靠近
          </p>
        </div>
        <div
          v-if="!firstLoaded"
          class="divide-y divide-slate-100 dark:divide-slate-800"
        >
          <div v-for="i in 3" :key="'task-sk-' + i" class="px-5 py-4">
            <el-skeleton animated :rows="1" />
          </div>
        </div>
        <div v-else class="divide-y divide-slate-100 dark:divide-slate-800">
          <div
            v-for="task in tasks"
            :key="task.key"
            class="flex items-center gap-3 px-5 py-4"
          >
            <div
              class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg"
              :class="task.tone"
            >
              <el-icon :size="18"
                ><component :is="task.done ? 'CircleCheck' : task.icon"
              /></el-icon>
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span class="font-medium text-slate-900 dark:text-white">{{
                  task.label
                }}</span>
                <span
                  v-if="task.count !== null && task.count > 0"
                  class="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-600 dark:bg-slate-800 dark:text-slate-300"
                  >{{ task.count }}</span
                >
              </div>
              <div
                class="mt-0.5 truncate text-sm text-slate-500 dark:text-slate-400"
              >
                {{ task.desc }}
              </div>
            </div>
            <button
              class="shrink-0 rounded-lg border border-slate-200 px-3 py-1.5 text-sm font-medium text-slate-700 transition hover:border-blue-300 hover:bg-blue-50 hover:text-blue-700 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-blue-500/15 dark:hover:text-blue-300"
              @click="router.push(task.path)"
            >
              {{ task.action }}
            </button>
          </div>
        </div>
      </section>

      <!-- 资料入口 (row 1, col 2) -->
      <section
        class="anim-item rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"
        style="animation-delay: 210ms"
      >
        <div
          class="border-b border-slate-100 px-5 py-4 dark:border-slate-800"
        >
          <h2 class="font-semibold text-slate-950 dark:text-white">
            资料入口
          </h2>
        </div>
        <div class="grid grid-cols-2 gap-2 px-5 py-4">
          <router-link
            v-for="item in resourceLinks"
            :key="item.path"
            :to="item.path"
            class="flex items-center gap-2 rounded-lg border border-slate-100 px-3 py-2.5 text-sm text-slate-700 transition hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700 dark:border-slate-800 dark:text-slate-300 dark:hover:bg-blue-500/15 dark:hover:text-blue-300"
          >
            <el-icon><component :is="item.icon" /></el-icon>
            {{ item.label }}
          </router-link>
        </div>
      </section>

      <!-- 学习概况 (row 2, col 1) -->
      <section
        class="anim-item rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"
        style="animation-delay: 270ms"
      >
        <div
          class="flex items-center justify-between border-b border-slate-100 px-5 py-4 dark:border-slate-800"
        >
          <div>
            <h2 class="font-semibold text-slate-950 dark:text-white">
              学习概况
            </h2>
            <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
              按刷题覆盖率排序
            </p>
          </div>
          <router-link
            to="/practice"
            class="text-sm font-medium text-blue-600 hover:text-blue-700 dark:text-blue-300"
            >分类刷题</router-link
          >
        </div>
        <div v-if="!firstLoaded" class="space-y-5 px-5 py-4">
          <div v-for="i in 5" :key="'kp-sk-' + i">
            <el-skeleton animated :rows="1" />
          </div>
        </div>
        <div v-else-if="progressBars.length" class="space-y-4 px-5 py-4">
          <div v-for="bar in progressBars" :key="bar.name">
            <div class="flex items-center justify-between text-sm">
              <span
                class="truncate font-medium text-slate-700 dark:text-slate-200"
                >{{ bar.name }}</span
              >
              <span class="ml-2 shrink-0 text-xs text-slate-400">
                {{ bar.done }}/{{ bar.total }}
                <span
                  v-if="bar.done"
                  class="ml-1 text-emerald-600 dark:text-emerald-400"
                  >· 正确率 {{ bar.accuracy }}%</span
                >
              </span>
            </div>
            <div
              class="mt-1.5 h-2 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800"
            >
              <div
                class="h-full rounded-full transition-all"
                :class="barColor(bar.coverage)"
                :style="{ width: bar.coverage + '%' }"
              />
            </div>
          </div>
        </div>
        <div v-else class="px-5 py-12 text-center text-sm text-slate-500">
          还没有刷题记录，去<router-link
            to="/practice"
            class="text-blue-600 hover:underline"
            >分类刷题</router-link
          >开始吧
        </div>
      </section>

      <!-- 必考速记 (row 2, col 2) —— 架构设计师等含冲刺资料的科目才显示 -->
      <section
        v-if="subjectStore.hasSprint"
        class="anim-item rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900"
        style="animation-delay: 330ms"
      >
        <div
          class="flex items-center justify-between border-b border-slate-100 px-5 py-4 dark:border-slate-800"
        >
          <div class="flex items-center gap-2">
            <el-icon class="text-blue-600 dark:text-blue-400" :size="16"
              ><Aim
            /></el-icon>
            <h2 class="font-semibold text-slate-950 dark:text-white">
              必考速记
            </h2>
          </div>
          <button
            class="rounded-md px-2 py-1 text-xs font-medium text-blue-600 transition hover:bg-blue-50 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-blue-500 dark:text-blue-400 dark:hover:bg-blue-500/10"
            @click="shuffleNote"
          >
            换一组
          </button>
        </div>
        <div class="space-y-3 px-5 py-4">
          <div class="mb-1 text-sm font-semibold text-slate-900 dark:text-white">
            {{ currentNote.topic }}
          </div>
          <div
            v-for="group in currentNote.groups"
            :key="group.label"
            class="space-y-1.5"
          >
            <div class="text-xs font-medium text-blue-600 dark:text-blue-400">
              {{ group.label }}
            </div>
            <div class="flex flex-wrap gap-1.5">
              <span
                v-for="point in group.points"
                :key="point"
                class="rounded-md bg-slate-50 px-2 py-0.5 text-xs text-slate-700 ring-1 ring-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:ring-slate-700"
              >
                {{ point }}
              </span>
            </div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
/* 入场动效：各区块淡入上移，animation-delay 由各 section 内联指定形成 stagger。
   纯 CSS 实现（skill 动效档位 Subtle：300-400ms、小位移，读作淡入而非滑动），
   尊重 prefers-reduced-motion —— 关闭时立即渲染最终状态。 */
@keyframes home-fade-in-up {
  from {
    opacity: 0;
    transform: translateY(12px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
.anim-item {
  animation: home-fade-in-up 0.4s ease both;
}

@media (prefers-reduced-motion: reduce) {
  .anim-item {
    animation: none;
  }
}
</style>
