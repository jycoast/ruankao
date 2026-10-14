import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { authApi, userApi, membershipApi } from '@/api'
import type { User, UserProfile } from '@/types'
import { useSubjectStore } from '@/stores/subject'
import router from '@/router'

// 游客标记:api 402 拦截器靠 token_kind 区分游客(guest)与正式用户
const GUEST_FLAG = 'token_kind'

interface MembershipState {
  is_member: boolean
  expire_at: string | null
  plan: string
  days_left: number
}

export const useUserStore = defineStore('user', () => {
  const token = ref(localStorage.getItem('token') || '')
  const user = ref<User | null>(null)
  const profile = ref<UserProfile | null>(null)
  const hasPassword = ref(true)
  const membership = ref<MembershipState | null>(null)
  const isLoggedIn = ref(!!localStorage.getItem('token'))
  const isMember = computed(() => !!membership.value?.is_member)

  async function fetchMembership() {
    try {
      const res: any = await membershipApi.status()
      membership.value = res.data
    } catch {
      membership.value = null
    }
  }

  // 登录后回到来源页。只接受本站相对路径 —— 直接用 query 里的值会变成开放重定向，
  // 攻击者可以拿 /login?redirect=https://evil.com 做钓鱼跳板。
  function redirectAfterLogin() {
    const raw = router.currentRoute.value.query.redirect
    const target = typeof raw === 'string' ? raw : ''
    if (target.startsWith('/') && !target.startsWith('//')) {
      router.replace(target)
    } else {
      router.push('/')
    }
  }

  async function login(username: string, password: string) {
    const res: any = await authApi.login({ username, password })
    token.value = res.data.token
    localStorage.setItem('token', res.data.token)
    localStorage.removeItem(GUEST_FLAG) // 换成正式账号,清游客标记
    isLoggedIn.value = true
    await fetchProfile()
    redirectAfterLogin()
  }

  async function register(username: string, email: string, password: string, nickname?: string, invite_code?: string, captcha_id?: string, captcha_code?: string) {
    const res: any = await authApi.register({ username, email, password, nickname, invite_code: invite_code || '', captcha_id: captcha_id || '', captcha_code: captcha_code || '' })
    token.value = res.data.token
    localStorage.setItem('token', res.data.token)
    localStorage.removeItem(GUEST_FLAG) // 换成正式账号,清游客标记
    isLoggedIn.value = true
    await fetchProfile()
    redirectAfterLogin()
  }

  async function fetchProfile() {
    try {
      const res: any = await userApi.getProfile()
      user.value = res.data.user
      profile.value = res.data.profile
      hasPassword.value = res.data.has_password
      // 科目偏好以服务端为准（用户考试科目不常变，跨设备一致）
      useSubjectStore().restoreFromServer(res.data.user?.subject_id)
      fetchMembership()
    } catch {
      logout()
    }
  }

  function logout() {
    token.value = ''
    user.value = null
    profile.value = null
    membership.value = null
    isLoggedIn.value = false
    localStorage.removeItem('token')
    localStorage.removeItem(GUEST_FLAG)
    // 清掉「刚主动切过科目」的窗口。否则退出后 30 秒内换账号登录,
    // 新用户的服务端科目会被窗口吞掉,直接继承上一个用户留在本地的科目。
    useSubjectStore().clearLocalChoiceWindow()
    router.push('/login')
  }

  async function loginWithToken(tokenStr: string) {
    token.value = tokenStr
    localStorage.setItem('token', tokenStr)
    localStorage.removeItem(GUEST_FLAG) // 换成正式账号,清游客标记
    isLoggedIn.value = true
    await fetchProfile()
    router.push('/')
  }

  // 游客体验:后端发 role=2 虚拟令牌,不落库;本地构造游客 user,不拉 /user/profile
  async function guestLogin() {
    const res: any = await authApi.guest()
    token.value = res.data.token
    localStorage.setItem('token', res.data.token)
    localStorage.setItem(GUEST_FLAG, 'guest')
    user.value = { id: 0, username: '游客', nickname: '游客', role: 2 } as User
    membership.value = null
    isLoggedIn.value = true
    redirectAfterLogin() // 无 redirect 时回 '/',由路由守卫改道 /exams
  }

  const isGuest = computed(() => user.value?.role === 2)

  // 刷新页面后用本地 token 恢复登录态（含 user，用于管理员校验）。
  // 失败时只清状态、不跳转，跳转交由路由守卫决定。
  async function restore() {
    if (!token.value) return
    // 游客令牌:不拉 profile(uid=0 后端回 401),本地构造游客 user,靠 token_kind 标记识别
    if (localStorage.getItem(GUEST_FLAG) === 'guest') {
      user.value = { id: 0, username: '游客', nickname: '游客', role: 2 } as User
      isLoggedIn.value = true
      return
    }
    try {
      const res: any = await userApi.getProfile()
      user.value = res.data.user
      profile.value = res.data.profile
      hasPassword.value = res.data.has_password
      isLoggedIn.value = true
      // 科目偏好以服务端为准（用户考试科目不常变，跨设备一致）
      useSubjectStore().restoreFromServer(res.data.user?.subject_id)
      fetchMembership()
    } catch {
      token.value = ''
      user.value = null
      profile.value = null
      membership.value = null
      isLoggedIn.value = false
      localStorage.removeItem('token')
    }
  }

  return { token, user, profile, hasPassword, membership, isMember, isLoggedIn, login, register, fetchProfile, fetchMembership, logout, loginWithToken, guestLogin, isGuest, restore }
})
