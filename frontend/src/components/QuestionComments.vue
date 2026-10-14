<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { commentApi } from '@/api'
import { useUserStore } from '@/stores/user'

const props = defineProps<{ questionId: number }>()

interface CommentUser { nickname: string; avatar: string }
interface ReplyItem {
  id: number; content: string; user: CommentUser; reply_to_user_id: number
  is_owner: boolean; created_at: string
}
interface CommentItem {
  id: number; content: string; user: CommentUser; reply_count: number
  replies: ReplyItem[] | null; is_owner: boolean; created_at: string
}

const userStore = useUserStore()
const router = useRouter()
const items = ref<CommentItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const loading = ref(false)
const draft = ref('')          // 顶级评论输入框
const replyDraft = ref<Record<number, string>>({})   // 每条顶级评论的回复输入框
const replyingTo = ref<number | null>(null)          // 当前展开回复输入框的顶级评论 id
const reportDialog = ref(false)
const reportReason = ref('')
const reportTarget = ref<{ id: number; kind: 'top' | 'reply' } | null>(null)
const submitting = ref(false)

const REPORT_REASONS = ['垃圾广告', '政治敏感', '人身攻击', '剧透刷屏', '其他']

function fmtTime(iso: string) {
  const d = new Date(iso)
  const now = Date.now()
  const diff = Math.floor((now - d.getTime()) / 1000)
  if (diff < 60) return '刚刚'
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`
  if (diff < 86400 * 30) return `${Math.floor(diff / 86400)} 天前`
  return d.toLocaleDateString('zh-CN')
}

async function load() {
  loading.value = true
  try {
    const res: any = await commentApi.list(props.questionId, { page: page.value, page_size: pageSize })
    items.value = res.data.items || []
    total.value = res.data.total || 0
  } catch {} finally {
    loading.value = false
  }
}

function goLogin() {
  router.push('/login')
}

async function submitTop() {
  if (!draft.value.trim()) return
  submitting.value = true
  try {
    await commentApi.create(props.questionId, { content: draft.value })
    draft.value = ''
    page.value = 1
    await load()
  } catch {} finally {
    submitting.value = false
  }
}

function toggleReply(id: number) {
  replyingTo.value = replyingTo.value === id ? null : id
}

async function submitReply(parentId: number, parentAuthor: string) {
  const content = (replyDraft.value[parentId] || '').trim()
  if (!content) return
  submitting.value = true
  try {
    await commentApi.create(props.questionId, { content, parent_id: parentId })
    replyDraft.value[parentId] = ''
    replyingTo.value = null
    await load()
  } catch {} finally {
    submitting.value = false
  }
}

async function removeComment(id: number) {
  try {
    await ElMessageBox.confirm('确定删除这条评论吗？删除后不可恢复。', '删除评论', { type: 'warning' })
  } catch { return }
  try {
    await commentApi.remove(id)
    await load()
  } catch {}
}

function openReport(id: number, kind: 'top' | 'reply') {
  reportTarget.value = { id, kind }
  reportReason.value = ''
  reportDialog.value = true
}

async function submitReport() {
  if (!reportTarget.value || !reportReason.value) return
  try {
    await commentApi.report(reportTarget.value.id, { reason: reportReason.value })
    ElMessage.success('举报已提交，我们会尽快处理')
    reportDialog.value = false
  } catch {}
}

function onPageChange(p: number) {
  page.value = p
  load()
}

function parentNickname(c: CommentItem, r: ReplyItem) {
  // 单层回复:回复对象必是顶级评论作者(reply_to_user_id == c.user)
  return c.user.nickname || '匿名'
}

onMounted(load)
</script>

<template>
  <div class="mt-6 rounded-lg bg-white p-4 shadow transition-colors dark:bg-gray-800">
    <div class="mb-3 flex items-center justify-between">
      <span class="font-semibold text-gray-800 dark:text-gray-200">讨论 ({{ total }})</span>
    </div>

    <!-- 顶级评论输入 -->
    <div v-if="userStore.isLoggedIn" class="mb-4">
      <el-input
        v-model="draft"
        type="textarea"
        :rows="3"
        maxlength="500"
        show-word-limit
        placeholder="说说你对这道题的理解，或提问……"
      />
      <div class="mt-2 text-right">
        <el-button type="primary" :disabled="!draft.trim()" :loading="submitting" @click="submitTop">
          发表评论
        </el-button>
      </div>
    </div>
    <div v-else class="mb-4 rounded border border-dashed border-gray-300 p-4 text-center text-sm text-gray-500 dark:border-gray-600 dark:text-gray-400">
      登录后可参与讨论
      <el-button size="small" class="ml-2" @click="goLogin">去登录</el-button>
    </div>

    <div v-loading="loading">
      <p v-if="!items.length && !loading" class="py-6 text-center text-sm text-gray-400">
        还没有讨论，来抢沙发～
      </p>

      <!-- 顶级评论 -->
      <div v-for="c in items" :key="c.id" class="border-b border-gray-100 py-3 last:border-0 dark:border-gray-700">
        <div class="flex items-start gap-2">
          <el-avatar :size="32" :src="c.user.avatar || ''">{{ c.user.nickname?.slice(0, 1) || '?' }}</el-avatar>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 text-sm">
              <span class="font-medium text-gray-800 dark:text-gray-200">{{ c.user.nickname || '匿名' }}</span>
              <span class="text-xs text-gray-400">{{ fmtTime(c.created_at) }}</span>
              <div class="ml-auto flex items-center gap-1">
                <el-button link size="small" @click="toggleReply(c.id)">回复</el-button>
                <el-button v-if="c.is_owner" link size="small" type="danger" @click="removeComment(c.id)">删除</el-button>
                <el-button link size="small" type="info" @click="openReport(c.id, 'top')">举报</el-button>
              </div>
            </div>
            <div class="mt-1 whitespace-pre-wrap break-words text-sm leading-relaxed text-gray-700 dark:text-gray-300">
              {{ c.content }}
            </div>

            <!-- 回复输入框 -->
            <div v-if="replyingTo === c.id" class="mt-2">
              <el-input
                v-model="replyDraft[c.id]"
                type="textarea"
                :rows="2"
                maxlength="500"
                show-word-limit
                :placeholder="`回复 @${c.user.nickname || '匿名'}`"
              />
              <div class="mt-2 text-right">
                <el-button size="small" @click="replyingTo = null">取消</el-button>
                <el-button size="small" type="primary" :disabled="!replyDraft[c.id]?.trim()" :loading="submitting" @click="submitReply(c.id, c.user.nickname || '')">
                  回复
                </el-button>
              </div>
            </div>

            <!-- 楼中楼回复:replies 可能为 null(服务层无回复时不返回 []),需空值防护 -->
            <div v-if="(c.replies || []).length" class="mt-2 space-y-2 rounded bg-gray-50 p-2 dark:bg-gray-900/40">
              <div v-for="r in (c.replies || [])" :key="r.id" class="flex items-start gap-2">
                <el-avatar :size="24" :src="r.user.avatar || ''">{{ r.user.nickname?.slice(0, 1) || '?' }}</el-avatar>
                <div class="min-w-0 flex-1 text-sm">
                  <span class="font-medium text-gray-700 dark:text-gray-300">{{ r.user.nickname || '匿名' }}</span>
                  <span class="text-xs text-gray-400"> 回复 @{{ parentNickname(c, r) }}</span>
                  <span class="text-xs text-gray-400"> · {{ fmtTime(r.created_at) }}</span>
                  <div class="whitespace-pre-wrap break-words text-gray-600 dark:text-gray-400">{{ r.content }}</div>
                </div>
                <div class="flex items-center gap-1">
                  <el-button v-if="r.is_owner" link size="small" type="danger" @click="removeComment(r.id)">删除</el-button>
                  <el-button link size="small" type="info" @click="openReport(r.id, 'reply')">举报</el-button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="total > pageSize" class="mt-3 flex justify-center">
        <el-pagination
          layout="prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="onPageChange"
        />
      </div>
    </div>

    <!-- 举报对话框 -->
    <el-dialog v-model="reportDialog" title="举报这条评论" width="420px">
      <el-select v-model="reportReason" placeholder="请选择举报理由" style="width: 100%">
        <el-option v-for="r in REPORT_REASONS" :key="r" :label="r" :value="r" />
      </el-select>
      <template #footer>
        <el-button @click="reportDialog = false">取消</el-button>
        <el-button type="danger" :disabled="!reportReason" @click="submitReport">提交举报</el-button>
      </template>
    </el-dialog>
  </div>
</template>
