<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { streamPost } from '@/utils/aiStream'
import { renderMd, stripOuterCodeFence } from '@/utils/markdown'
import { takeEssayPrefill } from '@/utils/essayPrefill'

const router = useRouter()
const topic = ref('')
const points = ref('')
const content = ref('')
const streaming = ref(false)
// 上一轮是否被模型输出上限截断 —— 用来提示用户内容不完整
const truncated = ref(false)
let abort: (() => void) | null = null

// 模型偶尔把整篇范文包进 ``` 围栏，那样会整篇渲染成代码块。
// 渲染与复制都走这里，保证用户拿到的是排版后的正文。
const displayContent = computed(() => stripOuterCodeFence(content.value))

// 从论文详情页/背题模式「一键生成范文」跳过来时，会先在这里放好主题与要点。
// 读取即清除，避免用户下次自己进来还被上次的题面占着。
onMounted(() => {
  const prefill = takeEssayPrefill()
  if (!prefill) return
  topic.value = prefill.topic
  points.value = prefill.points
})

function generate() {
  if (!topic.value.trim() || streaming.value) return
  content.value = ''
  truncated.value = false
  streaming.value = true
  abort = streamPost(
    '/ai/generate-essay',
    { topic: topic.value.trim(), points: points.value.trim() },
    {
      onDelta: (d) => (content.value += d),
      onError: (m) => { ElMessage.error(m); streaming.value = false },
      onDone: (t) => { streaming.value = false; truncated.value = t },
    },
  )
}

function stop() {
  abort?.()
  streaming.value = false
}

async function copy() {
  try {
    await navigator.clipboard.writeText(displayContent.value)
    ElMessage.success('已复制')
  } catch {
    ElMessage.error('复制失败')
  }
}
</script>

<template>
  <div class="mx-auto max-w-3xl">
    <button class="mb-3 flex items-center gap-1 text-sm text-slate-500 transition hover:text-emerald-600 dark:text-slate-400" @click="router.push('/essays')">
      <el-icon><ArrowLeft /></el-icon> 返回论文库
    </button>
    <h2 class="mb-1 text-xl font-bold text-gray-900 dark:text-gray-100">AI 生成论文</h2>
    <p class="mb-4 text-sm text-slate-500 dark:text-slate-400">输入主题，可附上你需要的论点/要点，生成一篇合格范文骨架供参考（约 2000-2500 字）</p>

    <div class="mb-6 rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <label class="mb-1 block text-sm text-slate-600 dark:text-slate-300">论文主题</label>
      <input v-model="topic" placeholder="如：论微服务架构设计及其应用" class="w-full rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm focus:border-emerald-400 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100" @keyup.enter="generate" />

      <label class="mb-1 mt-3 block text-sm text-slate-600 dark:text-slate-300">
        论点 / 要点
        <span class="text-slate-400 dark:text-slate-500">（选填，填了会要求 AI 逐一论述，不遗漏）</span>
      </label>
      <textarea v-model="points" rows="4" placeholder="每行一条，如：&#10;1. 微服务的拆分粒度与领域边界划分&#10;2. 服务治理：注册发现、熔断限流&#10;3. 数据一致性：分布式事务方案" class="w-full resize-y rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm leading-relaxed focus:border-emerald-400 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100" @keydown.ctrl.enter="generate" />

      <div class="mt-3 flex justify-end">
        <button v-if="!streaming" class="rounded-lg bg-emerald-600 px-5 py-2 text-sm font-medium text-white transition hover:bg-emerald-700 disabled:opacity-50" :disabled="!topic.trim()" @click="generate">生成范文</button>
        <button v-else class="rounded-lg border border-slate-300 px-5 py-2 text-sm dark:border-slate-600" @click="stop">停止</button>
      </div>
    </div>

    <div v-if="content" class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <div class="mb-3 flex justify-end">
        <button class="rounded-lg border border-slate-200 px-3 py-1 text-sm text-slate-600 transition hover:border-emerald-300 hover:text-emerald-600 dark:border-slate-700 dark:text-slate-300" @click="copy">复制全文</button>
      </div>
      <p v-if="truncated && !streaming" class="mb-3 rounded-lg border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-300">
        范文未输出完整：已达模型单次输出上限，最后一段可能被截断。可减少论点后重新生成，或请管理员调高输出上限。
      </p>
      <div class="essay-prose text-slate-700 dark:text-slate-300" v-html="renderMd(displayContent)" />
      <div v-if="streaming" class="mt-2 text-sm text-slate-400">生成中…</div>
    </div>
    <div v-else-if="streaming" class="py-12 text-center text-sm text-slate-400">生成中…</div>
  </div>
</template>
