<script setup lang="ts">
import { ref, nextTick, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { aiApi } from '@/api'
import { streamPost } from '@/utils/aiStream'
import { renderMd } from '@/utils/markdown'
import { useAI } from '@/composables/useAI'
import { useUserStore } from '@/stores/user'

const { aiEnabled, ensureAIStatus } = useAI()
const userStore = useUserStore()

const open = ref(false)
const input = ref('')
const streaming = ref(false)
const messages = ref<{ role: string; content: string }[]>([])
const convId = ref<number | null>(Number(localStorage.getItem('ai_assistant_conv')) || null)
const scrollRef = ref<HTMLElement | null>(null)
let abort: (() => void) | null = null

// 首次登录后探测 AI 是否可用
watch(
  () => userStore.isLoggedIn,
  (v) => { if (v) ensureAIStatus() },
  { immediate: true },
)

async function toggle() {
  open.value = !open.value
  if (open.value && convId.value) {
    try {
      const res: any = await aiApi.getConversation(convId.value)
      messages.value = (res.data.messages || []).map((m: any) => ({ role: m.role, content: m.content }))
      scrollToBottom()
    } catch {
      // 会话可能已失效，重置
      convId.value = null
      localStorage.removeItem('ai_assistant_conv')
    }
  }
}

function scrollToBottom() {
  nextTick(() => {
    const el = scrollRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

async function ensureConversation(): Promise<number | null> {
  if (convId.value) return convId.value
  try {
    const res: any = await aiApi.createConversation()
    convId.value = res.data.id
    localStorage.setItem('ai_assistant_conv', String(res.data.id))
    return convId.value
  } catch {
    return null
  }
}

function newChat() {
  convId.value = null
  localStorage.removeItem('ai_assistant_conv')
  messages.value = []
}

async function send() {
  const text = input.value.trim()
  if (!text || streaming.value) return
  const id = await ensureConversation()
  if (!id) return

  messages.value.push({ role: 'user', content: text })
  messages.value.push({ role: 'assistant', content: '' })
  input.value = ''
  streaming.value = true
  scrollToBottom()

  const assistant = messages.value[messages.value.length - 1]
  abort = streamPost(
    '/ai/chat',
    { conversation_id: id, message: text },
    {
      onDelta: (d) => { assistant.content += d; scrollToBottom() },
      onError: (m) => { assistant.content = assistant.content || `⚠️ ${m}`; ElMessage.error(m); streaming.value = false },
      onDone: () => { streaming.value = false },
    },
  )
}

function stop() {
  abort?.()
  streaming.value = false
}
</script>

<template>
  <div v-if="userStore.isLoggedIn && aiEnabled">
    <!-- 悬浮按钮 -->
    <button
      v-if="!open"
      class="fixed bottom-[calc(1.25rem+env(safe-area-inset-bottom))] right-5 z-40 flex items-center gap-1.5 rounded-full bg-gradient-to-br from-violet-600 to-indigo-500 px-4 py-3 text-sm font-medium text-white shadow-lg transition hover:shadow-xl"
      @click="toggle"
    >
      <el-icon :size="18"><MagicStick /></el-icon>
      <span class="hidden sm:inline">AI 助手</span>
    </button>

    <!-- 对话面板 -->
    <div
      v-else
      class="fixed bottom-0 right-0 z-40 flex h-[70vh] w-full flex-col bg-white pb-[env(safe-area-inset-bottom)] shadow-2xl sm:bottom-5 sm:right-5 sm:h-[560px] sm:w-96 sm:rounded-2xl sm:border sm:border-slate-200 sm:pb-0 sm:dark:border-slate-700 dark:bg-slate-900"
    >
      <div class="flex items-center justify-between border-b border-slate-100 px-4 py-3 dark:border-slate-800">
        <div class="flex items-center gap-1.5 font-semibold text-slate-900 dark:text-white">
          <el-icon class="text-violet-600"><MagicStick /></el-icon> AI 助手
        </div>
        <div class="flex items-center gap-1">
          <button class="rounded p-1 text-slate-400 transition hover:text-violet-600" title="新对话" @click="newChat"><el-icon><Plus /></el-icon></button>
          <button class="rounded p-1 text-slate-400 transition hover:text-slate-700 dark:hover:text-white" title="收起" @click="open = false"><el-icon><Close /></el-icon></button>
        </div>
      </div>

      <div ref="scrollRef" class="min-h-0 flex-1 space-y-3 overflow-y-auto p-3">
        <div v-if="!messages.length" class="flex h-full items-center justify-center px-6 text-center text-sm text-slate-400">
          问我软考架构师相关的任何问题～
        </div>
        <div v-for="(m, i) in messages" :key="i" class="flex gap-2" :class="m.role === 'user' ? 'flex-row-reverse' : ''">
          <div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-[10px] font-medium text-white" :class="m.role === 'user' ? 'bg-slate-600' : 'bg-violet-600'">{{ m.role === 'user' ? '我' : 'AI' }}</div>
          <div class="max-w-[80%] rounded-2xl px-3 py-2 text-sm" :class="m.role === 'user' ? 'bg-blue-600 text-white' : 'bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-100'">
            <span v-if="m.role === 'user'" class="whitespace-pre-wrap">{{ m.content }}</span>
            <div v-else class="md-content" v-html="renderMd(m.content) || '<span class=\'text-slate-400\'>思考中…</span>'" />
          </div>
        </div>
      </div>

      <div class="border-t border-slate-100 p-2.5 dark:border-slate-800">
        <div class="flex items-end gap-2">
          <textarea
            v-model="input"
            rows="1"
            placeholder="输入问题，Enter 发送"
            class="max-h-28 min-h-[38px] flex-1 resize-none rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm focus:border-violet-400 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            @keydown.enter.exact.prevent="send"
          />
          <button v-if="!streaming" class="shrink-0 rounded-lg bg-violet-600 px-3.5 py-2 text-sm font-medium text-white transition hover:bg-violet-700 disabled:opacity-50" :disabled="!input.trim()" @click="send">发送</button>
          <button v-else class="shrink-0 rounded-lg border border-slate-300 px-3.5 py-2 text-sm dark:border-slate-600" @click="stop">停止</button>
        </div>
      </div>
    </div>
  </div>
</template>
