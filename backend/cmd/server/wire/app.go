// Package wire 负责把所有 repo/service/handler/middleware 装配成 App。
//
// 设计动机：main.go 重构前 536 行,装配代码 91 个 New(...) 调用全部扁平
// 铺开,无法独立测试,改一个 handler 容易触发主分支冲突。这里把装配按层
// 拆成 4 个 struct,任何一处依赖变化只影响对应 struct。
//
// 装配顺序:Repos → Services(依赖 Repos) → Handlers(依赖 Services)
//           → Middlewares(依赖 Services,如 AuthMiddleware 需要 UserStateService)。
// NewApp 串起四步,main.go 只剩 server lifecycle。
package wire

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"software-exam/config"
	"software-exam/internal/api/handler"
	"software-exam/internal/api/middleware"
	"software-exam/internal/application/service"
	"software-exam/internal/domain/repository"
)

// App 装配完成后的全部依赖。main.go 拿到 App 即可开服务器。
type App struct {
	Repos       *Repos
	Services    *Services
	Handlers    *Handlers
	Middlewares *Middlewares
}

// NewApp 装配全栈。db/redis 必须在外部 init 好(config.Load → DB init → Redis init)。
// storageService 特殊——它依赖 config.AppConfig.OSS,不是 DB/Redis,在这里注入。
func NewApp(db *gorm.DB, redis *redis.Client) *App {
	repos := assembleRepos(db)
	services := assembleServices(repos, redis)
	handlers := assembleHandlers(services, repos)
	middlewares := assembleMiddlewares(services, redis)
	return &App{
		Repos:       repos,
		Services:    services,
		Handlers:    handlers,
		Middlewares: middlewares,
	}
}

// Repos 数据访问层。28 个 repo,顺序与原 main.go 一致,便于 review 对照。
type Repos struct {
	User            repository.UserRepository
	Profile         repository.UserProfileRepository
	Question        repository.QuestionRepository
	WrongQuestion   repository.WrongQuestionRepository
	Favorite        repository.FavoriteRepository
	Practice        repository.PracticeRecordRepository
	PracticeSession repository.PracticeSessionRepository
	PracticeQuery   repository.PracticeQueryRepository
	PaperSession    repository.PaperPracticeSessionRepository
	KnowledgePoint  repository.KnowledgePointRepository
	Article         repository.ArticleRepository
	Paper           repository.PaperRepository
	Exam            repository.ExamRecordRepository
	Subject         repository.SubjectRepository
	Announcement    repository.AnnouncementRepository
	Report          repository.QuestionReportRepository
	Feedback        repository.FeedbackRepository
	PasswordReset   repository.PasswordResetRepository
	EssaySubmission repository.EssaySubmissionRepository
	InviteCode      repository.InviteCodeRepository
	CardKey         repository.CardKeyRepository
	DailyCheckin    repository.DailyCheckinRepository
	DailySession    repository.DailySessionRepository
	QuestionNote    repository.QuestionNoteRepository
	AIConfig        repository.AIConfigRepository
	UserAIConfig    repository.UserAIConfigRepository
	AIConversation  repository.AIConversationRepository
	AIMessage       repository.AIMessageRepository
	AITrialUsage    repository.AITrialUsageRepository
	AICallLog       repository.AICallLogRepository
	Membership      repository.MembershipRepository
	MembershipPlan  repository.MembershipPlanRepository
	Order           repository.OrderRepository
	Notification    repository.NotificationRepository
	Comment         repository.CommentRepository
	CommentReport   repository.CommentReportRepository
	ExamSchedule    repository.ExamScheduleRepository
	Dashboard       repository.DashboardRepository
	UserSubjectStat repository.UserSubjectStatRepository
	SiteSettings    repository.SiteSettingsRepository
}

