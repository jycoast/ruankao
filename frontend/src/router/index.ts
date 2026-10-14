import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { useSubjectStore } from '@/stores/subject'
import { useSiteStore } from '@/stores/site'
import { ElMessage, ElMessageBox } from 'element-plus'
import { nextTick } from 'vue'
import { isMaintenanceBypassPath } from '@/utils/maintenance'

// 站点标题后缀。页面在 afterEach 里按 meta.title 拼出完整 title�?
// SPA 不改 title 的话搜索结果里每一页都叫同一个名字?
const SITE_NAME = '软考星图'

// 游客可达白名单:四块只读内容 + 冲刺读页 + 「我的」个人数据页(空态)。
// Anki 背题(/review)与 AI 配置(/ai/my-config)是会员专属页,不进白名单,由守卫弹会员引导。
// 登录/注册/OAuth 回调等身份页放行:游客在此切真实登录,store 会用正式令牌覆盖游客令牌并清 token_kind,游客态随之结束。
//
// /maintenance 必须在这里:维护中游客先被维护块改道到 /maintenance,而
// isMaintenanceBypassPath 只让 /maintenance **跳过维护块**,并不豁免下面的游客块 ——
// 游客块会拿这张表再判一次。少了这一条,游客块把游客改道 /exams,维护块又把他扔回
// /maintenance,两个块互踢成无限重定向,渲染线程被微任务占满、导航永不提交、页面白屏
// (dev 构建的 vue-router 检测器也拦不住:它把计数挂在每轮新建的 location 对象上)。
// 两张表是**按顺序**先后被查的,编辑其中一张前先看另一张。
function isGuestAllowedPath(path: string): boolean {
  if (path === '/' || path === '/exams' || path.startsWith('/exams/')) return true
  if (path === '/questions' || path.startsWith('/questions/')) return true
  if (path === '/knowledge' || path.startsWith('/knowledge/')) return true
  if (path === '/essays' || /^\/essays\/\d+$/.test(path)) return true
  // 冲刺只读页:总览 + 计算题/单词/必考速记/答题套路(带 ?tab= 的 query 同命中)。模拟卷/AI 出题是作答/AI 入口,不在内。
  if (path === '/sprint' || path === '/sprint/calc' || path === '/sprint/words' || path === '/sprint/points' || path === '/sprint/skills') return true
  // 「我的」个人数据页:游客可看空态/零态(后端 uid0 GET 返回空 200)。写/收藏/删除按钮已由页面 guardGuest 拦截。
  if (path === '/report' || path === '/wrong' || path === '/favorites' || path === '/feedback') return true
  // 笔记主从页：详情是 /my/notes/:id，必须放行**子路径**。
  // 只精确匹配 /my/notes 的话，游客点开任一条笔记都会被改道 /exams，主从布局直接坏掉。
  if (path === '/my/notes' || path.startsWith('/my/notes/')) return true
  if (path === '/login' || path === '/register' || path.startsWith('/oauth/')) return true
  if (path === '/maintenance') return true
  return false
}

// 游客点到的会员专属「我的」页:返回用于引导的文案;非会员页返回空字符串(不改道)。
function guestMemberLabel(path: string): string {
  if (path === '/review') return 'Anki 背题'
  if (path === '/ai/my-config') return 'AI 配置'
  if (path === '/my/diagnosis') return 'AI 诊断报告'
  return ''
}

