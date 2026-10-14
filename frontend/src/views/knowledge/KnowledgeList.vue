<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import MindmapView from '@/components/knowledge/MindmapView.vue'
import { knowledgeApi } from '@/api'
import { useSubjectStore } from '@/stores/subject'
import { requireMember } from '@/utils/memberGate'
import { exportKnowledgeImage, printKnowledgePdf } from '@/utils/knowledgeExport'

// —— 导出（会员专属）：整树展开 + 详情全文 ——
// PDF 走浏览器打印管线，一章一页：文字是真矢量（放大不糊），图片按原始分辨率嵌入。
// 图片长图是位图，受画布面积上限限制，300+ 卡片时放大必然发虚——它只适合快速分享。
const subjectStore = useSubjectStore()
const printing = ref(false)
const printStage = ref<'layout' | 'print' | ''>('')
const exporting = ref(false)
const exportingNow = ref(false)
const image = ref('')
const showResult = ref(false)

async function loadItems() {
  const items = await knowledgeApi.listAll(subjectStore.currentSubjectId)
  if (!items.length) ElMessage.warning('当前科目暂无知识点数据')
  return items
}

async function onExportPdf() {
  if (!requireMember('knowledgeExport')) return
  if (printing.value) return
  printing.value = true
  printStage.value = ''
  try {
    const items = await loadItems()
    if (!items.length) return
    await printKnowledgePdf(items, subjectStore.currentSubject()?.name || '软考', (stage) => {
      printStage.value = stage
      // 排完版、对话框即将弹出时才提示：早于此刻提示，等用户看到对话框时早就消失了
      if (stage === 'print') {
        ElMessage.info('在打印对话框中把「目标打印机」选为「另存为 PDF」，纸张设为默认即可')
      }
    })
  } catch (e: any) {
    ElMessage.error(e?.message || '导出失败，请重试')
  } finally {
    printing.value = false
    printStage.value = ''
  }
}

async function onExport() {
  if (!requireMember('knowledgeExport')) return
  if (exporting.value) return
  exporting.value = true
  try {
    const items = await loadItems()
    if (!items.length) return
    exportingNow.value = true
    image.value = await exportKnowledgeImage(items, subjectStore.currentSubject()?.name || '软考', (done, total) => {
      exportingNow.value = total > 0 && done < total
    })
    showResult.value = true
  } catch (e: any) {
    ElMessage.error(e?.message || '导出失败，请重试')
  } finally {
    exporting.value = false
    exportingNow.value = false
  }
}

function imageName() {
  return `${subjectStore.currentSubject()?.name || '软考'}-知识点导图.png`
}

function downloadImage() {
  const a = document.createElement('a')
  a.href = image.value
  a.download = imageName()
  a.click()
}
</script>

<template>
  <div>
    <div class="mb-4 flex items-center justify-between">
      <h2 class="text-xl font-bold text-gray-900 dark:text-gray-100">知识导图</h2>
      <div class="flex items-center gap-2">
        <el-button
          type="primary"
          :loading="printing"
          :disabled="printing || exporting"
          @click="onExportPdf"
        >
          <template v-if="!printing" #icon><el-icon><Printer /></el-icon></template>
          {{ printing ? (printStage === 'print' ? '准备打印…' : '排版中…') : '导出 PDF' }}
        </el-button>
        <el-button :loading="exporting" :disabled="exporting || printing" @click="onExport">
          <template v-if="!exporting" #icon><el-icon><Download /></el-icon></template>
          {{ exporting ? (exportingNow ? '截图渲染中…' : '布局计算中…') : '导出图片' }}
        </el-button>
      </div>
    </div>

    <MindmapView />

    <!-- 导出结果：单张长图；点击进全屏查看器（滚轮缩放平移，详情里的图片随整页放大） -->
    <el-dialog v-model="showResult" title="导出成功" width="640px" class="max-sm:!w-[calc(100vw-24px)]">
      <template #header>
        <div class="flex items-center justify-between pr-2">
          <span class="text-base font-semibold">导出成功</span>
          <el-button size="small" type="primary" @click="downloadImage">下载图片</el-button>
        </div>
      </template>
      <p class="mb-3 text-sm text-slate-500 dark:text-slate-400">
        已生成整树思维导图长图。图片较长，点击可全屏查看，滚轮缩放后可逐区域细读。
      </p>
      <el-image
        v-if="image"
        :src="image"
        :preview-src-list="[image]"
        :preview-teleported="true"
        fit="contain"
        class="max-h-[50vh] w-full cursor-zoom-in rounded-lg border border-slate-200 dark:border-slate-700"
      />
    </el-dialog>
  </div>
</template>
