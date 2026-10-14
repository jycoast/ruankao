package model

import "time"

type User struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"size:50;uniqueIndex;not null" json:"username"`
	Email        string    `gorm:"size:100;uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Nickname     string    `gorm:"size:50;default:''" json:"nickname"`
	Avatar       string    `gorm:"size:500;default:''" json:"avatar"`
	Phone        string    `gorm:"size:20;default:''" json:"phone"`
	Role         int8      `gorm:"default:0;not null" json:"role"`
	Status       int8      `gorm:"default:1;not null" json:"status"`
	LinuxDoID    uint64    `gorm:"default:0;index" json:"-"` // 0=未绑定
	SubjectID    uint64    `gorm:"default:1;not null" json:"subject_id"` // 当前考试科目(跨设备同步)
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (User) TableName() string { return "users" }

type UserProfile struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         uint64     `gorm:"uniqueIndex;not null" json:"user_id"`
	TotalPractice  int        `gorm:"default:0" json:"total_practice"`
	TotalCorrect   int        `gorm:"default:0" json:"total_correct"`
	TotalTime      int        `gorm:"default:0" json:"total_time"`
	StreakDays     int        `gorm:"default:0" json:"streak_days"`
	LastPracticeAt *time.Time `gorm:"column:last_practice_date" json:"last_practice_date"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (UserProfile) TableName() string { return "user_profiles" }

type Subject struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Code      string    `gorm:"size:50;default:''" json:"code"`
	Icon      string    `gorm:"size:50;default:''" json:"icon"`
	SortOrder int       `gorm:"default:0" json:"sort_order"`
	Status    int8      `gorm:"default:1;not null" json:"status"`
	HasEssay  int8      `gorm:"default:0;not null" json:"has_essay"`
	HasSprint int8      `gorm:"default:0;not null" json:"has_sprint"`
	Level     int8      `gorm:"default:2;not null" json:"level"`
	CreatedAt time.Time `json:"created_at"`
}

func (Subject) TableName() string { return "subjects" }

type Question struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	SubjectID      uint64    `gorm:"default:1" json:"subject_id"`
	Title          string    `gorm:"type:text;not null" json:"title"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	QuestionType   int8      `gorm:"not null" json:"question_type"`
	Difficulty     int8      `gorm:"default:1" json:"difficulty"`
	CategoryID     uint64    `gorm:"default:0" json:"category_id"`
	KnowledgePoint string    `gorm:"size:200;default:''" json:"knowledge_point"`
	Year           int16     `gorm:"default:0" json:"year"`
	SortOrder      int       `gorm:"default:0" json:"sort_order"` // 排序权重 0=未手动排序(排最后) 越小越靠前
	Batch          int8      `gorm:"default:0" json:"batch"` // 考试批次 0:单次 1:上半年 2:下半年
	PaperSection   string    `gorm:"size:50;default:''" json:"paper_section"`
	Explanation    string    `gorm:"type:text" json:"explanation"`
	Analysis       string    `gorm:"type:text" json:"analysis"`
	Score          float32   `gorm:"default:0" json:"score"`
	Status         int8      `gorm:"default:1;not null" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Options        []Option  `gorm:"foreignKey:QuestionID" json:"options,omitempty"`
}

func (Question) TableName() string { return "questions" }

type Option struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	QuestionID uint64 `gorm:"index;not null" json:"question_id"`
	Label      string `gorm:"column:option_label;size:1;not null" json:"label"`
	Content    string `gorm:"type:text;not null" json:"content"`
	IsCorrect  int8   `gorm:"default:0" json:"is_correct"`
	SortOrder  int    `gorm:"default:0" json:"sort_order"`
}

func (Option) TableName() string { return "options" }

type Paper struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	SubjectID   uint64    `gorm:"default:1" json:"subject_id"`
	Title       string    `gorm:"size:200;not null" json:"title"`
	Year        int16     `gorm:"not null" json:"year"`
	Batch       int8      `gorm:"default:0" json:"batch"` // 考试批次 0:单次 1:上半年 2:下半年
	PaperType   int8      `gorm:"not null" json:"paper_type"`
	TotalScore  float32   `gorm:"default:0" json:"total_score"`
	Duration    int       `gorm:"default:150" json:"duration"`
	Description string    `gorm:"type:text" json:"description"`
	Status      int8      `gorm:"default:1;not null" json:"status"`
	Source      int8      `gorm:"default:0;not null" json:"source"` // 0:真题 1:模拟卷
	CreatedBy   uint64    `gorm:"default:0" json:"created_by"`      // 生成模拟卷的用户ID
	CreatedAt   time.Time `json:"created_at"`
	// QuestionCount 管理端列表展示用的卷内题目数，非持久化字段
	QuestionCount int64 `gorm:"-" json:"question_count,omitempty"`
}

func (Paper) TableName() string { return "papers" }

type PaperQuestion struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	PaperID    uint64 `gorm:"not null" json:"paper_id"`
	QuestionID uint64 `gorm:"not null" json:"question_id"`
	SortOrder  int    `gorm:"default:0" json:"sort_order"`
}

func (PaperQuestion) TableName() string { return "paper_questions" }

type KnowledgePoint struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string    `gorm:"size:200;not null" json:"name"`
	CategoryID   uint64    `gorm:"default:0" json:"category_id"`
	SubjectID    uint64    `gorm:"default:1;not null" json:"subject_id"`
	ParentID     uint64    `gorm:"default:0" json:"parent_id"`
	Content      string    `gorm:"type:text" json:"content"`
	KeyPoints    string    `gorm:"type:text" json:"key_points"`
	EasyMistakes string    `gorm:"type:text" json:"easy_mistakes"`
	MatchKey     string    `gorm:"size:300;default:''" json:"match_key"` // 关联真题的关键词，'|' 分隔
	SortOrder    int       `gorm:"default:0" json:"sort_order"`
	Status       int8      `gorm:"default:1;not null" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (KnowledgePoint) TableName() string { return "knowledge_points" }

