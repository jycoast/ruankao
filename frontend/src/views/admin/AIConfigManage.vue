<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { aiApi } from '@/api'

const form = ref({
  base_url: '',
  api_key: '',
  model: '',
  temperature: 0.7,
  max_tokens: 2048,
  enabled: 0,
  free_trial_limit: 5,
})
const keySet = ref(false)
const keyMasked = ref('')
const showKey = ref(false)
const saving = ref(false)
const testing = ref(false)

async function copyKey() {
  const val = form.value.api_key || keyMasked.value
  if (!val) return
  try {
    await navigator.clipboard.writeText(val)
    ElMessage.success('已复制')
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}

async function load() {
  try {
    const res: any = await aiApi.getConfig()
    const d = res.data
    form.value.base_url = d.base_url || ''
    form.value.model = d.model || ''
    form.value.temperature = Number(d.temperature) || 0.7
    form.value.max_tokens = d.max_tokens || 2048
    form.value.enabled = d.enabled || 0
    form.value.free_trial_limit = d.free_trial_limit ?? 5
    keySet.value = !!d.key_set
    keyMasked.value = d.key_masked || ''
  } catch {}
}
onMounted(load)

async function save() {
  saving.value = true
  try {
    await aiApi.updateConfig({
      base_url: form.value.base_url.trim(),
      api_key: form.value.api_key, // 留空则后端保留原值
      model: form.value.model.trim(),
      temperature: form.value.temperature,
      max_tokens: form.value.max_tokens,
      enabled: form.value.enabled,
      free_trial_limit: form.value.free_trial_limit,
    })
    ElMessage.success('已保存')
    form.value.api_key = ''
    await load()
  } catch {
    // 拦截器已提示
  } finally {
    saving.value = false
  }
}

async function test() {
  testing.value = true
  try {
    const res: any = await aiApi.testConfig()
    ElMessage.success('连接成功：' + (res.data.reply || 'ok'))
  } catch {
    // 拦截器已提示
  } finally {
    testing.value = false
  }
}

// ---- 用量统计 ----

interface Stat {
  key: string
  name?: string
  calls: number
  tokens: number
  failed: number
}

const tab = ref('config')
const days = ref(7)
const usageLoading = ref(false)
const usage = ref<any>(null)

const SCENE_LABEL: Record<string, string> = {
  chat: 'AI 问答',
  explain: '题目解析',
  'generate-questions': '生成练习题',
  'generate-essay': '生成论文',
  'grade-essay': '论文评分',
  'grade-case': '案例判分',
  test: '连接测试',
}
const sceneLabel = (k: string) => SCENE_LABEL[k] || k

async function loadUsage() {
  usageLoading.value = true
  try {
    const res: any = await aiApi.usage(days.value)
    usage.value = res.data
  } catch {
    // 拦截器已提示
  } finally {
    usageLoading.value = false
  }
}

// 折线图用不上第三方库，按最大值折算高度即可
const maxDayTokens = computed(() =>
  Math.max(1, ...((usage.value?.by_day as Stat[]) || []).map((d) => d.tokens))
)

const logs = ref<any[]>([])
const logTotal = ref(0)
const logPage = ref(1)
const logScene = ref('')
const logLoading = ref(false)

async function loadLogs() {
  logLoading.value = true
  try {
    const res: any = await aiApi.callLogs({
      scene: logScene.value || undefined,
      page: logPage.value,
      page_size: 20,
    })
    logs.value = res.data.items || []
    logTotal.value = res.data.total || 0
  } catch {
    // 拦截器已提示
  } finally {
    logLoading.value = false
  }
}

watch(tab, (v) => {
  if (v === 'usage' && !usage.value) {
    loadUsage()
    loadLogs()
  }
})
</script>

<template>
  <div class="max-w-5xl">
    <h2 class="mb-1 text-xl font-bold text-gray-900 dark:text-gray-100">AI 管理</h2>
    <p class="mb-6 text-sm text-slate-500 dark:text-slate-400">OpenAI 兼容协议。可接 OpenAI、DeepSeek、通义千问、智谱、one-api 代理等。密钥仅存服务端，不回传前端。</p>

    <el-tabs v-model="tab">
      <el-tab-pane label="配置" name="config">
        <el-card class="max-w-2xl">
          <el-form label-width="110px" label-position="top">
            <el-form-item label="Base URL">
              <el-input v-model="form.base_url" placeholder="如 https://api.deepseek.com/v1 或 https://api.openai.com/v1" />
            </el-form-item>
            <el-form-item label="API Key">
              <el-input
                v-model="form.api_key"
                :type="showKey ? 'text' : 'password'"
                :placeholder="keySet ? `已配置（${keyMasked}），留空则保持不变` : '请输入 API Key'"
              >
                <template #suffix>
                  <div class="flex items-center gap-1">
                    <el-tooltip :content="showKey ? '隐藏' : '显示'" placement="top">
                      <el-button text size="small" @click="showKey = !showKey">
                        <el-icon><component :is="showKey ? 'Hide' : 'View'" /></el-icon>
                      </el-button>
                    </el-tooltip>
                    <el-tooltip content="复制" placement="top">
                      <el-button text size="small" @click="copyKey">
                        <el-icon><CopyDocument /></el-icon>
                      </el-button>
                    </el-tooltip>
                  </div>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item label="模型">
              <el-input v-model="form.model" placeholder="如 deepseek-chat / gpt-4o-mini / qwen-plus / claude-sonnet-4-6（经代理）" />
            </el-form-item>
            <el-form-item label="Temperature">
              <el-slider v-model="form.temperature" :min="0" :max="2" :step="0.1" show-input />
            </el-form-item>
            <el-form-item label="最大 Tokens">
              <el-input-number v-model="form.max_tokens" :min="256" :max="32768" :step="256" />
            </el-form-item>
            <el-form-item label="启用 AI 功能">
              <el-switch v-model="form.enabled" :active-value="1" :inactive-value="0" />
            </el-form-item>
            <el-form-item label="非会员免费试用次数">
              <el-input-number v-model="form.free_trial_limit" :min="0" :max="100" />
              <span class="ml-3 text-xs text-slate-400">每个非会员账号累计可用次数，0 = 关闭试用（AI 完全会员专属）</span>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="saving" @click="save">保存</el-button>
              <el-button :loading="testing" @click="test">测试连接</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="用量统计" name="usage">
        <div class="mb-4 flex items-center gap-3">
          <el-radio-group v-model="days" size="small" @change="loadUsage">
            <el-radio-button :value="7">近 7 天</el-radio-button>
            <el-radio-button :value="30">近 30 天</el-radio-button>
            <el-radio-button :value="90">近 90 天</el-radio-button>
          </el-radio-group>
          <el-button size="small" :loading="usageLoading" @click="loadUsage">刷新</el-button>
        </div>

        <div v-loading="usageLoading">
          <div v-if="usage" class="mb-5 grid grid-cols-2 gap-3 md:grid-cols-4">
            <el-card shadow="never" body-class="!py-4">
              <div class="text-xs text-slate-500">调用次数</div>
              <div class="mt-1 text-2xl font-bold">{{ usage.calls }}</div>
            </el-card>
            <el-card shadow="never" body-class="!py-4">
              <div class="text-xs text-slate-500">Token 总量</div>
              <div class="mt-1 text-2xl font-bold">{{ usage.tokens?.toLocaleString() }}</div>
            </el-card>
            <el-card shadow="never" body-class="!py-4">
              <div class="text-xs text-slate-500">失败次数</div>
              <div class="mt-1 text-2xl font-bold" :class="usage.failed > 0 ? 'text-red-500' : ''">{{ usage.failed }}</div>
            </el-card>
            <el-card shadow="never" body-class="!py-4">
              <div class="text-xs text-slate-500">使用人数</div>
              <div class="mt-1 text-2xl font-bold">{{ usage.users }}</div>
            </el-card>
          </div>

          <el-alert
            type="info"
            :closable="false"
            class="mb-5"
            title="流式调用（问答、解析、论文生成）多数上游不返回 token 计数，这类记录按字符估算并在日志里标注「估算」，仅供看趋势，不能直接当账单。自带 key 的会员调用不计入成本，已从「用量 Top」中排除。"
          />

          <div class="grid gap-4 md:grid-cols-2">
            <el-card shadow="never">
              <template #header><span class="text-sm font-semibold">按场景</span></template>
              <el-table :data="usage?.by_scene || []" size="small" empty-text="暂无数据">
                <el-table-column label="场景" min-width="120">
                  <template #default="{ row }: { row: any }">{{ sceneLabel(row.key) }}</template>
                </el-table-column>
                <el-table-column prop="calls" label="次数" width="80" />
                <el-table-column label="Tokens" width="110">
                  <template #default="{ row }: { row: any }">{{ row.tokens?.toLocaleString() }}</template>
                </el-table-column>
                <el-table-column prop="failed" label="失败" width="70" />
              </el-table>
            </el-card>

            <el-card shadow="never">
              <template #header><span class="text-sm font-semibold">用量 Top（不含自带 key）</span></template>
              <el-table :data="usage?.top_users || []" size="small" empty-text="暂无数据">
                <el-table-column label="用户" min-width="140">
                  <template #default="{ row }: { row: any }">{{ row.name || `#${row.key}` }}</template>
                </el-table-column>
                <el-table-column prop="calls" label="次数" width="80" />
                <el-table-column label="Tokens" width="110">
                  <template #default="{ row }: { row: any }">{{ row.tokens?.toLocaleString() }}</template>
                </el-table-column>
              </el-table>
            </el-card>
          </div>

          <el-card shadow="never" class="mt-4">
            <template #header><span class="text-sm font-semibold">每日 Token 消耗</span></template>
            <div v-if="usage?.by_day?.length" class="flex h-40 items-end gap-1">
              <div
                v-for="d in usage.by_day"
                :key="d.key"
                class="flex-1 rounded-t bg-blue-400/70 transition hover:bg-blue-500"
                :style="{ height: Math.max(2, (d.tokens / maxDayTokens) * 100) + '%' }"
                :title="`${d.key}：${d.calls} 次 / ${d.tokens.toLocaleString()} tokens`"
              />
            </div>
            <el-empty v-else description="暂无数据" :image-size="60" />
          </el-card>

          <el-card shadow="never" class="mt-4">
            <template #header>
              <div class="flex items-center justify-between">
                <span class="text-sm font-semibold">调用明细</span>
                <el-select
                  v-model="logScene"
                  size="small"
                  clearable
                  placeholder="全部场景"
                  style="width: 150px"
                  @change="logPage = 1; loadLogs()"
                >
                  <el-option v-for="(label, k) in SCENE_LABEL" :key="k" :label="label" :value="k" />
                </el-select>
              </div>
            </template>
            <el-table :data="logs" size="small" v-loading="logLoading" empty-text="暂无记录">
              <el-table-column label="时间" width="160">
                <template #default="{ row }: { row: any }">{{ new Date(row.created_at).toLocaleString('zh-CN') }}</template>
              </el-table-column>
              <el-table-column label="场景" width="110">
                <template #default="{ row }: { row: any }">{{ sceneLabel(row.scene) }}</template>
              </el-table-column>
              <el-table-column prop="user_id" label="用户" width="80" />
              <el-table-column prop="model" label="模型" min-width="130" show-overflow-tooltip />
              <el-table-column label="Tokens" width="120">
                <template #default="{ row }: { row: any }">
                  {{ row.total_tokens?.toLocaleString() }}
                  <el-tag v-if="row.estimated" size="small" type="info" class="ml-1">估算</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="耗时" width="80">
                <template #default="{ row }: { row: any }">{{ (row.duration_ms / 1000).toFixed(1) }}s</template>
              </el-table-column>
              <el-table-column label="状态" min-width="140">
                <template #default="{ row }: { row: any }">
                  <el-tag v-if="row.success" size="small" type="success">成功</el-tag>
                  <el-tooltip v-else :content="row.err_msg || '失败'" placement="top">
                    <el-tag size="small" type="danger">失败</el-tag>
                  </el-tooltip>
                  <el-tag v-if="row.own_key" size="small" class="ml-1">自带 key</el-tag>
                </template>
              </el-table-column>
            </el-table>
            <el-pagination
              v-if="logTotal > 20"
              class="mt-3 justify-end"
              layout="total, prev, pager, next"
              :total="logTotal"
              :page-size="20"
              v-model:current-page="logPage"
              @current-change="loadLogs"
            />
          </el-card>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>
