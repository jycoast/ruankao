package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type PageData struct {
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Items    interface{} `json:"items"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func SuccessWithPage(c *gin.Context, total int64, page, pageSize int, items interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: PageData{
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			Items:    items,
		},
	})
}

// BizErrorKey 业务失败标记。HTTP 状态恒为 200，中间件（如 AI 配额）靠它识别失败。
const BizErrorKey = "biz_error"

func Error(c *gin.Context, code int, message string) {
	c.Set(BizErrorKey, true)
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
	})
}

// ErrorWithData 与 Error 同语义（HTTP 恒为 200、置 BizErrorKey 标记），额外带一个 data 载荷。
//
// 给「业务码不够用」的场景：同一个 code 被多个 handler 复用，前端需要区分是谁发的。
// 目前唯一的使用者是维护中间件 —— 它附上 {maintenance:true} 判别位，因为 503
// 不是维护专属（handler/upload.go 在对象存储未配置时也回 503），前端只凭 code
// 分支会把「传图失败」误当成「站点维护」硬跳维护页。
func ErrorWithData(c *gin.Context, code int, message string, data interface{}) {
	c.Set(BizErrorKey, true)
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

func BadRequest(c *gin.Context, message string) {
	Error(c, 400, message)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, 401, message)
}

func Forbidden(c *gin.Context, message string) {
	Error(c, 403, message)
}

func NotFound(c *gin.Context, message string) {
	Error(c, 404, message)
}

func ServerError(c *gin.Context, message string) {
	Error(c, 500, message)
}