// RelatedQuestion 知识点关联真题的精简视图
type RelatedQuestion struct {
	ID           uint64 `json:"id"`
	Title        string `json:"title"`
	QuestionType int8   `json:"question_type"`
	Year         int16  `json:"year"`
}

type Article struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Title       string    `gorm:"size:200;not null" json:"title"`
	Content     string    `gorm:"type:text;not null" json:"content"`
	ArticleType int8      `gorm:"not null" json:"article_type"`
	SubjectID   uint64    `gorm:"default:1;not null" json:"subject_id"`
	Category    string    `gorm:"size:100;default:''" json:"category"`
	Year        int16     `gorm:"default:0" json:"year"`
	Score       float32   `gorm:"default:0" json:"score"`
	Author      string    `gorm:"size:50;default:''" json:"author"`
	Tags        string    `gorm:"size:500;default:''" json:"tags"`
	Status      int8      `gorm:"default:1;not null" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Article) TableName() string { return "articles" }

type WrongQuestion struct {
	ID              uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID          uint64     `gorm:"uniqueIndex:idx_wq_user_question;not null" json:"user_id"`
	QuestionID      uint64     `gorm:"uniqueIndex:idx_wq_user_question;not null" json:"question_id"`
	WrongCount      int        `gorm:"default:1" json:"wrong_count"`
	CorrectCount    int        `gorm:"default:0" json:"correct_count"`
	LastWrongAnswer string     `gorm:"type:text" json:"last_wrong_answer"`
	NextReviewTime  *time.Time `json:"next_review_time"`
	IntervalDays    int        `gorm:"default:1" json:"interval_days"`
	EaseFactor      float32    `gorm:"default:2.50" json:"ease_factor"`
	Status          int8       `gorm:"default:1" json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Question        *Question  `gorm:"foreignKey:QuestionID" json:"question,omitempty"`
}

func (WrongQuestion) TableName() string { return "wrong_questions" }

