import axios from 'axios'
import type { AxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import { isMaintenanceBypassPath } from '@/utils/maintenance'

// 扩展 Axios 配置：silent=true 时，业务错误（含 402 会员拦截）不弹窗、不自动跳转，交由调用方处理
declare module 'axios' {
  export interface AxiosRequestConfig {
    silent?: boolean
  }
}

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 10000,
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 登录态失效处理。后端所有业务错误都走 HTTP 200 + body.code，
// 所以 401 只能从 body 里判，不能靠 error.response.status。
// 并发请求会同时拿到 401，用标志位保证只提示一次、只跳转一次。
let authExpiredHandled = false

function handleAuthExpired(message?: string) {
  if (authExpiredHandled) return
  authExpiredHandled = true
  localStorage.removeItem('token')
  // history 路由下按 pathname 判断；已在登录页就不用再跳
  if (window.location.pathname.startsWith('/login')) return
  ElMessage.warning(message || '登录已过期，请重新登录')
  // 整页跳转而非 router.push：Pinia 里的 token/user 残留会让守卫误判成已登录。
  // redirect 取当前路径（path+query），登录后经 userStore.redirectAfterLogin 回到原页面。
  const current = window.location.pathname + window.location.search
  window.location.href = `/login?redirect=${encodeURIComponent(current)}`
}

api.interceptors.response.use(
  (response) => {
    const data = response.data
    const cfg: any = response.config || {}
    if (data.code !== 0) {
      // 401：token 过期或被踢，清 token 并跳登录（早于弹窗，避免满屏报错）
      if (data.code === 401) {
        handleAuthExpired(data.message)
        return Promise.reject(data)
      }
      // 503：系统维护中。守卫只在导航时触发，拦不住「用户已经停在页面里、
      // 维护突然开启」，所以这一层是必需的，不是冗余。
      // 必须同时看 data.maintenance 判别位：业务码 503 不是维护专属，
      // /api/upload/image 在对象存储未配置时也回 503（「图片上传未启用」）。
      // 只看 code 的话，普通用户传一张笔记图片就会被硬弹到维护页、还被告知站点挂了。
      // 裸 503 落回下面的普通错误提示分支。
      // 旁路路径不改道（复用守卫那份 isMaintenanceBypassPath）：登录页外壳会拉
      // /exam-schedules，它不在后端白名单里，维护期间必然回 503 —— 若不豁免，管理员
      // 点维护页上的「管理员登录」会被立刻弹回维护页，永远登不进来。调用方的 .catch
      // 也救不了：本拦截器在 reject 之前就已经整页跳转了。
      // 整页跳转而非 router.push：与 401 同理，避免 Pinia 残留态干扰维护页。
      if (data.code === 503 && data.data?.maintenance === true) {
        if (isMaintenanceBypassPath(window.location.pathname)) {
          // 旁路页面（登录页等）不跳转 —— 但必须给出反馈：503 分支在下面的
          // ElMessage.error 之前就 return 了，不补这一句，「游客模式试用」
          // 「找回密码」这些非白名单入口在维护期间点了会毫无反应
          //（Login.vue 的 catch 写着「拦截器已提示」，它依赖的就是这里）。
          // 静默调用方（silent）除外，见下。
          if (!cfg.silent) ElMessage.error(data.message)
        } else {
          window.location.assign('/maintenance')
        }
        return Promise.reject(data)
      }
      if (!cfg.silent) ElMessage.error(data.message)
      // 402：会员专属功能。真用户引导去开通；游客(无账号)引导登录,不跳 /membership
      if (data.code === 402 && !cfg.silent && !window.location.pathname.startsWith('/membership')) {
        const isGuest = localStorage.getItem('token_kind') === 'guest'
        if (isGuest) {
          ElMessage.warning('该内容需登录账号，登录后可开通会员使用')
        } else {
          // 整页跳转而非 import router：api 是 store 的依赖，反向引入 router 会成环
          window.location.assign('/membership')
        }
      }
      return Promise.reject(data)
    }
    return data
  },
  (error) => {
    // 兜底：网关/反代直接返回的 401（未经过业务层）
    if (error.response?.status === 401) {
      handleAuthExpired()
    } else if (error.response?.status === 429) {
      ElMessage.warning('操作过于频繁，请稍后再试')
    } else {
      ElMessage.error(error.response?.data?.message || '网络错误')
    }
    return Promise.reject(error)
  }
)

export const authApi = {
  register: (data: { username: string; email: string; password: string; nickname?: string; invite_code: string; captcha_id: string; captcha_code: string }) =>
    api.post('/auth/register', data),
  login: (data: { username: string; password: string }) =>
    api.post('/auth/login', data),
  getCaptcha: () => api.get('/auth/captcha'),
  // 游客模式：服务端建 role-2 虚拟账号并返回 token
  guest: () => api.post('/auth/guest'),
}

export const userApi = {
  getProfile: () => api.get('/user/profile'),
  updateProfile: (data: { nickname?: string; avatar?: string; phone?: string }) =>
    api.put('/user/profile', data),
  // 科目偏好跨设备同步。silent：失败不弹窗，本地切换已生效，下次切换重试
  updateSubject: (subjectId: number) =>
    api.put('/user/subject', { subject_id: subjectId }, { silent: true }),
  getStats: () => api.get('/user/stats'),
  getReport: (subjectId: number) => api.get('/user/report', { params: { subject_id: subjectId } }),
  changePassword: (data: { old_password: string; new_password: string }) =>
    api.put('/user/password', data),
}

export const dailyApi = {
  today: (subjectId: number) => api.get('/daily/today', { params: { subject_id: subjectId } }),
  checkin: (data: { subject_id: number; question_count: number; correct_count: number }) =>
    api.post('/daily/checkin', data),
  saveSession: (data: any) => api.post('/daily/session', data, { silent: true }),
}

export const questionApi = {
  list: (params: { page?: number; page_size?: number; question_type?: number; category_id?: number; year?: number; difficulty?: number; keyword?: string; subject_id?: number }) =>
    api.get('/questions', { params }),
  getById: (id: number) => api.get(`/questions/${id}`),
  getCategories: (params?: { subject_id?: number }) => api.get('/questions/categories', { params }),
  create: (data: any) => api.post('/admin/questions', data),
  update: (id: number, data: any) => api.put(`/admin/questions/${id}`, data),
  delete: (id: number) => api.delete(`/admin/questions/${id}`),
  sort: (ids: number[]) => api.post('/admin/questions/sort', { ids }),
}

export const practiceApi = {
  submitAnswer: (data: {
    question_id: number
    user_answer: string
    practice_type: number
    time_spent?: number
    // 分类刷题传这三项，后端据此把作答记进「继续刷题」的草稿
    subject_id?: number
    question_type?: number
    knowledge_point?: string
  }) => api.post('/practice/submit', data),
  // fresh=true 从头开始（清空上次答题记录），false 继续上次的进度
  startPractice: (data: {
    subject_id: number
    question_type: number
    knowledge_point: string
    fresh: boolean
  }) => api.post('/practice/start', data),
  sessionState: (params: { subject_id: number; question_type: number; knowledge_point: string }) =>
    api.get('/practice/session', { params }),
  getWrongQuestions: (params?: { status?: number; page?: number; page_size?: number }) =>
    api.get('/practice/wrong', { params }),
  deleteWrongQuestion: (questionId: number) =>
    api.delete(`/practice/wrong/${questionId}`),
  // silent 是 axios config（首页静默刷新用），page/page_size 走查询参数，两者分开放
  getDueReview: (params?: { page?: number; page_size?: number; silent?: boolean }) => {
    const { silent, ...query } = params || {}
    return api.get('/practice/review', { params: query, silent })
  },
  review: (data: { question_id: number; is_correct: boolean }) =>
    api.post('/practice/review', data),
  getCategoryProgress: () => api.get('/practice/category-progress'),
}

// 会员专属「AI 诊断报告」(/my/diagnosis)。AI 流式端点不走这里，直接用 streamPost。
export const diagnosisApi = {
  summary: (subjectId: number) => api.get('/diagnosis/summary', { params: { subject_id: subjectId } }),
}

export const favoriteApi = {
  add: (data: { target_type: number; target_id: number }) =>
    api.post('/favorites', data),
  remove: (targetType: number, targetId: number) =>
    api.delete(`/favorites/${targetType}/${targetId}`),
  list: (params?: { target_type?: number; page?: number; page_size?: number }) =>
    api.get('/favorites', { params }),
  check: (params: { target_type: number; target_id: number }) =>
    api.get('/favorites/check', { params }),
}

export const knowledgeApi = {
  list: (params?: { category_id?: number; page?: number; page_size?: number; subject_id?: number }) =>
    api.get('/knowledge', { params }),
  // 后台整树管理专用：GET /admin/knowledge，page_size 上限 1000。
  // 公开接口 page_size 被钳到 50（管理员 200），超过即静默截断尾部节点——
  // 残缺快照组树会把丢父级的子节点顶成顶级节点，拖拽提交还会固化截断结果
  adminList: (params?: { category_id?: number; page?: number; page_size?: number; subject_id?: number }) =>
    api.get('/admin/knowledge', { params }),
  // 公开接口分页拉全量（普通用户每页最多 50）：循环翻页直到取满 total。
  // 前台导图/列表/导出/AI 出题都按全量组树，必须用它而不是单次 list
  listAll: async (subject_id?: number) => {
    const out: any[] = []
    let page = 1
    for (;;) {
      const res: any = await knowledgeApi.list({ page, page_size: 50, subject_id })
      const items = res.data?.items || []
      out.push(...items)
      const total = Number(res.data?.total || 0)
      if (!items.length || out.length >= total) break
      page++
    }
    return out
  },
  getById: (id: number) => api.get(`/knowledge/${id}`),
  relatedQuestions: (id: number, limit = 20) =>
    api.get(`/knowledge/${id}/questions`, { params: { limit } }),
  create: (data: any) => api.post('/admin/knowledge', data),
  update: (id: number, data: any) => api.put(`/admin/knowledge/${id}`, data),
  delete: (id: number) => api.delete(`/admin/knowledge/${id}`),
  // base_version 为页面加载时的树版本（MAX updated_at）：期间被其他管理员改过，
  // 后端返回 409 拒绝整树覆写。silent 由页面自行区分冲突/失败提示；
  // 整树提交节点多，单独放宽超时到 30s（默认 10s 在高延迟链路下不够）
  reorder: (items: { id: number; parent_id: number; sort_order: number }[], base_version?: string) =>
    api.put('/admin/knowledge/reorder', { items, base_version }, { silent: true, timeout: 30000 }),
}

export const articleApi = {
  list: (params?: { article_type?: number; page?: number; page_size?: number; subject_id?: number; keyword?: string; category?: string }) =>
    api.get('/articles', { params }),
  getById: (id: number) => api.get(`/articles/${id}`),
  create: (data: any) => api.post('/admin/articles', data),
  update: (id: number, data: any) => api.put(`/admin/articles/${id}`, data),
  delete: (id: number) => api.delete(`/admin/articles/${id}`),
}

// 后台编辑器图片上传。不设 Content-Type，交给浏览器带 boundary。
// 图片可能较大，超时放宽到 60s（api 实例默认 10s 会误杀正常上传）。
export const uploadApi = {
  image: (file: File | Blob, filename = 'paste.png') => {
    const fd = new FormData()
    fd.append('file', file, filename)
    return api.post('/admin/upload/image', fd, { timeout: 60000 })
  },
  // 笔记图片上传：走 /api/upload/image（普通用户通道），subDir 固定为 note。
  // 与 admin 通道隔开子目录，避免互相覆盖 + 让管理员审核范围更聚焦。
  noteImage: (file: File | Blob, filename = 'note.png') => {
    const fd = new FormData()
    fd.append('file', file, filename)
    return api.post('/upload/image', fd, { timeout: 60000 })
  },
}

export const examApi = {
  listPapers: (params?: { paper_type?: number; page?: number; page_size?: number; subject_id?: number }) =>
    api.get('/exams/papers', { params }),
  getPaper: (id: number) => api.get(`/exams/papers/${id}`),
  startExam: (paperId: number) => api.post(`/exam/start/${paperId}`),
  submitExam: (data: { exam_record_id: number; answers: { question_id: number; answer: string }[] }) =>
    api.post('/exam/submit', data),
  generateMock: (data: {
    subject_id?: number
    years?: number
    single?: number
    multiple?: number
    case?: number
    essay?: number
  }) => api.post('/exam/generate-mock', data),
  // 真题练习模式断点草稿（服务端持久化）。silent：失败不弹错误条，best-effort
  practiceSessionState: (params: { paper_id: number; mode: string }) =>
    api.get('/exam/practice-session', { params, silent: true }),
  savePracticeSession: (data: {
    paper_id: number
    mode: string
    answers: Record<number, any>
    practiceResults: Record<number, any>
    currentIndex: number
  }) => api.post('/exam/practice-session', data, { silent: true }),
  clearPracticeSession: (params: { paper_id: number; mode: string }) =>
    api.delete('/exam/practice-session', { params, silent: true }),
  // 管理端：试卷管理（仅真题）
  adminList: (params?: { paper_type?: number; year?: number; keyword?: string; page?: number; page_size?: number; subject_id?: number }) =>
    api.get('/admin/papers', { params }),
  adminCreate: (data: {
    subject_id?: number
    title: string
    year: number
    batch?: number
    paper_type: number
    total_score?: number
    duration?: number
    description?: string
    status?: number
  }) => api.post('/admin/papers', data),
  adminUpdate: (id: number, data: {
    subject_id?: number
    title?: string
    year?: number
    batch?: number
    paper_type?: number
    total_score?: number
    duration?: number
    description?: string
    status?: number
  }) => api.put(`/admin/papers/${id}`, data),
  adminDelete: (id: number) => api.delete(`/admin/papers/${id}`),
  adminQuestions: (id: number) => api.get(`/admin/papers/${id}/questions`),
  adminReplaceQuestions: (id: number, questionIds: number[]) =>
    api.put(`/admin/papers/${id}/questions`, { question_ids: questionIds }),
}

export const adminApi = {
  listUsers: (params?: { page?: number; page_size?: number; keyword?: string; role?: number; status?: number }) =>
    api.get('/admin/users', { params }),
  updateUserStatus: (id: number, status: number) =>
    api.put(`/admin/users/${id}/status`, { status }),
  updateUserRole: (id: number, role: number) =>
    api.put(`/admin/users/${id}/role`, { role }),
  resetUserPassword: (id: number, newPassword: string) =>
    api.put(`/admin/users/${id}/password`, { new_password: newPassword }),
  getDashboard: () => api.get('/admin/dashboard'),
  getOnline: () => api.get('/admin/online'),
  getSiteSettings: () => api.get('/admin/site-settings'),
  updateSiteSettings: (enabled: boolean) =>
    api.put('/admin/site-settings', { maintenance_enabled: enabled }),
}

export const announcementApi = {
  // 用户端：未读公告 / 标记已读
  listUnread: () => api.get('/announcements'),
  markRead: (id: number) => api.post(`/announcements/${id}/read`),
  // 管理端
  adminList: (params?: { page?: number; page_size?: number }) =>
    api.get('/admin/announcements', { params }),
  create: (data: { title: string; content: string }) =>
    api.post('/admin/announcements', data),
  update: (id: number, data: { title?: string; content?: string; status?: number }) =>
    api.put(`/admin/announcements/${id}`, data),
  delete: (id: number) => api.delete(`/admin/announcements/${id}`),
}

// 考试排期：公开只读供倒计时使用，管理端 CRUD
export const examScheduleApi = {
  list: () => api.get('/exam-schedules'),
  adminList: () => api.get('/admin/exam-schedules'),
  create: (data: { label: string; exam_date: string }) =>
    api.post('/admin/exam-schedules', data),
  update: (id: number, data: { label: string; exam_date: string }) =>
    api.put(`/admin/exam-schedules/${id}`, data),
  delete: (id: number) => api.delete(`/admin/exam-schedules/${id}`),
}

export const reportApi = {
  submit: (questionId: number, data: { report_type: number; description: string }) =>
    api.post(`/questions/${questionId}/report`, data),
  mine: (params?: { page?: number; page_size?: number }) =>
    api.get('/questions/my-reports', { params }),
  adminList: (params?: { page?: number; page_size?: number; status?: number }) =>
    api.get('/admin/reports', { params }),
  updateStatus: (id: number, data: { status: number; admin_note?: string }) =>
    api.put(`/admin/reports/${id}`, data),
}

export const feedbackApi = {
  submit: (data: { feedback_type: number; content: string; contact?: string }) =>
    api.post('/feedbacks', data),
  mine: (params?: { page?: number; page_size?: number }) =>
    api.get('/feedbacks/mine', { params }),
  adminList: (params?: { page?: number; page_size?: number; status?: number; type?: number; keyword?: string }) =>
    api.get('/admin/feedbacks', { params }),
  updateStatus: (id: number, data: { status: number; admin_note?: string }) =>
    api.put(`/admin/feedbacks/${id}`, data),
}

export const notificationApi = {
  list: (params?: { page?: number; page_size?: number; unread?: number }) =>
    api.get('/notifications', { params }),
  unreadCount: () => api.get('/notifications/unread-count'),
  markRead: (id: number) => api.put(`/notifications/${id}/read`),
  markAllRead: () => api.put('/notifications/read-all'),
}

export const passwordResetApi = {
  submit: (data: { account: string; contact?: string; reason?: string }) =>
    api.post('/auth/password-reset', data),
  adminList: (params?: { page?: number; page_size?: number; status?: number }) =>
    api.get('/admin/password-resets', { params }),
  handle: (id: number, data: { status: number; new_password?: string; admin_note?: string }) =>
    api.put(`/admin/password-resets/${id}`, data),
}

export const essaySubmissionApi = {
  submit: (data: { title: string; content: string; category?: string; year?: number; score?: number }) =>
    api.post('/essays/submit', data),
  mySubmissions: (params?: { page?: number; page_size?: number }) =>
    api.get('/essays/my-submissions', { params }),
  adminList: (params?: { page?: number; page_size?: number; status?: number }) =>
    api.get('/admin/essay-submissions', { params }),
  adminGetOne: (id: number) => api.get(`/admin/essay-submissions/${id}`),
  adminReview: (id: number, data: { action: 'approve' | 'reject'; note?: string; article_type?: number }) =>
    api.put(`/admin/essay-submissions/${id}/review`, data),
}

export const inviteCodeApi = {
  generate: (data: { count: number; note?: string }) =>
    api.post('/admin/invite-codes/generate', data),
  list: (params?: { page?: number; page_size?: number }) =>
    api.get('/admin/invite-codes', { params }),
  delete: (id: number) => api.delete(`/admin/invite-codes/${id}`),
}

export const membershipApi = {
  plans: () => api.get('/membership/plans'),
  channels: () => api.get('/membership/channels'),
  status: () => api.get('/membership/status'),
  // type 取值由 channels() 动态下发，新增支付渠道时前端不需要改
  createOrder: (data: { plan: string; type: string }) =>
    api.post('/membership/orders', data),
  queryOrder: (outTradeNo: string) => api.get(`/membership/orders/${outTradeNo}`),
  listOrders: (params?: { page?: number; page_size?: number }) =>
    api.get('/membership/orders', { params }),
  // 卡密兑换
  redeem: (data: { code: string }) => api.post('/membership/redeem', data),
  // 管理端：套餐与价格
  adminListPlans: () => api.get('/admin/membership/plans'),
  adminUpdatePlan: (
    key: string,
    data: { name: string; amount: number; ldc_amount: number; sort_order: number; enabled: number },
  ) => api.put(`/admin/membership/plans/${key}`, data),
  // 管理端：订单列表（含下单用户名，支持状态/套餐/关键词筛选）
  adminListOrders: (params?: { page?: number; page_size?: number; status?: number; plan?: string; keyword?: string }) =>
    api.get('/admin/orders', { params: { ...params, status: params?.status ?? -1 } }),
}

export const cardKeyApi = {
  generate: (data: { count: number; plan: string; note?: string }) =>
    api.post('/admin/card-keys/generate', data),
  list: (params?: { page?: number; page_size?: number; keyword?: string; plan?: string; status?: number }) =>
    api.get('/admin/card-keys', { params }),
  delete: (id: number) => api.delete(`/admin/card-keys/${id}`),
}

export const questionNoteApi = {
  get: (questionId: number) => api.get(`/questions/${questionId}/note`),
  save: (questionId: number, content: string) =>
    api.put(`/questions/${questionId}/note`, { content }),
  delete: (questionId: number) => api.delete(`/questions/${questionId}/note`),
  myNotes: (params?: { page?: number; page_size?: number; keyword?: string }) =>
    api.get(`/my/notes`, { params }),
  // 单条笔记详情：一次拿回笔记 + 完整题目（含选项）。参数是题目 id，不是笔记 id。
  // 传 silent：本仓的 404 是 HTTP 200 + body code 404，拦截器默认会 reject 并弹
  // 「笔记不存在」红条；详情面板自己用 el-empty 表达这个状态，再叠一个 toast 是错的。
  myNoteDetail: (questionId: number, config?: AxiosRequestConfig) =>
    api.get(`/my/notes/${questionId}`, config),
}

export const aiApi = {
  // 状态
  status: () => api.get('/ai/status'),
  // 会话
  createConversation: (title?: string) => api.post('/ai/conversations', { title }),
  getConversation: (id: number) => api.get(`/ai/conversations/${id}`),
  // 非流式功能：大模型补全耗时较长，覆盖默认 10s 超时
  generateQuestions: (data: { knowledge_point: string; count: number }) =>
    api.post('/ai/generate-questions', data, { timeout: 120000 }),
  gradeEssay: (content: string) => api.post('/ai/grade-essay', { content }, { timeout: 120000 }),
  gradeCase: (data: {
    title: string
    content: string
    reference: string
    explanation: string
    user_answer: string
    score: number
  }) => api.post('/ai/grade-case', data, { timeout: 120000 }),
  // 会员自定义配置
  getMyConfig: () => api.get('/ai/my-config'),
  updateMyConfig: (data: {
    base_url: string
    api_key?: string
    model: string
    temperature: number
    max_tokens: number
    enabled: number
  }) => api.put('/ai/my-config', data),
  testMyConfig: () => api.post('/ai/my-config/test', {}, { timeout: 120000 }),
  // 管理员配置
  getConfig: () => api.get('/admin/ai/config'),
  updateConfig: (data: {
    base_url: string
    api_key?: string
    model: string
    temperature: number
    max_tokens: number
    enabled: number
    free_trial_limit?: number
  }) => api.put('/admin/ai/config', data),
  testConfig: () => api.post('/admin/ai/test'),
  // 管理员用量统计
  usage: (days: number) => api.get('/admin/ai/usage', { params: { days } }),
  callLogs: (params: { scene?: string; page: number; page_size: number }) =>
    api.get('/admin/ai/logs', { params }),
}

export default api

export const commentApi = {
  // 游客可读;登录后返回的每条带 is_owner
  list: (questionId: number, params?: { page?: number; page_size?: number }) =>
    api.get(`/questions/${questionId}/comments`, { params }),
  create: (questionId: number, data: { content: string; parent_id?: number }) =>
    api.post(`/questions/${questionId}/comments`, data),
  remove: (id: number) => api.delete(`/comments/${id}`),
  report: (id: number, data: { reason: string }) =>
    api.post(`/comments/${id}/report`, data),
  // 管理端
  adminList: (params?: { status?: number; keyword?: string; question_id?: number; page?: number; page_size?: number }) =>
    api.get('/admin/comments', { params }),
  adminSetStatus: (id: number, data: { status: number }) =>
    api.put(`/admin/comments/${id}/status`, data),
  adminReportList: (params?: { status?: number; page?: number; page_size?: number }) =>
    api.get('/admin/comment-reports', { params }),
  adminHandleReport: (id: number, data: { action: 'delete' | 'dismiss'; admin_note?: string }) =>
    api.put(`/admin/comment-reports/${id}`, data),
}
