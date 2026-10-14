import { createApp } from 'vue'
import { createPinia } from 'pinia'
// el-* 组件与其样式由 unplugin-vue-components 按需引入（见 vite.config.ts），
// 不再整包引 element-plus/dist/index.css。
// 但以下三类模板扫描不到，样式必须手工引：
//   ElMessage / ElMessageBox / ElNotification —— 命令式调用
//   v-loading —— 指令
//   base.css —— 各组件共用的基础样式
import 'element-plus/theme-chalk/base.css'
import 'element-plus/theme-chalk/el-message.css'
import 'element-plus/theme-chalk/el-message-box.css'
import 'element-plus/theme-chalk/el-notification.css'
import 'element-plus/theme-chalk/el-loading.css'
// EP 官方暗黑主题变量（html.dark 下生效）。主题色由 appearance.ts 以行内样式注入，
// 行内样式优先于这里的 html.dark 规则，自定义主题色在暗黑模式下不会被官方默认蓝覆盖。
import 'element-plus/theme-chalk/dark/css-vars.css'
import * as icons from './icons'
import App from './App.vue'
import router from './router'
import { useAppearance } from './stores/appearance'
import './style.css'

const app = createApp(App)

for (const [name, component] of Object.entries(icons)) {
  app.component(name, component)
}

app.use(createPinia())
app.use(router)

const appearance = useAppearance()
// 主题色变量须在 mount 前注入：Tailwind 的 blue-* 类编译成 rgb(var(--tw-blue-500-rgb) / ...)，
// 变量未设时这些类首屏无色。init() 同步执行，早于首屏渲染。
appearance.init()

app.mount('#app')