type Favorite struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     uint64    `gorm:"uniqueIndex:idx_fav_target;not null" json:"user_id"`
	TargetType int8      `gorm:"uniqueIndex:idx_fav_target;not null" json:"target_type"`
	TargetID   uint64    `gorm:"uniqueIndex:idx_fav_target;not null" json:"target_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (Favorite) TableName() string { return "favorites" }

type PracticeRecord struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint64    `gorm:"index;not null" json:"user_id"`
	QuestionID   uint64    `gorm:"index;not null" json:"question_id"`
	PracticeType int8      `gorm:"not null" json:"practice_type"`
	UserAnswer   string    `gorm:"type:text" json:"user_answer"`
	IsCorrect    int8      `gorm:"default:0" json:"is_correct"`
	TimeSpent    int       `gorm:"default:0" json:"time_spent"`
	PaperID      uint64    `gorm:"default:0" json:"paper_id"`
	CreatedAt    time.Time `json:"created_at"`
}

func (PracticeRecord) TableName() string { return "practice_records" }

// PracticeSession 分类刷题的断点草稿：本次抽中哪些题、已经答了什么。
// 与 PracticeRecord 的分工：PracticeRecord 是不可变流水，喂给学习报告和正确率；
// 本表是可覆盖的草稿，只服务「继续刷题」，清掉它不影响任何统计。
type PracticeSession struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         uint64    `gorm:"uniqueIndex:idx_session_scope;not null" json:"user_id"`
	SubjectID      uint64    `gorm:"uniqueIndex:idx_session_scope;not null" json:"subject_id"`
	QuestionType   int8      `gorm:"uniqueIndex:idx_session_scope;not null" json:"question_type"`
	KnowledgePoint string    `gorm:"uniqueIndex:idx_session_scope;size:200;not null" json:"knowledge_point"`
	QuestionIDs    string    `gorm:"type:text;not null" json:"-"` // JSON 数组，元素顺序即刷题顺序
	Answers        string    `gorm:"type:text;not null" json:"-"` // JSON 对象 {question_id: 用户答案}
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (PracticeSession) TableName() string { return "practice_sessions" }

// PaperPracticeSession 真题练习模式的断点草稿：作答、判题结果、当前题位置。
// 与 PracticeRecord 的分工：PracticeRecord 是不可变流水，喂给学习报告和正确率；
// 本表是可覆盖的草稿，只服务「继续练习」，清掉它不影响任何统计（同 practice_sessions）。
type PaperPracticeSession struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"uniqueIndex:idx_pps_scope;not null" json:"user_id"`
	PaperID   uint64    `gorm:"uniqueIndex:idx_pps_scope;not null" json:"paper_id"`
	Mode      string    `gorm:"uniqueIndex:idx_pps_scope;size:20;not null" json:"mode"`
	Data      string    `gorm:"type:text;not null" json:"-"` // JSON: {answers, practiceResults, currentIndex}
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (PaperPracticeSession) TableName() string { return "paper_practice_sessions" }

type ExamRecord struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         uint64     `gorm:"index;not null" json:"user_id"`
	PaperID        uint64     `gorm:"index;not null" json:"paper_id"`
	Score          float32    `gorm:"default:0" json:"score"`
	TotalQuestions int        `gorm:"default:0" json:"total_questions"`
	CorrectCount   int        `gorm:"default:0" json:"correct_count"`
	TimeSpent      int        `gorm:"default:0" json:"time_spent"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	Status         int8       `gorm:"default:0" json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (ExamRecord) TableName() string { return "exam_records" }

type Announcement struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Title     string    `gorm:"size:200;not null" json:"title"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	CreatedBy uint64    `gorm:"default:0" json:"created_by"`
	Status    int8      `gorm:"default:1;not null" json:"status"` // 1=发布 0=下线
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Announcement) TableName() string { return "announcements" }

type ExamSchedule struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Label     string    `gorm:"size:50;not null" json:"label"`
	ExamDate  time.Time `gorm:"type:date;not null" json:"exam_date"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ExamSchedule) TableName() string { return "exam_schedules" }

type AnnouncementRead struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         uint64    `gorm:"uniqueIndex:uk_user_ann;not null" json:"user_id"`
	AnnouncementID uint64    `gorm:"uniqueIndex:uk_user_ann;not null" json:"announcement_id"`
	CreatedAt      time.Time `json:"created_at"`
}

func (AnnouncementRead) TableName() string { return "announcement_reads" }

