import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '@/api'
import { userApi } from '@/api'
import { useUserStore } from '@/stores/user'

export interface Subject {
  id: number
  name: string
  code: string
  sort_order: number
  has_essay: number
  has_sprint: number
  // 资格等级：1=高级 2=中级 3=初级（旧数据/异常值按 2 处理）
  level: number
}

// 等级组定义：显示顺序即 level 升序
const LEVEL_GROUPS = [
  { level: 1, name: '高级' },
  { level: 2, name: '中级' },
  { level: 3, name: '初级' },
]

export const useSubjectStore = defineStore('subject', () => {
  const subjects = ref<Subject[]>([])
  const currentSubjectId = ref<number>(Number(localStorage.getItem('subject_id')) || 1)
  // subjects 晚于 profile 返回时暂存服务端科目，loadSubjects 成功后补应用
  const pendingServerSubject = ref<number>(0)
  // 用户最近一次主动切科目的时间戳(ms)。setSubject 时打点;窗口内 applyServerSubject 跳过,
  // 防止服务端回滚刚切的新科目。
  // 注意:这是纯内存态,整页 reload 后归零 —— 它救不了「刷新后回滚」那个场景。
  // 真正保证刷新不回滚的是 flushPendingSync(刷新前先等 PUT 落定),本窗口只是 SPA 内的补充防线。
  // 跨设备同步走 login() 路径(login 后无最近切科目,applyServerSubject 正常生效),不受影响。
  let recentLocalChoiceAt = 0
  const RECENT_LOCAL_CHOICE_WINDOW = 30_000
  // 最近一次把科目偏好同步到服务端的请求。整页刷新前必须 await 它:reload 若抢在 PUT 完成前
  // 发生,服务端 subject_id 仍是旧值,重载后 router.beforeEach → restoreFromServer(旧值)
  // 会把刚切的新科目回滚掉。
  let pendingSync: Promise<void> | null = null

  async function loadSubjects() {
    try {
      const res: any = await api.get('/subjects')
      subjects.value = (res.data || []).map((s: any) => ({ ...s, level: Number(s.level) || 2 }))
      if (pendingServerSubject.value) {
        applyServerSubject(pendingServerSubject.value)
        pendingServerSubject.value = 0
      }
    } catch {}
  }

  function setSubject(id: number) {
    currentSubjectId.value = id
    localStorage.setItem('subject_id', String(id))
    recentLocalChoiceAt = Date.now()
    // 登录用户把选择同步到服务端（换设备登录后恢复）。fire-and-forget：
    // 失败静默，本地已生效；游客/未登录不同步
    const userStore = useUserStore()
    if (userStore.isLoggedIn) {
      // 记下这一次请求,供刷新前 flushPendingSync 等待。后一次切换会覆盖它 —— 永远只等最新一次。
      // 结果不关心(失败静默,本地已生效),这里只要一个可等待的句柄
      const p = userApi.updateSubject(id).then(
        () => {},
        () => {},
      )
      pendingSync = p
      p.then(() => {
        if (pendingSync === p) pendingSync = null
      })
    } else {
      pendingSync = null
    }
  }

  // 等最新一次科目同步落定(成功/失败/超时都返回,不会让刷新无限期挂起)。
  // 整页刷新前调用:确保服务端已经是用户刚选的科目,重载后 restoreFromServer 不会回滚。
  async function flushPendingSync(timeoutMs = 2000) {
    const deadline = Date.now() + timeoutMs
    while (pendingSync) {
      const p = pendingSync
      const left = deadline - Date.now()
      if (left <= 0) break
      await Promise.race([p, new Promise((r) => setTimeout(r, left))])
      // 等待期间用户又切了科目 → pendingSync 已被更新的一次覆盖,继续等最新那次
      if (pendingSync === p) break
    }
  }

  // 退出登录时清掉窗口标记。否则 30 秒内换账号登录,新用户的服务端科目会被窗口吞掉,
  // 直接继承上一个用户留在 localStorage 里的科目。
  function clearLocalChoiceWindow() {
    recentLocalChoiceAt = 0
  }

  // 登录/restore 后用服务端值恢复科目。仅当值在已加载科目列表里（存在且启用）
  // 且与本地不同才覆盖；服务端脏数据（科目已禁用）忽略，保持本地值
  function restoreFromServer(id?: number) {
    const sid = Number(id) || 0
    if (!sid) return
    // 窗口期内用户刚主动切过科目 → 本地选择优先,避免 reload 抢在 userApi.updateSubject
    // HTTP 完成前导致的回滚(用户看到"切了又回到旧科目")。
    if (Date.now() - recentLocalChoiceAt < RECENT_LOCAL_CHOICE_WINDOW) return
    if (!subjects.value.length) {
      pendingServerSubject.value = sid
      return
    }
    applyServerSubject(sid)
  }

  function applyServerSubject(sid: number) {
    // 兜底:即使 restoreFromServer 已放过窗口,直接调用也再校验一次
    if (Date.now() - recentLocalChoiceAt < RECENT_LOCAL_CHOICE_WINDOW) return
    if (subjects.value.some((s) => s.id === sid) && sid !== currentSubjectId.value) {
      currentSubjectId.value = sid
      localStorage.setItem('subject_id', String(sid))
    }
  }

  const currentSubject = () => subjects.value.find(s => s.id === currentSubjectId.value) || subjects.value[0]

  // 能力开关：从科目属性派生，新增科目由后台勾选，前端无需改 code 白名单。
  const hasEssay = computed(() => (currentSubject()?.has_essay ?? 0) === 1)
  const hasSprint = computed(() => (currentSubject()?.has_sprint ?? 0) === 1)

  // 按等级分组（对话框数据源）：恒定组顺序 1→2→3，空组剔除，组内按 sort_order
  const subjectsByLevel = computed(() =>
    LEVEL_GROUPS
      .map((g) => ({
        ...g,
        items: subjects.value
          .filter((s) => (s.level || 2) === g.level)
          .sort((a, b) => a.sort_order - b.sort_order),
      }))
      .filter((g) => g.items.length > 0)
  )

  return { subjects, currentSubjectId, loadSubjects, setSubject, restoreFromServer, flushPendingSync, clearLocalChoiceWindow, currentSubject, hasEssay, hasSprint, subjectsByLevel }
})