// 维护旁路谓词已抽到 @/utils/maintenance —— axios 拦截器也要用它。
// 两个消费者共用一份实现，避免「守卫放行、拦截器又把人弹走」的漂移。

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: () => import('@/components/layout/AppLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        { path: '', component: () => import('@/views/Home.vue') },
        { path: 'questions', component: () => import('@/views/question/QuestionList.vue') },
        { path: 'questions/:id', component: () => import('@/views/question/QuestionDetail.vue') },
        { path: 'practice', component: () => import('@/views/question/PracticeByCategory.vue') },
        { path: 'daily', component: () => import('@/views/daily/DailyPractice.vue'), meta: { requiresAuth: true } },
        { path: 'report', component: () => import('@/views/profile/StudyReport.vue'), meta: { requiresAuth: true } },
        { path: 'wrong', component: () => import('@/views/question/WrongQuestions.vue'), meta: { requiresAuth: true } },
        { path: 'review', component: () => import('@/views/question/ReviewMode.vue'), meta: { requiresAuth: true } },
        { path: 'favorites', component: () => import('@/views/question/Favorites.vue'), meta: { requiresAuth: true } },
        // 知识点与范文对未登录用户开放，供搜索引擎收录（后端同名路由已改 OptionalAuth�?
        { path: 'knowledge', component: () => import('@/views/knowledge/KnowledgeList.vue'), meta: { requiresAuth: false, title: '软考知识导图' } },
        { path: 'knowledge/:id', component: () => import('@/views/knowledge/KnowledgeDetail.vue'), meta: { requiresAuth: false } },
        { path: 'sprint', component: () => import('@/views/sprint/SprintHome.vue') },
        { path: 'sprint/calc', component: () => import('@/views/sprint/CalcTypes.vue') },
        { path: 'sprint/words', component: () => import('@/views/sprint/WordReview.vue') },
        { path: 'sprint/points', component: () => import('@/views/sprint/MustKnow.vue') },
        { path: 'sprint/skills', component: () => import('@/views/sprint/AnswerSkills.vue') },
        { path: 'sprint/mock-exam', component: () => import('@/views/sprint/MockExam.vue'), meta: { requiresAuth: true } },
        { path: 'sprint/ai-questions', component: () => import('@/views/ai/GenerateQuestions.vue') },
        { path: 'essays/ai-generate', component: () => import('@/views/ai/GenerateEssay.vue') },
        { path: 'essays/ai-grade', component: () => import('@/views/ai/GradeEssay.vue') },
        { path: 'ai/my-config', component: () => import('@/views/ai/MyAIConfig.vue'), meta: { requiresAuth: true } },
        { path: 'feedback', component: () => import('@/views/feedback/SubmitFeedback.vue'), meta: { requiresAuth: true } },
        { path: 'essays', component: () => import('@/views/essay/EssayList.vue'), meta: { requiresAuth: false, title: '软考论文范文' } },
        { path: 'essays/:id', component: () => import('@/views/essay/EssayDetail.vue'), meta: { requiresAuth: false } },
        { path: 'exams', component: () => import('@/views/exam/ExamList.vue') },
        { path: 'exams/:id', component: () => import('@/views/exam/ExamDetail.vue') },
        { path: 'exam/:id', component: () => import('@/views/exam/ExamSession.vue'), meta: { requiresAuth: true } },
        { path: 'exam/:id/review', component: () => import('@/views/exam/ExamReview.vue'), meta: { requiresAuth: true, title: '真题背题' } },
        { path: 'profile', component: () => import('@/views/profile/Profile.vue'), meta: { requiresAuth: true } },
        { path: 'change-password', component: () => import('@/views/profile/ChangePassword.vue'), meta: { requiresAuth: true } },
        { path: 'membership', component: () => import('@/views/membership/Membership.vue'), meta: { requiresAuth: true } },
        { path: 'membership/orders', component: () => import('@/views/membership/Orders.vue'), meta: { requiresAuth: true } },
        { path: 'essays/submit', component: () => import('@/views/essay/SubmitEssay.vue') },
        { path: 'essays/my-submissions', component: () => import('@/views/essay/MySubmissions.vue') },
        { path: 'my/notifications', component: () => import('@/views/my/Notifications.vue'), meta: { requiresAuth: true, title: '我的消息' } },
        { path: 'my/reports', component: () => import('@/views/my/MyReports.vue'), meta: { requiresAuth: true, title: '我的纠错' } },
        { path: 'my/feedbacks', component: () => import('@/views/my/MyFeedbacks.vue'), meta: { requiresAuth: true, title: '我的反馈' } },
        // :id 可选，且是**题目 id**（question_notes 对 (user_id, question_id) 唯一，
        // 对同一用户两者一一对应；用题目 id 是因为详情要拿它去取题面）。
        // 用单条可选参数路由而不是两条记录：/my/notes → /my/notes/5 不重新挂载组件，
        // 左侧列表已加载的数据和滚动位置得以保留，否则每次点笔记列表都重置回顶部。
        { path: 'my/notes/:id?', component: () => import('@/views/my/MyNotes.vue'), meta: { requiresAuth: true, title: '笔记' } },
        { path: 'my/diagnosis', component: () => import('@/views/profile/Diagnosis.vue'), meta: { requiresAuth: true, title: 'AI 诊断报告' } },
      ],
    },
    {
      path: '/admin',
      component: () => import('@/components/layout/AdminLayout.vue'),
      meta: { requiresAdmin: true },
      children: [
        { path: '', component: () => import('@/views/admin/Dashboard.vue') },
        { path: 'questions', component: () => import('@/views/admin/QuestionManage.vue') },
        { path: 'papers', component: () => import('@/views/admin/PaperManage.vue') },
        { path: 'users', component: () => import('@/views/admin/UserManage.vue') },
        { path: 'knowledge', component: () => import('@/views/admin/KnowledgeManage.vue') },
        { path: 'articles', component: () => import('@/views/admin/ArticleManage.vue') },
        { path: 'announcements', component: () => import('@/views/admin/AnnouncementManage.vue') },
        { path: 'exam-schedules', component: () => import('@/views/admin/ExamScheduleManage.vue'), meta: { title: '考试排期' } },
        { path: 'reports', component: () => import('@/views/admin/QuestionReportManage.vue') },
        { path: 'feedbacks', component: () => import('@/views/admin/FeedbackManage.vue') },
        { path: 'comments', component: () => import('@/views/admin/CommentManage.vue'), meta: { title: '评论管理' } },
        { path: 'comment-reports', component: () => import('@/views/admin/CommentReportManage.vue'), meta: { title: '举报处理' } },
        { path: 'essay-submissions', component: () => import('@/views/admin/EssaySubmissionManage.vue') },
        { path: 'invite-codes', component: () => import('@/views/admin/InviteCodeManage.vue') },
        { path: 'card-keys', component: () => import('@/views/admin/CardKeyManage.vue') },
        { path: 'membership-plans', component: () => import('@/views/admin/MembershipPlanManage.vue') },
        { path: 'orders', component: () => import('@/views/admin/OrderManage.vue'), meta: { title: '订单管理' } },
        { path: 'password-resets', component: () => import('@/views/admin/PasswordResetManage.vue') },
        { path: 'ai', component: () => import('@/views/admin/AIConfigManage.vue') },
      ],
    },
    // 放在顶层而不是 AppLayout 的子路由里：维护页不该带导航栏——点导航会立刻被弹回来，看着像坏了。
    { path: '/maintenance', component: () => import('@/views/Maintenance.vue'), meta: { requiresAuth: false, title: '系统维护中' } },
    { path: '/login', component: () => import('@/views/auth/Login.vue'), meta: { title: '登录' } },
    { path: '/register', component: () => import('@/views/auth/Register.vue'), meta: { title: '注册' } },
    { path: '/oauth/callback', component: () => import('@/views/auth/OAuthCallback.vue') },
    // history 模式下任意路径都会命�?index.html，没有兜底路由就是一片空白�?
    { path: '/:pathMatch(.*)*', component: () => import('@/views/NotFound.vue'), meta: { title: '页面不存在' } },
  ],
  // 切换路由回到顶部；浏览器前进/后退恢复原位�?
  scrollBehavior(_to, _from, saved) {
    return saved || { top: 0 }
  },
})