func assembleRepos(db *gorm.DB) *Repos {
	return &Repos{
		User:            repository.NewUserRepository(db),
		Profile:         repository.NewUserProfileRepository(db),
		Question:        repository.NewQuestionRepository(db),
		WrongQuestion:   repository.NewWrongQuestionRepository(db),
		Favorite:        repository.NewFavoriteRepository(db),
		Practice:        repository.NewPracticeRecordRepository(db),
		PracticeSession: repository.NewPracticeSessionRepository(db),
		PracticeQuery:   repository.NewPracticeQueryRepository(db),
		PaperSession:    repository.NewPaperPracticeSessionRepository(db),
		KnowledgePoint:  repository.NewKnowledgePointRepository(db),
		Article:         repository.NewArticleRepository(db),
		Paper:           repository.NewPaperRepository(db),
		Exam:            repository.NewExamRecordRepository(db),
		Subject:         repository.NewSubjectRepository(db),
		Announcement:    repository.NewAnnouncementRepository(db),
		Report:          repository.NewQuestionReportRepository(db),
		Feedback:        repository.NewFeedbackRepository(db),
		PasswordReset:   repository.NewPasswordResetRepository(db),
		EssaySubmission: repository.NewEssaySubmissionRepository(db),
		InviteCode:      repository.NewInviteCodeRepository(db),
		CardKey:         repository.NewCardKeyRepository(db),
		DailyCheckin:    repository.NewDailyCheckinRepository(db),
		DailySession:    repository.NewDailySessionRepository(db),
		QuestionNote:    repository.NewQuestionNoteRepository(db),
		AIConfig:        repository.NewAIConfigRepository(db),
		UserAIConfig:    repository.NewUserAIConfigRepository(db),
		AIConversation:  repository.NewAIConversationRepository(db),
		AIMessage:       repository.NewAIMessageRepository(db),
		AITrialUsage:    repository.NewAITrialUsageRepository(db),
		AICallLog:       repository.NewAICallLogRepository(db),
		Membership:      repository.NewMembershipRepository(db),
		MembershipPlan:  repository.NewMembershipPlanRepository(db),
		Order:           repository.NewOrderRepository(db),
		Notification:    repository.NewNotificationRepository(db),
		Comment:         repository.NewCommentRepository(db),
		CommentReport:   repository.NewCommentReportRepository(db),
		ExamSchedule:    repository.NewExamScheduleRepository(db),
		Dashboard:       repository.NewDashboardRepository(db),
		UserSubjectStat: repository.NewUserSubjectStatRepository(db),
		SiteSettings:    repository.NewSiteSettingsRepository(db),
	}
}

// Services 业务层。Service 间有依赖(examService 依赖 practiceService),
// 必须按依赖顺序装配:无依赖的先,有依赖的后,Go 编译器保证调用顺序。
type Services struct {
	UserState        *service.UserStateService
	Question         *service.QuestionService
	Practice         *service.PracticeService
	Favorite         *service.FavoriteService
	Knowledge        *service.KnowledgeService
	Article          *service.ArticleService
	Exam             *service.ExamService
	Admin            *service.AdminService
	Announcement     *service.AnnouncementService
	Notification     *service.NotificationService
	Comment          *service.CommentService
	Report           *service.QuestionReportService
	Feedback         *service.FeedbackService
	PasswordReset    *service.PasswordResetService
	EssaySubmission  *service.EssaySubmissionService
	InviteCode       *service.InviteCodeService
	QuestionNote     *service.QuestionNoteService
	AI               *service.AIService
	Membership       *service.MembershipService
	Plan             *service.PlanService
	User             *service.UserService
	Payment          *service.PaymentService
	CardKey          *service.CardKeyService
	Daily            *service.DailyService
	ExamSchedule     *service.ExamScheduleService
	Storage          *service.StorageService
	LoginLock        *service.LoginLockService
	Site             *service.SiteService
}

