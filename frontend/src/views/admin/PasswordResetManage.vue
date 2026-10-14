<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { passwordResetApi } from '@/api'

interface ResetRequest {
  id: number
  user_id: number
  username: string
  email: string
  account: string
  contact: string
  reason: string
  client_ip: string
  status: number
  admin_note: string
  created_at: string
}

const list = ref<ResetRequest[]>([])
const total = ref(0)
const page = ref(1)
const statusFilter = ref(0) // 默认只看待处理

const detailVisible = ref(false)
const current = ref<ResetRequest | null>(null)
const newPassword = ref('')
const adminNote = ref('')
const saving = ref(false)

const STATUS_LABELS: Record<number, { text: string; type: string }> = {
  0: { text: '待处理', type: 'danger' },
  1: { text: '已重置', type: 'success' },
  2: { text: '已忽略', type: 'info' },
}

async function load() {
  const res: any = await passwordResetApi.adminList({
    page: page.value,
    page_size: 20,
    status: statusFilter.value,
  })
  list.value = res.data.items || []
  total.value = res.data.total || 0
}

onMounted(load)

function changeStatus(val: number) {
  statusFilter.value = val
  page.value = 1
  load()
}

// 随机密码比让管理员现编强：现编的多半是 123456 这类，且容易复用到多个账号
function genPassword() {
  const chars = 'abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789'
  const buf = new Uint32Array(12)
  crypto.getRandomValues(buf)
  newPassword.value = Array.from(buf, (n) => chars[n % chars.length]).join('')
}

function openDetail(row: ResetRequest) {
  current.value = row
  adminNote.value = row.admin_note || ''
  newPassword.value = ''
  if (row.status === 0) genPassword()
  detailVisible.value = true
}

async function copyPassword() {
  try {
    await navigator.clipboard.writeText(newPassword.value)
    ElMessage.success('已复制')
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}

async function approve() {
  if (!current.value) return
  if (newPassword.value.length < 6) {
    ElMessage.warning('新密码至少 6 位')
    return
  }
  // 密码只在这里出现这一次，后端不落库；关掉弹窗就再也拿不到了
  await copyPassword()
  saving.value = true
  try {
    await passwordResetApi.handle(current.value.id, {
      status: 1,
      new_password: newPassword.value,
      admin_note: adminNote.value,
    })
    ElMessage.success('密码已重置，新密码已复制到剪贴板，请通过申请人留的联系方式告知')
    detailVisible.value = false
    load()
  } finally {
    saving.value = false
  }
}

async function reject() {
  if (!current.value) return
  saving.value = true
  try {
    await passwordResetApi.handle(current.value.id, { status: 2, admin_note: adminNote.value })
    ElMessage.success('已忽略该申请')
    detailVisible.value = false
    load()
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div>
    <div class="mb-4 flex items-center justify-between">
      <div>
        <h2 class="text-xl font-bold">找回密码申请</h2>
        <p class="mt-1 text-sm text-slate-500">站点未接入邮件服务，重置走人工核实。请先通过申请人留的联系方式确认身份，再重置。</p>
      </div>
      <el-radio-group :model-value="statusFilter" @change="changeStatus as any">
        <el-radio-button :value="-1">全部</el-radio-button>
        <el-radio-button :value="0">待处理</el-radio-button>
        <el-radio-button :value="1">已重置</el-radio-button>
        <el-radio-button :value="2">已忽略</el-radio-button>
      </el-radio-group>
    </div>

    <el-table :data="list" stripe>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column label="账号" min-width="140">
        <template #default="{ row }: { row: any }">
          <div class="text-sm">{{ row.username }}</div>
          <div class="text-xs text-slate-400">{{ row.email }}</div>
        </template>
      </el-table-column>
      <el-table-column label="提交时填写" min-width="140">
        <template #default="{ row }: { row: any }">
          <span class="text-sm text-gray-600">{{ row.account }}</span>
        </template>
      </el-table-column>
      <el-table-column label="联系方式" min-width="130">
        <template #default="{ row }: { row: any }">
          <span class="text-sm text-gray-600">{{ row.contact || '—' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="来源 IP" width="130">
        <template #default="{ row }: { row: any }">
          <span class="text-xs text-slate-400">{{ row.client_ip || '—' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }: { row: any }">
          <el-tag :type="STATUS_LABELS[row.status]?.type as any" size="small">
            {{ STATUS_LABELS[row.status]?.text }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="提交时间" width="170">
        <template #default="{ row }: { row: any }">
          <span class="text-sm text-gray-500">{{ row.created_at?.slice(0, 16) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="100">
        <template #default="{ row }: { row: any }">
          <el-button size="small" type="primary" plain @click="openDetail(row as ResetRequest)">
            {{ row.status === 0 ? '处理' : '查看' }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="mt-4 flex justify-center">
      <el-pagination
        v-model:current-page="page"
        :page-size="20"
        :total="total"
        layout="prev, pager, next"
        @current-change="load"
      />
    </div>

    <el-dialog v-model="detailVisible" title="处理找回密码申请" width="560px">
      <template v-if="current">
        <el-descriptions :column="1" border size="small" class="mb-4">
          <el-descriptions-item label="账号">{{ current.username }}（{{ current.email }}）</el-descriptions-item>
          <el-descriptions-item label="提交时填写">{{ current.account }}</el-descriptions-item>
          <el-descriptions-item label="联系方式">{{ current.contact || '未填写' }}</el-descriptions-item>
          <el-descriptions-item label="补充说明">
            <span class="whitespace-pre-wrap">{{ current.reason || '无' }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="来源 IP">{{ current.client_ip || '未知' }}</el-descriptions-item>
          <el-descriptions-item label="当前状态">
            <el-tag :type="STATUS_LABELS[current.status]?.type as any" size="small">
              {{ STATUS_LABELS[current.status]?.text }}
            </el-tag>
          </el-descriptions-item>
        </el-descriptions>

        <template v-if="current.status === 0">
          <el-alert
            type="warning"
            :closable="false"
            class="mb-3"
            title="重置前请先通过上面的联系方式核实身份。新密码不会入库，点「确认重置」后只在剪贴板里出现这一次，弹窗关掉就找不回了。"
          />
          <el-form label-position="top">
            <el-form-item label="新密码">
              <el-input v-model="newPassword" placeholder="至少 6 位">
                <template #append>
                  <el-button @click="genPassword">重新生成</el-button>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item label="处理备注">
              <el-input
                v-model="adminNote"
                type="textarea"
                :rows="2"
                placeholder="如「已通过 QQ 核实身份」（选填）"
                maxlength="500"
                show-word-limit
              />
            </el-form-item>
          </el-form>
        </template>
        <div v-else-if="current.admin_note" class="text-sm text-slate-500">
          处理备注：{{ current.admin_note }}
        </div>
      </template>
      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
        <template v-if="current?.status === 0">
          <el-button type="info" :loading="saving" @click="reject">忽略</el-button>
          <el-button type="danger" :loading="saving" @click="approve">确认重置并复制密码</el-button>
        </template>
      </template>
    </el-dialog>
  </div>
</template>
