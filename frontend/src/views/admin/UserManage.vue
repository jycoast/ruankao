<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminApi } from '@/api'
import type { User } from '@/types'

const users = ref<User[]>([])
const total = ref(0)
const page = ref(1)
const keyword = ref('')
const role = ref<number>(-1) // -1=全部 0=用户 1=管理员
const status = ref<number>(-1) // -1=全部 0=禁用 1=正常

async function load() {
  const res: any = await adminApi.listUsers({
    page: page.value,
    page_size: 20,
    keyword: keyword.value || undefined,
    role: role.value,
    status: status.value,
  })
  users.value = res.data.items || []
  total.value = res.data.total || 0
  // 当前页越界（如筛选后结果变少、或同名并发删除）时自动回到第 1 页，避免停在一页空结果上
  if (!users.value.length && page.value > 1) {
    page.value = 1
    load()
  }
}

onMounted(load)

function search() {
  page.value = 1
  load()
}

function reset() {
  keyword.value = ''
  role.value = -1
  status.value = -1
  page.value = 1
  load()
}

async function toggleStatus(user: User) {
  const newStatus = user.status === 1 ? 0 : 1
  await adminApi.updateUserStatus(user.id, newStatus)
  load()
}

async function toggleRole(user: User) {
  const newRole = user.role === 1 ? 0 : 1
  await ElMessageBox.confirm(
    newRole === 1 ? `将「${user.username}」设为管理员？` : `取消「${user.username}」的管理员权限？`,
    '提示',
    { type: 'warning' },
  )
  await adminApi.updateUserRole(user.id, newRole)
  ElMessage.success('已更新')
  load()
}

async function resetPassword(user: User) {
  const res = await ElMessageBox.prompt(
    `为「${user.username}」设置新密码（至少6位），请自行告知用户。`,
    '重置密码',
    {
      inputType: 'password',
      inputPlaceholder: '请输入新密码',
      inputValidator: (v: string) => (v && v.length >= 6 ? true : '密码长度不能少于6位'),
    },
  ).catch(() => null)
  if (!res) return
  await adminApi.resetUserPassword(user.id, res.value)
  ElMessage.success('密码已重置')
}
</script>

<template>
  <div>
        <h2 class="text-xl font-bold mb-4">用户管理</h2>
    <div class="mb-4 flex flex-wrap items-center gap-3">
      <el-input
        v-model="keyword"
        placeholder="搜索用户名 / 邮箱 / 昵称"
        clearable
        style="width: 240px"
        @keyup.enter="search"
        @clear="search"
      />
      <el-select v-model="role" placeholder="角色" style="width: 140px" @change="search">
        <el-option label="角色：全部" :value="-1" />
        <el-option label="管理员" :value="1" />
        <el-option label="普通用户" :value="0" />
      </el-select>
      <el-select v-model="status" placeholder="状态" style="width: 140px" @change="search">
        <el-option label="状态：全部" :value="-1" />
        <el-option label="正常" :value="1" />
        <el-option label="禁用" :value="0" />
      </el-select>
      <el-button type="primary" @click="search">搜索</el-button>
      <el-button @click="reset">重置</el-button>
    </div>
    <el-table :data="users" stripe>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="username" label="用户名" />
      <el-table-column prop="email" label="邮箱" />
      <el-table-column prop="nickname" label="昵称" />
      <el-table-column prop="role" label="角色" width="100">
        <template #default="{ row }: { row: any }">
          <el-tag :type="row.role === 1 ? 'danger' : 'info'">{{ row.role === 1 ? '管理员' : '用户' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="100">
        <template #default="{ row }: { row: any }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'">{{ row.status === 1 ? '正常' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="is_member" label="会员" width="90">
        <template #default="{ row }: { row: any }">
          <el-tag :type="row.is_member ? 'warning' : 'info'">{{ row.is_member ? '会员' : '非会员' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="days_left" label="剩余天数" width="130">
        <template #default="{ row }: { row: any }">
          <span v-if="row.is_member" :title="row.expire_at ? '到期时间：' + row.expire_at : ''">
            {{ row.days_left }} 天
          </span>
          <span v-else class="text-gray-400">—</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="300">
        <template #default="{ row }: { row: any }">
          <el-button size="small" :type="row.status === 1 ? 'danger' : 'success'" @click="toggleStatus(row)">
            {{ row.status === 1 ? '禁用' : '启用' }}
          </el-button>
          <el-button size="small" :type="row.role === 1 ? 'info' : 'primary'" @click="toggleRole(row)">
            {{ row.role === 1 ? '设为用户' : '设为管理员' }}
          </el-button>
          <el-button size="small" type="warning" @click="resetPassword(row)">重置密码</el-button>
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
  </div>
</template>
