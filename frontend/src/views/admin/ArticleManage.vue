<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { articleApi } from '@/api'
import { useSubjectStore } from '@/stores/subject'
import type { Article } from '@/types'
import { ElMessage, ElMessageBox } from 'element-plus'

const subjectStore = useSubjectStore()
const articles = ref<Article[]>([])
const total = ref(0)
const page = ref(1)
const keyword = ref('')
const articleType = ref<number>(0) // 0=全部
const category = ref('')
const dialogVisible = ref(false)
const form = ref({ title: '', content: '', article_type: 1, category: '', year: 0, author: '', tags: '' })
const editingId = ref<number | null>(null)

const uploading = ref(false)

async function load() {
  const res: any = await articleApi.list({
    page: page.value,
    page_size: 20,
    subject_id: subjectStore.currentSubjectId,
    article_type: articleType.value || undefined,
    keyword: keyword.value || undefined,
    category: category.value || undefined,
  })
  articles.value = res.data.items || []
  total.value = res.data.total || 0
  // 当前页越界时自动回到第 1 页，避免停在一页空结果上
  if (!articles.value.length && page.value > 1) {
    page.value = 1
    load()
  }
}

onMounted(load)
// 切换科目后强制刷新数据(整页 reload 是首选;此处 watch 是兜底,防止 reload 因任何原因被吞
// 时还停留在旧科目数据上)
watch(() => subjectStore.currentSubjectId, () => load())

function search() {
  page.value = 1
  load()
}

function reset() {
  keyword.value = ''
  articleType.value = 0
  category.value = ''
  page.value = 1
  load()
}

function openCreate() {
  editingId.value = null
  form.value = { title: '', content: '', article_type: 1, category: '', year: 0, author: '', tags: '' }
  dialogVisible.value = true
}

function openEdit(item: Article) {
  editingId.value = item.id
  form.value = { title: item.title, content: item.content, article_type: item.article_type, category: item.category, year: item.year, author: item.author, tags: item.tags }
  dialogVisible.value = true
}

async function save() {
  const payload = { ...form.value, subject_id: subjectStore.currentSubjectId }
  if (editingId.value) {
    await articleApi.update(editingId.value, payload)
  } else {
    await articleApi.create(payload)
  }
  ElMessage.success('保存成功')
  dialogVisible.value = false
  load()
}

async function handleDelete(id: number) {
  await ElMessageBox.confirm('确认删除？')
  await articleApi.delete(id)
  ElMessage.success('删除成功')
  load()
}
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-xl font-bold">
        论文管理
        <el-tag size="small" class="ml-2">{{ subjectStore.currentSubject()?.name || '当前科目' }}</el-tag>
      </h2>
      <el-button type="primary" @click="openCreate">新增文章</el-button>
    </div>

    <p class="mb-3 text-sm text-gray-400">当前仅展示 / 维护「{{ subjectStore.currentSubject()?.name }}」的论文，切换右上角科目可管理其他科目。</p>

    <div class="mb-4 flex flex-wrap items-center gap-3">
      <el-input
        v-model="keyword"
        placeholder="搜索标题"
        clearable
        style="width: 220px"
        @keyup.enter="search"
        @clear="search"
      />
      <el-select
        v-model="articleType"
        placeholder="类型"
        style="width: 160px"
        @change="search"
      >
        <el-option label="类型：全部" :value="0" />
        <el-option label="论文范文" :value="1" />
        <el-option label="写作模板" :value="2" />
        <el-option label="评分标准" :value="3" />
        <el-option label="AI点评" :value="4" />
      </el-select>
      <el-input
        v-model="category"
        placeholder="分类"
        clearable
        style="width: 180px"
        @keyup.enter="search"
        @clear="search"
      />
      <el-button type="primary" @click="search">搜索</el-button>
      <el-button @click="reset">重置</el-button>
    </div>

    <el-table :data="articles" stripe>
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="title" label="标题" show-overflow-tooltip />
      <el-table-column prop="article_type" label="类型" width="100">
        <template #default="{ row }: { row: any }">
          {{ ({ 1: '论文范文', 2: '写作模板', 3: '评分标准', 4: 'AI点评' } as Record<number, string>)[row.article_type] }}
        </template>
      </el-table-column>
      <el-table-column prop="year" label="年份" width="80" />
      <el-table-column prop="category" label="分类" width="140" show-overflow-tooltip />
      <el-table-column label="操作" width="150">
        <template #default="{ row }: { row: any }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="handleDelete(row.id)">删除</el-button>
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

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑文章' : '新增文章'" width="700px">
      <el-form v-loading="uploading" element-loading-text="图片上传中…" :model="form" label-width="80px">
        <el-form-item label="标题">
          <el-input v-model="form.title" />
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="form.article_type">
            <el-option label="论文范文" :value="1" />
            <el-option label="写作模板" :value="2" />
            <el-option label="评分标准" :value="3" />
            <el-option label="AI点评" :value="4" />
          </el-select>
        </el-form-item>
        <el-form-item label="分类">
          <el-input v-model="form.category" placeholder="如：软件架构、需求工程（可留空）" />
        </el-form-item>
        <el-form-item label="年份">
          <el-input-number v-model="form.year" :min="2000" :max="2030" />
        </el-form-item>
        <el-form-item label="作者">
          <el-input v-model="form.author" />
        </el-form-item>
        <el-form-item label="内容">
          <FullScreenEditor
            v-model="form.content"
            label="论文正文"
            :rows="10"
            :preview="true"
            placeholder="支持 Markdown，可直接粘贴图片自动上传"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
