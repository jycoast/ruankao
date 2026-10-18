<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { announcementApi } from '@/api'
import type { Announcement } from '@/types'

const list = ref<Announcement[]>([])
const total = ref(0)
const page = ref(1)

const dialogVisible = ref(false)
const editing = ref<Announcement | null>(null)
const form = ref({ title: '', content: '', status: 1 })
const saving = ref(false)

async function load() {
  const res: any = await announcementApi.adminList({ page: page.value, page_size: 20 })
  list.value = res.data.items || []
  total.value = res.data.total || 0
}

onMounted(load)

function openCreate() {
  editing.value = null
  form.value = { title: '', content: '', status: 1 }
  dialogVisible.value = true
}

function openEdit(row: Announcement) {
  editing.value = row
  form.value = { title: row.title, content: row.content, status: row.status }
  dialogVisible.value = true
}

async function save() {
  if (!form.value.title.trim() || !form.value.content.trim()) {
    ElMessage.warning('请填写标题和内容')
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await announcementApi.update(editing.value.id, form.value)
    } else {
      await announcementApi.create({ title: form.value.title, content: form.value.content })
    }
    dialogVisible.value = false
    ElMessage.success('已保存')
    load()
  } finally {
    saving.value = false
  }
}

async function remove(row: Announcement) {
  await ElMessageBox.confirm(`确定删除公告「${row.title}」？`, '提示', { type: 'warning' })
  await announcementApi.delete(row.id)
  ElMessage.success('已删除')
  load()
}

async function toggleStatus(row: Announcement) {
  const status = row.status === 1 ? 0 : 1
  await announcementApi.update(row.id, { status })
  load()
}
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold">公告管理</h2>
      <el-button type="primary" @click="openCreate">发布公告</el-button>
    </div>

    <el-table :data="list" stripe>
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="title" label="标题" min-width="200" show-overflow-tooltip />
      <el-table-column prop="status" label="状态" width="100">
        <template #default="{ row }: { row: any }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '发布中' : '已下线' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="200" />
      <el-table-column label="操作" width="260">
        <template #default="{ row }: { row: any }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" :type="row.status === 1 ? 'warning' : 'success'" @click="toggleStatus(row)">
            {{ row.status === 1 ? '下线' : '上线' }}
          </el-button>
          <el-button size="small" type="danger" @click="remove(row)">删除</el-button>
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

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑公告' : '发布公告'" width="600px">
      <el-form label-width="60px">
        <el-form-item label="标题">
          <el-input v-model="form.title" placeholder="公告标题" maxlength="200" show-word-limit />
        </el-form-item>
        <el-form-item label="内容">
          <el-input v-model="form.content" type="textarea" :rows="8" placeholder="支持 Markdown" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
