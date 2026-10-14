package handler

import (
	"software-exam/internal/application/service"
	"software-exam/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type FavoriteHandler struct {
	favoriteService *service.FavoriteService
}

func NewFavoriteHandler(favoriteService *service.FavoriteService) *FavoriteHandler {
	return &FavoriteHandler{favoriteService: favoriteService}
}

type AddFavoriteRequest struct {
	TargetType int8   `json:"target_type" binding:"required"`
	TargetID   uint64 `json:"target_id" binding:"required"`
}

func (h *FavoriteHandler) Add(c *gin.Context) {
	userID := c.GetUint64("user_id")
	var req AddFavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.favoriteService.Add(userID, req.TargetType, req.TargetID); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *FavoriteHandler) Remove(c *gin.Context) {
	userID := c.GetUint64("user_id")
	targetType, _ := strconv.Atoi(c.Param("target_type"))
	targetID, _ := strconv.ParseUint(c.Param("target_id"), 10, 64)
	if err := h.favoriteService.Remove(userID, int8(targetType), targetID); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *FavoriteHandler) List(c *gin.Context) {
	userID := c.GetUint64("user_id")
	targetType, _ := strconv.Atoi(c.DefaultQuery("target_type", "1"))
	page, pageSize := parsePage(c)
	favs, total, err := h.favoriteService.List(userID, int8(targetType), page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.SuccessWithPage(c, total, page, pageSize, favs)
}

func (h *FavoriteHandler) Check(c *gin.Context) {
	userID := c.GetUint64("user_id")
	targetType, _ := strconv.Atoi(c.Query("target_type"))
	targetID, _ := strconv.ParseUint(c.Query("target_id"), 10, 64)
	exists, err := h.favoriteService.Exists(userID, int8(targetType), targetID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"exists": exists})
}
