<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { aiApi } from '@/api'

const form = ref({
  base_url: '',
  api_key: '',
  model: '',
  temperature: 0.7,
  max_tokens: 2048,
  enabled: 0,
})
const keySet = ref(false)
const keyMasked = ref('')
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)

async function load() {
  try {
    const res: any = await aiApi.getMyConfig()
    const d = res.data
    form.value.base_url = d.base_url || ''
    form.value.model = d.model || ''
    form.value.temperature = Number(d.temperature) || 0.7
    form.value.max_tokens = d.max_tokens || 2048
    form.value.enabled = d.enabled || 0
    keySet.value = !!d.key_set
    keyMasked.value = d.key_masked || ''
  } catch {
    // 非会员会被拦截器引导至开通页
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function save() {
  // 与后端一致：启用时三项必须齐全，不允许只填 Base URL 借用平台的 Key
  if (form.value.enabled === 1 && (!form.value.base_url.trim() || !form.value.model.trim() || (!keySet.value && !form.value.api_key))) {
    ElMessage.warning('启用自定义 AI 需同时填写 Base URL、API Key 和模型')
    return
  }
  saving.value = true
  try {
    await aiApi.updateMyConfig({
      base_url: form.value.base_url.trim(),
      api_key: form.value.api_key,
      model: form.value.model.trim(),
      temperature: form.value.temperature,
      max_tokens: form.value.max_tokens,
      enabled: form.value.enabled,
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
    const res: any = await aiApi.testMyConfig()
    ElMessage.success('连接成功：' + (res.data.reply || 'ok'))
  } catch {
    // 拦截器已提示
  } finally {
    testing.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-2xl px-4 py-6">
    <h2 class="mb-1 text-xl font-bold text-gray-900 dark:text-gray-100">AI 配置</h2>
    <p class="mb-6 text-sm text-slate-500 dark:text-slate-400">
      会员专属：填入自己的大模型接口，启用后 AI 功能走你的配置。Base URL、API Key、模型需<strong>同时填写</strong>才会生效，
      缺任一项则整体使用平台默认。
    </p>

    <div v-if="loading" class="py-16 text-center text-gray-400">加载中...</div>

    <el-card v-else>
      <el-form label-width="110px" label-position="top">
        <el-form-item label="Base URL">
          <el-input v-model="form.base_url" placeholder="如 https://api.deepseek.com/v1 或 https://api.openai.com/v1" />        </el-form-item>
        <el-form-item label="API Key">
          <el-input v-model="form.api_key" type="password" show-password :placeholder="keySet ? `已配置（${keyMasked}），留空则保持不变` : '请输入 API Key'" />
        </el-form-item>
        <el-form-item label="模型">
          <el-input v-model="form.model" placeholder="如 deepseek-chat / gpt-4o-mini / qwen-plus" />
        </el-form-item>
        <el-form-item label="Temperature">
          <el-slider v-model="form.temperature" :min="0" :max="2" :step="0.1" show-input />
        </el-form-item>
        <el-form-item label="最大 Tokens">
          <el-input-number v-model="form.max_tokens" :min="256" :max="32768" :step="256" />
        </el-form-item>
        <el-form-item label="启用我的配置">
          <el-switch v-model="form.enabled" :active-value="1" :inactive-value="0" />
          <span class="ml-3 text-xs text-slate-400">开启后 AI 功能将走你自己的接口，不受平台开关限制</span>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="save">保存</el-button>
          <el-button :loading="testing" @click="test">测试连接</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>
