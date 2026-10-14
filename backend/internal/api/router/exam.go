package router

import (
	"github.com/gin-gonic/gin"

	"software-exam/cmd/server/wire"
	"software-exam/pkg/response"
)

// registerExam 考试域:
//   - /api/exams:试卷列表/详情(已登录,数据接口限流)
//   - /api/exam:开始/提交/历史/模拟/练习会话(已登录)
//   - /api/subjects:科目列表(游客开放,布局组件每页都会拉,挂 authMW 会让游客触发 401)
//   - /api/exam-schedules:考试排期(游客开放,倒计时 chip 每页拉)
//
// subjects 直接调 subjectRepo.List()(原 main.go 行 422-429 inline 写法),
// 不走 handler。repos 由 NewRouter 透传到这里,避免在 Handlers struct 里塞
// 临时 anon handler。
func registerExam(r *gin.Engine, h *wire.Handlers, mw *wire.Middlewares, repos *wire.Repos) {
	exams := r.Group("/api/exams")
	exams.Use(mw.Auth, mw.RateGlobal, mw.RateData)
	{
		exams.GET("/papers", h.Exam.ListPapers)
		exams.GET("/papers/:id", h.Exam.GetPaper)
	}

	subjects := r.Group("/api/subjects")
	subjects.Use(mw.OptionalAuth, mw.RatePublic)
	subjects.GET("", func(c *gin.Context) {
		list, err := repos.Subject.List()
		if err != nil {
			response.ServerError(c, err.Error())
			return
		}
		response.Success(c, list)
	})

	examSchedules := r.Group("/api/exam-schedules")
	examSchedules.Use(mw.OptionalAuth, mw.RatePublic)
	examSchedules.GET("", h.ExamSchedule.List)

	examUser := r.Group("/api/exam")
	examUser.Use(mw.Auth, mw.RateGlobal)
	{
		examUser.POST("/start/:id", h.Exam.StartExam)
		examUser.POST("/submit", h.Exam.SubmitExam)
		examUser.GET("/records", h.Exam.GetExamRecords)
		examUser.POST("/generate-mock", h.Exam.GenerateMock)
		examUser.POST("/practice-session", h.Exam.SavePracticeSession)
		examUser.GET("/practice-session", h.Exam.GetPracticeSession)
		examUser.DELETE("/practice-session", h.Exam.ClearPracticeSession)
	}
}