// 懒加载 chunk 加载失败的兜底：发版后旧 hash 资源从服务器移除（或 SW activate 清掉旧缓存
// 瞬间、另一标签页仍跑旧代码去请求旧 chunk），动态 import 会 404 报错，路由直接白屏。
// 这里捕获后整页重载到目标路由，拉取最新 index.html 与新资源。10 秒内只重载一次
// （sessionStorage 记时间戳，跨整页重载仍有效），避免部署异常时陷入刷新死循环。
const CHUNK_RELOAD_KEY = 'chunk_reload_at'
router.onError((error, to) => {
  const msg = String(error?.message || '')
  const isChunkError =
    error?.name === 'ChunkLoadError' ||
    /Failed to fetch dynamically imported module|Importing a module script failed|error loading dynamically imported module|Loading chunk .* failed/i.test(msg)
  if (!isChunkError) return
  const now = Date.now()
  if (now - Number(sessionStorage.getItem(CHUNK_RELOAD_KEY) || 0) < 10_000) return
  sessionStorage.setItem(CHUNK_RELOAD_KEY, String(now))
  // history 路由：目标路径直接整页导航（部署侧已有 SPA fallback 把未知路径落回 index.html），
  // 新页面里由 index.html 重新拉起应用
  window.location.assign(to.fullPath)
})