func assembleServices(repos *Repos, redis *redis.Client) *Services {
	// 第 1 批:无内部依赖的 service
	userState := service.NewUserStateService(repos.User)
	question := service.NewQuestionService(repos.Question)
	practice := service.NewPracticeService(repos.Question, repos.WrongQuestion, repos.Practice,
		repos.Profile, repos.UserSubjectStat, repos.PracticeSession, repos.PracticeQuery)
	favorite := service.NewFavoriteService(repos.Favorite, repos.Question, repos.Article, repos.KnowledgePoint)
	knowledge := service.NewKnowledgeService(repos.KnowledgePoint)
	article := service.NewArticleService(repos.Article)
	announcement := service.NewAnnouncementService(repos.Announcement)
	notification := service.NewNotificationService(repos.Notification)
	passwordReset := service.NewPasswordResetService(repos.PasswordReset, repos.User)
	questionNote := service.NewQuestionNoteService(repos.QuestionNote, repos.Question)
	membership := service.NewMembershipService(repos.Membership)
	plan := service.NewPlanService(repos.MembershipPlan)
	inviteCode := service.NewInviteCodeService(repos.InviteCode)
	examSchedule := service.NewExamScheduleService(repos.ExamSchedule)
	storage := service.NewStorageService(config.AppConfig.OSS)
	loginLock := service.NewLoginLockService()
	// 站点开关：全局中间件每请求都会问它一次，进程内缓存（task-2）只有在
	// 实例长活时才有意义 —— 每次请求现构会让缓存和单飞锁双双失效。
	site := service.NewSiteService(repos.SiteSettings)

	// 第 2 批:有内部依赖的 service
	// userService 依赖 membership + notification(原 main.go 行 105 注释明确指出)
	user := service.NewUserService(repos.User, repos.Profile, membership, notification, repos.Subject)
	// examService 依赖 practice(给 GenerateMock 用历史题目);comment/report/feedback/essay 都依赖 notification
	exam := service.NewExamService(repos.Paper, repos.Question, repos.Exam, practice, repos.PaperSession)
	comment := service.NewCommentService(repos.Comment, repos.CommentReport, notification)
	report := service.NewQuestionReportService(repos.Report, notification)
	feedback := service.NewFeedbackService(repos.Feedback, repos.Question, notification)
	essaySubmission := service.NewEssaySubmissionService(repos.EssaySubmission, repos.Article, notification)
	ai := service.NewAIService(repos.AIConfig, repos.UserAIConfig, repos.AIConversation, repos.AIMessage,
		repos.AITrialUsage, repos.AICallLog, repos.User, repos.PracticeQuery)
	payment := service.NewPaymentService(repos.Order, membership, plan)
	cardKey := service.NewCardKeyService(repos.CardKey, membership, plan)
	daily := service.NewDailyService(repos.DailyCheckin, repos.Question, repos.Profile, repos.DailySession)
	admin := service.NewAdminService(repos.User, repos.Question, repos.Profile, repos.Membership,
		repos.Dashboard, userState)

	return &Services{
		UserState:       userState,
		Question:        question,
		Practice:        practice,
		Favorite:        favorite,
		Knowledge:       knowledge,
		Article:         article,
		Exam:            exam,
		Admin:           admin,
		Announcement:    announcement,
		Notification:    notification,
		Comment:         comment,
		Report:          report,
		Feedback:        feedback,
		PasswordReset:   passwordReset,
		EssaySubmission: essaySubmission,
		InviteCode:      inviteCode,
		QuestionNote:    questionNote,
		AI:              ai,
		Membership:      membership,
		Plan:            plan,
		User:            user,
		Payment:         payment,
		CardKey:         cardKey,
		Daily:           daily,
		ExamSchedule:    examSchedule,
		Storage:         storage,
		LoginLock:       loginLock,
		Site:            site,
	}
}

// Handlers HTTP 入口层。每个 handler 依赖的 service 由 assembleServices 已经装好。
type Handlers struct {
	User            *handler.UserHandler
	Question        *handler.QuestionHandler
	Practice        *handler.PracticeHandler
	Favorite        *handler.FavoriteHandler
	Knowledge       *handler.KnowledgeHandler
	Article         *handler.ArticleHandler
	Exam            *handler.ExamHandler
	Admin           *handler.AdminHandler
	Announcement    *handler.AnnouncementHandler
	Report          *handler.QuestionReportHandler
	Feedback        *handler.FeedbackHandler
	Notification    *handler.NotificationHandler
	PasswordReset   *handler.PasswordResetHandler
	OAuth           *handler.OAuthHandler
	EssaySubmission *handler.EssaySubmissionHandler
	InviteCode      *handler.InviteCodeHandler
	QuestionNote    *handler.QuestionNoteHandler
	AI              *handler.AIHandler
	Payment         *handler.PaymentHandler
	AntiCrawl       *handler.AntiCrawlHandler
	CardKey         *handler.CardKeyHandler
	Daily           *handler.DailyHandler
	Upload          *handler.UploadHandler
	Comment         *handler.CommentHandler
	ExamSchedule    *handler.ExamScheduleHandler
	SEO             *handler.SEOHandler
	Site            *handler.SiteHandler
}

