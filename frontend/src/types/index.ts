export interface User {
  id: number
  username: string
  email: string
  nickname: string
  avatar: string
  phone: string
  role: number
  status: number
  created_at: string
  // 管理后台用户列表附带：会员状态（口径同 /membership/status）
  is_member?: boolean
  plan?: string
  expire_at?: string | null
  days_left?: number
  // 当前考试科目（服务端持久化，登录后 restore 用）
  subject_id?: number
}

export interface UserProfile {
  id: number
  user_id: number
  total_practice: number
  total_correct: number
  total_time: number
  streak_days: number
  last_practice_date: string
}

export interface Question {
  id: number
  subject_id: number
  title: string
  content: string
  question_type: number
  difficulty: number
  category_id: number
  knowledge_point: string
  year: number
  sort_order?: number
  paper_section: string
  explanation: string
  analysis: string
  score: number
  options: Option[]
}

export interface Option {
  id: number
  question_id: number
  label: string
  content: string
  is_correct: number
  sort_order: number
}

export interface Paper {
  id: number
  subject_id: number
  title: string
  year: number
  batch: number // 考试批次 0=单次 1=上半年 2=下半年
  paper_type: number // 1上午题 2下午题I 3下午题II 4综合
  total_score: number
  duration: number
  description: string
  status: number // 1=发布 0=下线
  source?: number // 0=真题 1=模拟卷
  question_count?: number
  created_at?: string
}

export interface KnowledgePoint {
  id: number
  name: string
  category_id: number
  parent_id: number
  content: string
  key_points: string
  easy_mistakes: string
  sort_order: number
}

export interface Article {
  id: number
  title: string
  content: string
  article_type: number
  category: string
  year: number
  score: number
  author: string
  tags: string
}

export interface WrongQuestion {
  id: number
  user_id: number
  question_id: number
  wrong_count: number
  correct_count: number
  next_review_time: string
  interval_days: number
  ease_factor: number
  status: number
  question: Question
  // 后端 model.WrongQuestion 会序列化，前端类型之前漏声明；排序按最新答错要用 updated_at
  created_at?: string
  updated_at?: string
}

export interface PracticeRecord {
  id: number
  user_id: number
  question_id: number
  practice_type: number
  user_answer: string
  is_correct: number
  time_spent: number
  created_at: string
}

export interface ExamRecord {
  id: number
  user_id: number
  paper_id: number
  score: number
  total_questions: number
  correct_count: number
  time_spent: number
  started_at: string
  finished_at: string
  status: number
}

export interface PageResponse<T> {
  total: number
  page: number
  page_size: number
  items: T[]
}

export interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

export interface Announcement {
  id: number
  title: string
  content: string
  created_by: number
  status: number
  created_at: string
}

export interface QuestionReport {
  id: number
  question_id: number
  user_id: number
  username: string
  report_type: number
  description: string
  status: number
  admin_note: string
  created_at: string
  question_preview: string
  // 管理端列表附带题目识别字段（标题/年份/题型/题干摘要），便于直接定位并编辑
  question_title?: string
  question_year?: number
  question_type?: number
  question_content?: string
}

export interface Feedback {
  id: number
  user_id: number
  username: string
  feedback_type: number
  // 内容纠错类反馈可关联具体题目，便于管理员定位/编辑
  question_id?: number
  question_title?: string
  question_year?: number
  question_type?: number
  question_content?: string
  content: string
  contact: string
  status: number
  admin_note: string
  created_at: string
}

export interface EssaySubmission {
  id: number
  user_id: number
  username?: string
  title: string
  content?: string
  category: string
  year: number
  score: number
  review_status: number // 0=待审核 1=已通过 2=已拒绝
  reviewer_note: string
  article_id: number
  created_at: string
}

export interface InviteCode {
  id: number
  code: string
  created_by: number
  used_by: number
  used_at: string | null
  note: string
  status: number // 0=未使用 1=已使用
  created_at: string
}

export interface CardKey {
  id: number
  code: string
  plan: string // month/season/exam/year
  created_by: number
  used_by: number
  used_at: string | null
  note: string
  status: number // 0=未使用 1=已使用
  created_at: string
}

export interface AdminOrder {
  id: number
  out_trade_no: string
  user_id: number
  username: string
  plan: string
  channel: string
  amount: number // 分
  status: number // 0待支付 1已支付 2已退款 3已关闭
  pay_channel: string
  trade_no: string
  paid_at: string | null
  created_at: string
}

export interface QuestionNote {
  content: string
  exists: boolean
  updated_at?: string
}

/** 「我的笔记」列表/详情里的单条笔记（含题目归属信息）。 */
export interface NoteWithQuestion {
  id: number
  question_id: number
  content: string
  updated_at: string
  question_title: string
  question_stem: string
  question_type: number
  knowledge_point: string
  paper_title: string
}
