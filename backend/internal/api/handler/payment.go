package handler

import (
	"log"
	"net/http"
	"strconv"

	"software-exam/internal/application/service"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	paySvc        *service.PaymentService
	membershipSvc *service.MembershipService
	planSvc       *service.PlanService
}

func NewPaymentHandler(paySvc *service.PaymentService, membershipSvc *service.MembershipService, planSvc *service.PlanService) *PaymentHandler {
	return &PaymentHandler{paySvc: paySvc, membershipSvc: membershipSvc, planSvc: planSvc}
}

// Plans GET /api/membership/plans —— 在售套餐
func (h *PaymentHandler) Plans(c *gin.Context) {
	plans, err := h.paySvc.ListPlans()
	if err != nil {
		response.ServerError(c, "获取套餐失败")
		return
	}
	response.Success(c, plans)
}

// PayChannels GET /api/membership/channels —— 当前可用的支付通道
// 前端据此决定展示哪些支付按钮：未配置商户信息的通道不该露出来让用户点了才报错。
func (h *PaymentHandler) PayChannels(c *gin.Context) {
	response.Success(c, h.paySvc.AvailableChannels())
}

// Status GET /api/membership/status
func (h *PaymentHandler) Status(c *gin.Context) {
	userID := c.GetUint64("user_id")
	response.Success(c, h.membershipSvc.Status(userID))
}

// CreateOrder POST /api/membership/orders
func (h *PaymentHandler) CreateOrder(c *gin.Context) {
	var req struct {
		Plan string `json:"plan" binding:"required"`
		Type string `json:"type" binding:"required"` // alipay / wxpay
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	userID := c.GetUint64("user_id")
	payURL, outTradeNo, err := h.paySvc.CreateOrder(userID, req.Plan, req.Type)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"pay_url": payURL, "out_trade_no": outTradeNo})
}

// QueryOrder GET /api/membership/orders/:out_trade_no
func (h *PaymentHandler) QueryOrder(c *gin.Context) {
	userID := c.GetUint64("user_id")
	order, err := h.paySvc.QueryOrder(userID, c.Param("out_trade_no"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"status": order.Status, "paid": order.Status == 1})
}

// ListOrders GET /api/membership/orders —— 当前用户的订单记录
func (h *PaymentHandler) ListOrders(c *gin.Context) {
	userID := c.GetUint64("user_id")
	page, pageSize := parsePage(c)
	list, total, err := h.paySvc.ListOrders(userID, page, pageSize)
	if err != nil {
		response.ServerError(c, "查询订单失败")
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}

// Notify 处理支付异步回调（无需登录）。channel 由路由绑定，不接受回调方自报，
// 因为两条通道密钥不同，验签必须用对应的那把。
// 协议要求：成功时返回纯文本 "success"，其它返回 "fail"
//
// 各家易支付分支实现不一：多数用 GET query，也有 POST form 的。
// 只解析其中一种，另一种就是「用户付了钱拿不到会员」，所以两种都收。
func (h *PaymentHandler) notify(c *gin.Context, channel string) {
	params := map[string]string{}
	// ParseForm 会把 query 和 POST body 一起填进 c.Request.Form
	_ = c.Request.ParseForm()
	for k, v := range c.Request.Form {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	if err := h.paySvc.HandleNotify(channel, params); err != nil {
		log.Printf("[pay] notify 处理失败 channel=%s out_trade_no=%s: %v", channel, params["out_trade_no"], err)
		c.String(http.StatusOK, "fail")
		return
	}
	c.String(http.StatusOK, "success")
}

// Notify GET|POST /api/pay/notify —— 易支付异步回调
func (h *PaymentHandler) Notify(c *gin.Context) { h.notify(c, service.ChannelYiPay) }

// NotifyLDC GET|POST /api/pay/ldc/notify —— LINUX DO Credit 异步回调
func (h *PaymentHandler) NotifyLDC(c *gin.Context) { h.notify(c, service.ChannelLDC) }

// NotifyYueTongYun GET|POST /api/pay/ytpay/notify —— 悦通云异步回调
func (h *PaymentHandler) NotifyYueTongYun(c *gin.Context) { h.notify(c, service.ChannelYueTongYun) }

// AdminGrant POST /api/admin/memberships/grant —— 管理员手动开通会员
func (h *PaymentHandler) AdminGrant(c *gin.Context) {
	var req struct {
		UserID uint64 `json:"user_id" binding:"required"`
		Days   int    `json:"days" binding:"required,min=1"`
		Plan   string `json:"plan"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	plan := req.Plan
	if plan == "" {
		plan = "manual"
	}
	if err := h.membershipSvc.Grant(req.UserID, plan, req.Days); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"status": h.membershipSvc.Status(req.UserID)})
}

// AdminListPlans GET /api/admin/membership/plans —— 含已下架套餐
func (h *PaymentHandler) AdminListPlans(c *gin.Context) {
	plans, err := h.planSvc.ListAll()
	if err != nil {
		response.ServerError(c, "获取套餐失败")
		return
	}
	response.Success(c, plans)
}

// AdminUpdatePlan PUT /api/admin/membership/plans/:key —— 调价 / 上下架
func (h *PaymentHandler) AdminUpdatePlan(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		Amount    int    `json:"amount" binding:"min=0"`
		LDCAmount int    `json:"ldc_amount" binding:"min=0"`
		SortOrder int    `json:"sort_order"`
		Enabled   int8   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.planSvc.UpdatePrice(c.Param("key"), req.Name, req.Amount, req.LDCAmount, req.SortOrder, req.Enabled); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// AdminListOrders GET /api/admin/orders 管理端订单列表（含下单用户名，支持筛查）
func (h *PaymentHandler) AdminListOrders(c *gin.Context) {
	page, pageSize := parsePage(c)
	// status: -1=全部 0=待支付 1=已支付
	status, _ := strconv.Atoi(c.DefaultQuery("status", "-1"))
	req := service.AdminOrderListReq{
		Page:     page,
		PageSize: pageSize,
		Keyword:  c.Query("keyword"),
		Status:   int8(status),
		Plan:     c.Query("plan"),
	}
	list, total, err := h.paySvc.ListAllOrders(req)
	if err != nil {
		response.ServerError(c, "查询订单失败")
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, list)
}