// Notification 站内信。审核流处理完后主动告知用户，未读以 ReadAt 为空判定。
type Notification struct {
	ID        uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64     `gorm:"index;not null" json:"user_id"`
	Type      int8       `gorm:"not null;default:1" json:"type"` // 1=审核结果 2=系统消息
	Title     string     `gorm:"size:120;default:''" json:"title"`
	Content   string     `gorm:"type:text" json:"content"`
	BizType   string     `gorm:"size:20;default:''" json:"biz_type"` // report/essay/feedback
	BizID     uint64     `gorm:"default:0" json:"biz_id"`
	Link      string     `gorm:"size:200;default:''" json:"link"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }

type QuestionReport struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	QuestionID  uint64    `gorm:"index;not null" json:"question_id"`
	UserID      uint64    `gorm:"index;not null" json:"user_id"`
	ReportType  int8      `gorm:"not null;default:1" json:"report_type"` // 1=题目有误 2=答案有误 3=解析有误 4=选项有误 5=其他
	Description string    `gorm:"type:text;not null" json:"description"`
	Status      int8      `gorm:"not null;default:0" json:"status"` // 0=待处理 1=已处理 2=已忽略
	AdminNote   string    `gorm:"size:500;default:''" json:"admin_note"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (QuestionReport) TableName() string { return "question_reports" }

type Feedback struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint64    `gorm:"index;not null" json:"user_id"`
	FeedbackType int8      `gorm:"not null;default:1" json:"feedback_type"` // 1=功能建议 2=问题反馈 3=内容纠错 4=其他
	QuestionID   uint64    `gorm:"not null;default:0" json:"question_id"`   // 关联题目ID，0=未关联（内容纠错时填）
	Content      string    `gorm:"type:text;not null" json:"content"`
	Contact      string    `gorm:"size:100;default:''" json:"contact"`
	Status       int8      `gorm:"not null;default:0" json:"status"` // 0=待处理 1=已处理 2=已忽略
	AdminNote    string    `gorm:"size:500;default:''" json:"admin_note"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (Feedback) TableName() string { return "feedbacks" }

// PasswordReset 找回密码申请，管理员人工审核。项目没有 SMTP，这是最低成本的可用方案。
//
// 只在账号确实存在时才建行 —— 表里有没有这条记录就是「账号存不存在」的答案，
// 接口层再怎么统一返回成功，管理员一看列表还是把不存在的账号也暴露了。
// 不存新密码：管理员审核时现场生成，经 Contact 线下告知，密码不落库也不进日志。
type PasswordReset struct {
	ID     uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID uint64 `gorm:"index;not null" json:"user_id"`
	// Account 提交时填的用户名或邮箱，原样保留：管理员据此核对本人身份
	Account   string    `gorm:"size:100;default:''" json:"account"`
	Contact   string    `gorm:"size:100;default:''" json:"contact"`
	Reason    string    `gorm:"size:500;default:''" json:"reason"`
	ClientIP  string    `gorm:"size:64;default:''" json:"client_ip"`
	Status    int8      `gorm:"not null;default:0" json:"status"` // 0=待处理 1=已处理 2=已忽略
	AdminNote string    `gorm:"size:500;default:''" json:"admin_note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (PasswordReset) TableName() string { return "password_resets" }

type EssaySubmission struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint64    `gorm:"index;not null" json:"user_id"`
	Title        string    `gorm:"size:200;not null" json:"title"`
	Content      string    `gorm:"type:text;not null" json:"content"`
	Category     string    `gorm:"size:100;default:''" json:"category"`
	Year         int16     `gorm:"default:0" json:"year"`
	Score        float32   `gorm:"default:0" json:"score"`
	ReviewStatus int8      `gorm:"not null;default:0" json:"review_status"` // 0=待审核 1=已通过 2=已拒绝
	ReviewerNote string    `gorm:"size:500;default:''" json:"reviewer_note"`
	ArticleID    uint64    `gorm:"default:0" json:"article_id"` // 通过后对应的文章 ID
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (EssaySubmission) TableName() string { return "essay_submissions" }

type InviteCode struct {
	ID        uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Code      string     `gorm:"size:20;uniqueIndex;not null" json:"code"`
	CreatedBy uint64     `gorm:"default:0" json:"created_by"`
	UsedBy    uint64     `gorm:"default:0" json:"used_by"`
	UsedAt    *time.Time `json:"used_at"`
	Note      string     `gorm:"size:200;default:''" json:"note"`
	Status    int8       `gorm:"default:0;not null" json:"status"` // 0=未使用 1=已使用
	CreatedAt time.Time  `json:"created_at"`
}

func (InviteCode) TableName() string { return "invite_codes" }

// CardKey 会员兑换卡密：一次性使用，绑定套餐(Plan)，兑换后按套餐天数开通会员。
// 结构仿 InviteCode，多一个 Plan 字段。
type CardKey struct {
	ID        uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Code      string     `gorm:"size:32;uniqueIndex;not null" json:"code"`
	Plan      string     `gorm:"size:20;not null" json:"plan"` // month/season/exam/year
	CreatedBy uint64     `gorm:"default:0" json:"created_by"`
	UsedBy    uint64     `gorm:"default:0" json:"used_by"`
	UsedAt    *time.Time `json:"used_at"`
	Note      string     `gorm:"size:200;default:''" json:"note"`
	Status    int8       `gorm:"default:0;not null" json:"status"` // 0=未使用 1=已使用
	CreatedAt time.Time  `json:"created_at"`
}

func (CardKey) TableName() string { return "card_keys" }

// DailyCheckin 每日一练签到记录：一个用户一天一条。
// CheckinDate 为 YYYY-MM-DD（本地日），配合唯一索引保证幂等。
type DailyCheckin struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        uint64    `gorm:"uniqueIndex:uk_user_date;not null" json:"user_id"`
	CheckinDate   string    `gorm:"size:10;uniqueIndex:uk_user_date;not null" json:"checkin_date"`
	QuestionCount int       `gorm:"default:0" json:"question_count"`
	CorrectCount  int       `gorm:"default:0" json:"correct_count"`
	CreatedAt     time.Time `json:"created_at"`
}

