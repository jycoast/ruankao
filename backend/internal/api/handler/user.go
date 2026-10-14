package handler

import (
	"errors"
	"fmt"
	"net/http"
	"software-exam/internal/application/service"
	"software-exam/internal/domain/model"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type UserHandler struct {
	userService       *service.UserService
	inviteCodeService *service.InviteCodeService
	practiceService   *service.PracticeService
	loginLock         *service.LoginLockService
}

func NewUserHandler(userService *service.UserService, inviteCodeService *service.InviteCodeService, practiceService *service.PracticeService, loginLock *service.LoginLockService) *UserHandler {
	return &UserHandler{userService: userService, inviteCodeService: inviteCodeService, practiceService: practiceService, loginLock: loginLock}
}

type RegisterRequest struct {
	Username    string `json:"username" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=6"`
	Nickname    string `json:"nickname"`
	InviteCode  string `json:"invite_code"`
	CaptchaID   string `json:"captcha_id" binding:"required"`
	CaptchaCode string `json:"captcha_code" binding:"required"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

func translateRegisterErr(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, e := range ve {
			switch e.Field() {
			case "Username":
				return "用户名不能为空"
			case "Email":
				if e.Tag() == "required" {
					return "邮箱不能为空"
				}
				return "邮箱格式不正确，请输入有效的邮箱地址"
			case "Password":
				if e.Tag() == "required" {
					return "密码不能为空"
				}
				return "密码长度不能少于6位"
			}
		}
	}
	return "参数错误，请检查输入"
}

func translateChangePasswordErr(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, e := range ve {
			switch e.Field() {
			case "NewPassword":
				if e.Tag() == "required" {
					return "请输入新密码"
				}
				return "新密码长度不能少于6位"
			}
		}
	}
	return "参数错误，请检查输入"
}

func (h *UserHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, translateRegisterErr(err))
		return
	}

	// 图形验证码：防机器批量注册
	if !VerifyCaptcha(req.CaptchaID, req.CaptchaCode) {
		response.Error(c, 400, "验证码错误或已过期，请刷新重试")
		return
	}

	// 验证邀请码（可选）
	if req.InviteCode != "" {
		if err := h.inviteCodeService.Validate(req.InviteCode); err != nil {
			response.Error(c, 400, err.Error())
			return
		}
	}

	token, userID, err := h.userService.Register(req.Username, req.Email, req.Password, req.Nickname)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	// 标记邀请码已使用
	if req.InviteCode != "" {
		_ = h.inviteCodeService.UseCode(req.InviteCode, userID)
	}

	response.Success(c, gin.H{"token": token})
}

func (h *UserHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	ip := c.ClientIP()
	if h.loginLock != nil {
		if locked, retryAfter := h.loginLock.Check(req.Username, ip); locked {
			rejectLoginLocked(c, retryAfter)
			return
		}
	}

	token, err := h.userService.Login(req.Username, req.Password)
	if err != nil {
		if h.loginLock != nil {
			if justLocked, retryAfter := h.loginLock.RecordFailure(req.Username, ip); justLocked {
				rejectLoginLocked(c, retryAfter)
				return
			}
		}
		response.Error(c, 400, err.Error())
		return
	}

	if h.loginLock != nil {
		h.loginLock.ClearUser(req.Username)
	}
	response.Success(c, gin.H{"token": token})
}

// GuestLogin POST /api/auth/guest —— 游客体验入口:无账号密码验证码,直接发游客令牌。
func (h *UserHandler) GuestLogin(c *gin.Context) {
	token, err := h.userService.GuestLogin()
	if err != nil {
		response.ServerError(c, "游客登录失败，请稍后重试")
		return
	}
	response.Success(c, gin.H{"token": token})
}

func rejectLoginLocked(c *gin.Context, retryAfter int) {
	if retryAfter <= 0 {
		retryAfter = 1
	}
	c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
	minutes := (retryAfter + 59) / 60
	msg := fmt.Sprintf("登录尝试过多，请 %d 分钟后再试", minutes)
	if minutes <= 1 {
		msg = "登录尝试过多，请稍后再试"
	}
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"code":    429,
		"message": msg,
		"data": gin.H{
			"retry_after_sec": retryAfter,
		},
	})
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := c.GetUint64("user_id")
	if userID == 0 {
		// 虚拟游客(role 2)没有 DB 账号：直调个人资料回 401 而非 500（不影响真用户）
		response.Error(c, 401, "游客账号无个人资料，请先登录")
		return
	}
	profile, err := h.userService.GetProfile(userID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, profile)
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req struct {
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar"`
		Phone    string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.userService.UpdateProfile(userID, req.Nickname, req.Avatar, req.Phone); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// UpdateSubject PUT /api/user/subject —— 登录用户科目偏好落库（跨设备同步）
func (h *UserHandler) UpdateSubject(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req struct {
		SubjectID uint64 `json:"subject_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	subjectID, err := h.userService.UpdateSubject(userID, req.SubjectID)
	if err != nil {
		response.BadRequest(c, "科目不存在或未启用")
		return
	}
	response.Success(c, gin.H{"subject_id": subjectID})
}

func (h *UserHandler) GetStats(c *gin.Context) {
	userID := c.GetUint64("user_id")
	profile, err := h.userService.GetStats(userID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	// profile 里的 total_correct/total_practice 是每次提交都 +1 的累加值，
	// 反复刷同一题能把正确率刷到接近 100%。首页展示的正确率改用与学习报告
	// 相同的口径（每题只算最近一次、只算客观题），两处不再各算各的。
	acc := h.practiceService.GetAccuracy(userID)
	response.Success(c, struct {
		*model.UserProfile
		service.AccuracyStat
	}{profile, acc})
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, translateChangePasswordErr(err))
		return
	}
	userID := c.GetUint64("user_id")
	if err := h.userService.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, nil)
}