func assembleHandlers(svc *Services, repos *Repos) *Handlers {
	// SEO 单独构造:它依赖 knowledge + article service,在 main 里原本也内联现构。
	seo := handler.NewSEOHandler(svc.Knowledge, svc.Article)

	// subjects 路由不走 handler——直接调 subjectRepo.List()。
	// 这里把它包成匿名函数 handler,避免 main.go 出现"绕过 Handlers 结构"的特例。
	_ = repos // 留给后续 subjects 路由直接取 repo 用

	return &Handlers{
		User:            handler.NewUserHandler(svc.User, svc.InviteCode, svc.Practice, svc.LoginLock),
		Question:        handler.NewQuestionHandler(svc.Question),
		Practice:        handler.NewPracticeHandler(svc.Practice),
		Favorite:        handler.NewFavoriteHandler(svc.Favorite),
		Knowledge:       handler.NewKnowledgeHandler(svc.Knowledge),
		Article:         handler.NewArticleHandler(svc.Article),
		Exam:            handler.NewExamHandler(svc.Exam, svc.Membership),
		Admin:           handler.NewAdminHandler(svc.Admin),
		Announcement:    handler.NewAnnouncementHandler(svc.Announcement),
		Report:          handler.NewQuestionReportHandler(svc.Report),
		Feedback:        handler.NewFeedbackHandler(svc.Feedback),
		Notification:    handler.NewNotificationHandler(svc.Notification),
		PasswordReset:   handler.NewPasswordResetHandler(svc.PasswordReset),
		OAuth:           handler.NewOAuthHandler(svc.User),
		EssaySubmission: handler.NewEssaySubmissionHandler(svc.EssaySubmission),
		InviteCode:      handler.NewInviteCodeHandler(svc.InviteCode),
		QuestionNote:    handler.NewQuestionNoteHandler(svc.QuestionNote),
		AI:              handler.NewAIHandler(svc.AI, svc.Membership),
		Payment:         handler.NewPaymentHandler(svc.Payment, svc.Membership, svc.Plan),
		AntiCrawl:       handler.NewAntiCrawlHandler(),
		CardKey:         handler.NewCardKeyHandler(svc.CardKey),
		Daily:           handler.NewDailyHandler(svc.Daily),
		Upload:          handler.NewUploadHandler(svc.Storage),
		Comment:         handler.NewCommentHandler(svc.Comment),
		ExamSchedule:    handler.NewExamScheduleHandler(svc.ExamSchedule),
		SEO:             seo,
		Site:            handler.NewSiteHandler(svc.Site),
	}
}

// Middlewares 中间件。AuthMiddleware 需要 UserStateService,RequireMembership 需要 Membership,
// AIQuota 需要 Membership + AI。RateLimit 系列无依赖。
type Middlewares struct {
	Auth           gin.HandlerFunc
	OptionalAuth   gin.HandlerFunc
	RequireMember  gin.HandlerFunc
	AIQuota        gin.HandlerFunc
	Admin          gin.HandlerFunc
	Cors           gin.HandlerFunc
	RateGlobal     gin.HandlerFunc
	RateData       gin.HandlerFunc
	RateIP         gin.HandlerFunc
	RatePublic     gin.HandlerFunc
	RateAction     func(name string, perMinute int) gin.HandlerFunc
	RateAI         gin.HandlerFunc // AI 生成接口专用限额
	AIConcurrency  gin.HandlerFunc // AI 生成接口在途并发上限
	TrackQA        gin.HandlerFunc // 题库详情页统计
	Maintenance    gin.HandlerFunc // 系统维护闸门
}

func assembleMiddlewares(svc *Services, redis *redis.Client) *Middlewares {
	return &Middlewares{
		Auth:          middleware.AuthMiddleware(svc.UserState),
		OptionalAuth:  middleware.OptionalAuth(svc.UserState),
		RequireMember: middleware.RequireMembership(svc.Membership),
		AIQuota:       middleware.AIQuota(svc.Membership, svc.AI),
		Admin:         middleware.AdminMiddleware(),
		Cors:          middleware.CorsMiddleware(),
		RateGlobal:    middleware.RateLimitGlobal(),
		RateData:      middleware.RateLimitData(),
		RateIP:        middleware.RateLimitByIP(),
		RatePublic:    middleware.RateLimitPublic(),
		RateAction:    middleware.RateLimitAction,
		RateAI:        middleware.RateLimitAI(),
		AIConcurrency: middleware.AIConcurrency(),
		TrackQA:       middleware.TrackQuestionAccess(),
		Maintenance:   middleware.Maintenance(svc.Site, svc.UserState),
	}
}