func (DailyCheckin) TableName() string { return "daily_checkins" }

// DailySession 每日一练断点草稿：可覆盖、可删，不影响 practice_records 统计。
// 作用域是 (user_id, date, subject_id)，同一作用域只留一份。
type DailySession struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"uniqueIndex:idx_daily_session_scope;not null" json:"user_id"`
	Date      string    `gorm:"size:10;uniqueIndex:idx_daily_session_scope;not null" json:"date"`
	SubjectID uint64    `gorm:"uniqueIndex:idx_daily_session_scope;not null" json:"subject_id"`
	Data      string    `gorm:"type:text;not null" json:"-"` // JSON
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DailySession) TableName() string { return "daily_sessions" }

type QuestionNote struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     uint64    `gorm:"uniqueIndex:uk_user_question;not null" json:"user_id"`
	QuestionID uint64    `gorm:"uniqueIndex:uk_user_question;not null" json:"question_id"`
	Content    string    `gorm:"type:longtext;not null" json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (QuestionNote) TableName() string { return "question_notes" }

// UserSubjectStat 用户按科目的学习累加（与 user_profiles 同构，但维度是 user_id+subject_id）。
// 学习概况按当前科目切换时读这张表；user_profiles 继续承担全科目合计。
type UserSubjectStat struct {
	ID             uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         uint64     `gorm:"uniqueIndex:uk_user_subject;not null" json:"user_id"`
	SubjectID      uint64     `gorm:"uniqueIndex:uk_user_subject;not null" json:"subject_id"`
	TotalPractice  int        `gorm:"default:0" json:"total_practice"`
	TotalCorrect   int        `gorm:"default:0" json:"total_correct"`
	TotalTime      int        `gorm:"default:0" json:"total_time"`
	StreakDays     int        `gorm:"default:0" json:"streak_days"`
	LastPracticeAt *time.Time `gorm:"column:last_practice_date" json:"last_practice_date"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (UserSubjectStat) TableName() string { return "user_subject_stats" }

// NoteWithQuestion 笔记连同题目概要，用于「我的笔记」列表展示。
type NoteWithQuestion struct {
	ID              uint64    `json:"id"`
	QuestionID      uint64    `json:"question_id"`
	Content         string    `json:"content"`
	UpdatedAt       time.Time `json:"updated_at"`
	QuestionTitle   string    `json:"question_title"`   // 题干缩略（前端可再裁剪）
	QuestionStem    string    `json:"question_stem"`    // 题干前 N 字，用于卡片预览
	QuestionType    int8      `json:"question_type"`
	KnowledgePoint  string    `json:"knowledge_point"`
	PaperTitle      string    `json:"paper_title"`      // 所属套卷，如「2025年上半年综合知识」
}

// ---- AI Agent ----

// AIConfig 全局单行（id=1）大模型配置（OpenAI 兼容）。api_key 仅存 DB，绝不回传前端。
type AIConfig struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	BaseURL        string    `gorm:"size:255;default:''" json:"base_url"`
	APIKey         string    `gorm:"size:255;default:''" json:"-"`
	Model          string    `gorm:"size:100;default:''" json:"model"`
	Temperature    float64   `gorm:"type:decimal(3,2);default:0.70" json:"temperature"`
	MaxTokens      int       `gorm:"default:2048" json:"max_tokens"`
	Enabled        int8      `gorm:"default:0;not null" json:"enabled"`          // 0=关 1=开
	FreeTrialLimit int       `gorm:"default:5;not null" json:"free_trial_limit"` // 非会员累计免费次数，0=不提供试用
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (AIConfig) TableName() string { return "ai_configs" }

// SiteSettings 站点级开关（单行表，固定 id=1）。
// 形状照搬 ai_configs：仓库不为全局配置引入通用 KV 表，每个功能一张单行表。
type SiteSettings struct {
	ID                 uint8     `gorm:"primaryKey" json:"id"`
	MaintenanceEnabled int8      `gorm:"not null;default:0" json:"maintenance_enabled"` // 0=关 1=开
	UpdatedAt          time.Time `json:"updated_at"`
}

func (SiteSettings) TableName() string { return "site_settings" }

// AITrialUsage 非会员 AI 免费试用累计用量（每用户单行）。
type AITrialUsage struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"uniqueIndex;not null" json:"user_id"`
	UsedCount int       `gorm:"default:0;not null" json:"used_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AITrialUsage) TableName() string { return "ai_trial_usages" }

// UserAIConfig 会员自定义 AI 配置（每用户单行）。缺省字段回退全局 AIConfig；
// 会员配置完整（base_url+api_key+model 均非空且 enabled=1）时不受全局开关限制。api_key 绝不回传前端。
type UserAIConfig struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      uint64    `gorm:"uniqueIndex;not null" json:"user_id"`
	BaseURL     string    `gorm:"size:255;default:''" json:"base_url"`
	APIKey      string    `gorm:"size:255;default:''" json:"-"`
	Model       string    `gorm:"size:100;default:''" json:"model"`
	Temperature float64   `gorm:"type:decimal(3,2);default:0.70" json:"temperature"`
	MaxTokens   int       `gorm:"default:2048" json:"max_tokens"`
	Enabled     int8      `gorm:"default:0;not null" json:"enabled"` // 0=关 1=开
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (UserAIConfig) TableName() string { return "user_ai_configs" }

type AIConversation struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"index;not null" json:"user_id"`
	Title     string    `gorm:"size:100;default:''" json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AIConversation) TableName() string { return "ai_conversations" }

type AIMessage struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID uint64    `gorm:"index;not null" json:"conversation_id"`
	Role           string    `gorm:"size:16;not null" json:"role"` // user / assistant
	Content        string    `gorm:"type:longtext;not null" json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

func (AIMessage) TableName() string { return "ai_messages" }

// AICallLog 一次大模型调用的记录，用于成本核算与异常排查。
//
// 只记 scene / 用量 / 耗时 / 成败，不落 prompt 与回复正文 ——
// 会话内容已在 ai_messages 里，重复存一份既涨库又扩大泄露面。
// OwnKey 标记这次用的是用户自带的 key：这类调用不花平台的钱，
// 统计成本时必须排除，否则账单口径直接失真。
type AICallLog struct {
	ID               uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           uint64 `gorm:"index;not null" json:"user_id"`
	Scene            string `gorm:"size:32;index;not null" json:"scene"` // chat/explain/generate-questions/generate-essay/grade-essay/test
	Model            string `gorm:"size:100;default:''" json:"model"`
	PromptTokens     int    `gorm:"default:0;not null" json:"prompt_tokens"`
	CompletionTokens int    `gorm:"default:0;not null" json:"completion_tokens"`
	TotalTokens      int    `gorm:"default:0;not null" json:"total_tokens"`
	DurationMs       int    `gorm:"default:0;not null" json:"duration_ms"`
	Success          int8   `gorm:"default:1;not null" json:"success"` // 0=失败 1=成功
	ErrMsg           string `gorm:"size:255;default:''" json:"err_msg"`
	OwnKey           int8   `gorm:"default:0;not null" json:"own_key"` // 1=用户自带 key，不计平台成本
	// Estimated=1 表示 token 数是按字符估算的：多数 OpenAI 兼容服务流式调用不返回 usage，
	// 而站内大部分场景都是流式。不标出来的话，估算值会混进账单口径被当成真实计数。
	Estimated int8      `gorm:"default:0;not null" json:"estimated"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (AICallLog) TableName() string { return "ai_call_logs" }

// Membership 会员有效期：一个用户一行，是否会员只看 ExpireAt
type Membership struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"uniqueIndex;not null" json:"user_id"`
	Plan      string    `gorm:"size:20;default:''" json:"plan"`
	ExpireAt  time.Time `json:"expire_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Membership) TableName() string { return "memberships" }

// MembershipPlan 会员套餐。价格可由管理员在后台调整，用于做限时活动。
// Amount 与 LDCAmount 是两条独立定价线（人民币 / LINUX DO Credit 积分），
// 都以「分」为单位，默认 1 积分 = 1 元但可以各调各的。
type MembershipPlan struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	PlanKey   string    `gorm:"column:plan_key;size:20;uniqueIndex;not null" json:"key"`
	Name      string    `gorm:"size:50;not null" json:"name"`
	Days      int       `gorm:"not null" json:"days"`
	Amount    int       `gorm:"not null" json:"amount"`     // 人民币，单位：分
	LDCAmount int       `gorm:"not null" json:"ldc_amount"` // LDC 积分，单位：0.01 积分
	SortOrder int       `gorm:"default:0;not null" json:"sort_order"`
	Enabled   int8      `gorm:"default:1;not null" json:"enabled"` // 0=下架，不可新购
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (MembershipPlan) TableName() string { return "membership_plans" }

// Order 订单
type Order struct {
	ID         uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OutTradeNo string     `gorm:"size:64;uniqueIndex;not null" json:"out_trade_no"`
	UserID     uint64     `gorm:"index;not null" json:"user_id"`
	Plan       string     `gorm:"size:20;not null" json:"plan"`
	Channel    string     `gorm:"size:20;default:'yipay';not null" json:"channel"` // yipay / ldc / yuetongyun
	// Amount 下单时锁定的价格快照，单位：分（LDC 通道即 0.01 积分）。
	// 必须存快照 —— 管理员做活动改价后，回调按当前价校验会把活动前下的单全判成金额不符。
	Amount     int    `gorm:"not null" json:"amount"`
	Status     int8   `gorm:"default:0;not null" json:"status"` // 0待支付 1已支付 2已退款 3已关闭
	PayChannel string `gorm:"size:20;default:''" json:"pay_channel"`
	TradeNo    string `gorm:"size:64;default:''" json:"trade_no"`
	PaidAt     *time.Time `json:"paid_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (Order) TableName() string { return "orders" }

// ── 评论区/答疑讨论 (083) ─────────────────────────────────────────

type QuestionComment struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	QuestionID    uint64    `gorm:"index:idx_q;not null" json:"question_id"`
	UserID        uint64    `gorm:"index:idx_user;not null" json:"user_id"`
	ParentID      uint64    `gorm:"index:idx_parent;default:0;not null" json:"parent_id"` // 0=顶级,否则指向被回复的顶级评论 id
	ReplyToUserID uint64    `gorm:"default:0;not null" json:"reply_to_user_id"`            // 被回复人(仅 parent_id>0 时)
	Content       string    `gorm:"type:text;not null" json:"content"`
	Status        int8      `gorm:"default:0;not null" json:"status"` // 0=正常 1=用户自删 2=管理员隐藏
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CommentReport struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	CommentID uint64    `gorm:"uniqueIndex:uk_comment_user;not null" json:"comment_id"`
	UserID    uint64    `gorm:"uniqueIndex:uk_comment_user;not null" json:"user_id"`
	Reason    string    `gorm:"size:200;default:''" json:"reason"`
	Status    int8      `gorm:"default:0;not null" json:"status"` // 0=待处理 1=已处理 2=驳回
	AdminNote string    `gorm:"size:500;default:''" json:"admin_note"`
	CreatedAt time.Time `json:"created_at"`
	HandledAt *time.Time `json:"handled_at"`
}

// CommentWithUser 评论区列表用:评论 + 作者昵称/头像(join users)
type CommentWithUser struct {
	QuestionComment
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// AdminCommentView 管理端评论列表用:评论 + 作者 + 题干摘要
type AdminCommentView struct {
	QuestionComment
	Nickname        string `json:"nickname"`
	Avatar          string `json:"avatar"`
	QuestionSnippet string `json:"question_snippet"`
}

// AdminCommentReportView 管理端举报队列用:举报 + 评论内容 + 作者/举报人昵称
type AdminCommentReportView struct {
	CommentReport
	CommentContent   string `json:"comment_content"`
	AuthorNickname   string `json:"author_nickname"`
	ReporterNickname string `json:"reporter_nickname"`
}
