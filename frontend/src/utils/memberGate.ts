import { useUserStore } from '@/stores/user'
import router from '@/router'
import { ElMessageBox } from 'element-plus'

// —— 会员权益注册表：改权益只改这里，一处生效 ——
// enabled: true=会员专属（未达会员弹引导）；false=免费放行（将来开放免费不必删代码）
export const memberGates = {
  pdfExport: { enabled: true, name: '真题解析 PDF 导出' },
  recentExam: { enabled: true, name: '近三年真题练习与模拟考试' },
  knowledgeExport: { enabled: true, name: '知识点导出' }, // 图片长图 + PDF（一章一页）
  diagnosis: { enabled: true, name: 'AI 诊断报告' }, // 错题智能诊断 + 薄弱点画像
  // 以后新功能在此加一行即可接入门禁
} as const
export type MemberFeature = keyof typeof memberGates

// 通用门禁：已开通 → true 直接放行；未达 → 弹引导弹窗，返回 false
export function requireMember(feature: MemberFeature): boolean {
  const cfg = memberGates[feature]
  if (!cfg.enabled) return true // 权益调成免费，此函数自动放行
  const userStore = useUserStore()
  if (userStore.isMember) return true
  const isRealUser = userStore.isLoggedIn && userStore.user?.role !== 2 // 真实登录用户(游客视同未登录)
  const isGuest = userStore.isLoggedIn && userStore.user?.role === 2
  ElMessageBox.confirm(
    isRealUser
      ? `${cfg.name}为会员专属功能，开通后即可使用。`
      : `登录并开通会员后，即可使用${cfg.name}。`,
    '会员专属功能',
    {
      confirmButtonText: isRealUser ? '去开通' : '去登录/开通',
      cancelButtonText: '暂不',
      type: 'warning',
    },
  )
    .then(() => {
      if (isRealUser) {
        router.push('/membership')
      } else if (isGuest) {
        // 游客:登录后回到刚才浏览的页面(store 校验同站相对路径后 replace 跳回)
        router.push({ path: '/login', query: { redirect: router.currentRoute.value.fullPath } })
      } else {
        // 匿名:登录后直通会员页,保住转化链路
        router.push({ path: '/login', query: { redirect: '/membership' } })
      }
    })
    .catch(() => {})
  return false
}

// 近三年真题练习/考试为会员专属：year >= 当前年-2（动态滚动，前后端同规则）
export function isRecentPaperYear(year: number): boolean {
  return year >= new Date().getFullYear() - 2
}
