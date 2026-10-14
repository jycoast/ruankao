import { useUserStore } from '@/stores/user'
import router from '@/router'
import { ElMessageBox } from 'element-plus'

// 游客(role 2)写/作答/入口守卫：游客触发「登录后使用」引导并去登录(带回当前页)；
// 非游客一律放行。用于在 提交/删除/重新做题/开始练习/带锁卡片 发起请求或跳转前拦下。
export function guardGuest(featureLabel: string): boolean {
  const userStore = useUserStore()
  if (!userStore.isGuest) return true
  ElMessageBox.confirm(`登录后即可使用「${featureLabel}」。`, '登录后使用', {
    confirmButtonText: '去登录',
    cancelButtonText: '暂不',
    type: 'warning',
  })
    .then(() => {
      router.push(loginGuestRoute())
    })
    .catch(() => {})
  return false
}

// 游客空态/零态页上的「去登录」直链(不弹确认框)：回到当前页，登录后继续。
export function loginGuestRoute() {
  return { path: '/login', query: { redirect: router.currentRoute.value.fullPath } }
}