// resolveMeta 取最靠近叶子的那个显式值�?
// to.matched.some() 只要父路由标�?requiresAuth 就恒�?true�?
// 子路由无法把自己声明成公开�?—�?而知识点/范文正需要这么做�?
function resolveMeta<T>(to: { matched: { meta: Record<string, unknown> }[] }, key: string): T | undefined {
  for (let i = to.matched.length - 1; i >= 0; i--) {
    const v = to.matched[i].meta[key]
    if (v !== undefined) return v as T
  }
  return undefined
}

router.beforeEach(async (to, _from, next) => {
  const userStore = useUserStore()
  // 刷新页面后有 token 但用户信息未加载 �?先恢复（�?role，供管理员校验）
  if (userStore.token && !userStore.user) {
    await userStore.restore()
  }

  // —— 维护模式 ——
  // 放在身份恢复之后：判 role 要用它。放在所有业务判定之前：维护中不必再算科目能力。
  if (!isMaintenanceBypassPath(to.path) && userStore.user?.role !== 1) {
    const siteStore = useSiteStore()
    if (await siteStore.fetchMaintenance()) {
      next('/maintenance')
      return
    }
  }

  const requiresAuth = resolveMeta<boolean>(to, 'requiresAuth') === true
  const requiresAdmin = resolveMeta<boolean>(to, 'requiresAdmin') === true

  if (requiresAdmin && (!userStore.isLoggedIn || userStore.user?.role !== 1)) {
    next('/login')
    return
  }
  if (requiresAuth && !userStore.isLoggedIn) {
    // 带上来源，登录后回到原页面，而不是一律甩回首�?
    next({ path: '/login', query: { redirect: to.fullPath } })
    return
  }

  // 游客(role 2):白名单内容放行;会员工具(/review、/ai/my-config)弹开通引导;其余改道真题列表
  if (userStore.isLoggedIn && userStore.user?.role === 2) {
    const memberLabel = guestMemberLabel(to.path)
    if (memberLabel) {
      ElMessageBox.confirm(`「${memberLabel}」为会员专属功能，登录并开通会员后即可使用。`, '会员专属功能', {
        confirmButtonText: '去登录',
        cancelButtonText: '暂不',
        type: 'warning',
      })
        .then(() => {
          // 登录后直通会员页,保住转化漏斗(与匿名会员引导一致)
          next({ path: '/login', query: { redirect: '/membership' } })
        })
        .catch(() => {
          next(false)
        })
      return
    }
    if (!isGuestAllowedPath(to.path)) {
      next('/exams')
      return
    }
  }

  // 接科目能力裁剪入口：无对应功能的科目直接回首页，避免看到不相关内容。
  const subjectStore = useSubjectStore()
  // 刷新直接落在 /sprint 或 /essays 时，subjects 还没加载（loadSubjects 在 AppLayout
  // onMounted 里才调），此刻读 hasSprint/hasEssay 是空列表算出的 false，
  // 会把系统架构设计师这类有论文功能的科目误判成"该科目暂无论文功能"踢回首页。
  // 守卫里先等一次加载完成再判定。
  const needCapability = to.path.startsWith('/sprint') || to.path.startsWith('/essays')
  if (needCapability && !subjectStore.subjects.length) {
    await subjectStore.loadSubjects()
  }
  if (to.path.startsWith('/sprint') && !subjectStore.hasSprint) {
    next('/')
    nextTick(() => ElMessage.info('该科目暂无冲刺资料'))
    return
  }
  if (to.path.startsWith('/essays') && !subjectStore.hasEssay) {
    next('/')
    nextTick(() => ElMessage.info('该科目暂无论文功能'))
    return
  }

  next()
})

// 每页独立 title：SPA 不改的话搜索结果里所有页面都叫同一个名�?
// 兜底文案与 index.html 的 <title> 保持一致，且不再写死单一科目（站点已覆盖多个科目）
router.afterEach((to) => {
  const title = resolveMeta<string>(to, 'title')
  document.title = title ? `${title} - ${SITE_NAME}` : `${SITE_NAME} - 软考知识导图与论文范文`
})

export default router
