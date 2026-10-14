<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { examScheduleApi } from '@/api'

interface ScheduleRow {
  id: number
  label: string
  exam_date: string
}

const list = ref<ScheduleRow[]>([])
const loading = ref(false)

const dialogVisible = ref(false)
const editing = ref<ScheduleRow | null>(null)
const form = ref({ label: '', exam_date: '' })
const saving = ref(false)

// 今天零点（本地时区）——过去/今天已过的排期标注「已过期」
const today = computed(() => {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
})

async function load() {
  loading.value = true
  try {
    const res: any = await examScheduleApi.adminList()
    list.value = res.data || []
  } finally {
    loading.value = false
  }
}

onMounted(load)

function openCreate() {
  editing.value = null
  form.value = { label: '', exam_date: '' }
  dialogVisible.value = true
}

function openEdit(row: ScheduleRow) {
  editing.value = row
  form.value = { label: row.label, exam_date: String(row.exam_date).slice(0, 10) }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.label.trim() || !form.value.exam_date) {
    ElMessage.warning('请填写排期名称和考试日期')
    return
  }
  saving.value = true
  try {
    const payload = { label: form.value.label.trim(), exam_date: form.value.exam_date }
    if (editing.value) {
      await examScheduleApi.update(editing.value.id, payload)
    } else {
      await examScheduleApi.create(payload)
    }
    dialogVisible.value = false
    ElMessage.success('已保存')
    load()
  } finally {
    saving.value = false
  }
}

async function remove(row: ScheduleRow) {
  await ElMessageBox.confirm(`确定删除排期「${row.label}」？删除后前台倒计时将不再显示该场次。`, '提示', { type: 'warning' })
  await examScheduleApi.delete(row.id)
  ElMessage.success('已删除')
  load()
}
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold">考试排期</h2>
      <el-button type="primary" @click="openCreate">新增排期</el-button>
    </div>

    <el-alert
      class="mb-4"
      type="info"
      :closable="false"
      show-icon
      title="考试日期全国统一，保存后前台倒计时（顶部距考提示、首页倒计时卡、登录页横幅）自动更新。"
    />

    <el-table :data="list" stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="label" label="排期名称" min-width="160" />
      <el-table-column label="考试日期" width="200">
        <template #default="{ row }: { row: any }">
          <span :class="row.exam_date?.slice(0, 10) < today ? 'text-slate-400' : ''">
            {{ String(row.exam_date).slice(0, 10) }}
          </span>
          <el-tag v-if="row.exam_date?.slice(0, 10) < today" class="ml-2" size="small" type="info">已过期</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="{ row }: { row: any }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
      <template #empty>
        <el-empty description="暂无排期，点击右上角新增" />
      </template>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑排期' : '新增排期'" width="480px">
      <el-form label-width="80px" @submit.prevent>
        <el-form-item label="排期名称" required>
          <el-input v-model="form.label" placeholder="如：2026年秋季" maxlength="50" show-word-limit />
        </el-form-item>
        <el-form-item label="考试日期" required>
          <el-date-picker
            v-model="form.exam_date"
            type="date"
            value-format="YYYY-MM-DD"
            placeholder="选择考试日期"
            class="!w-full"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
