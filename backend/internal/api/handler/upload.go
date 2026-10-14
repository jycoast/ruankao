package handler

import (
	"errors"

	"software-exam/internal/application/service"
	"software-exam/pkg/response"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	storageService *service.StorageService
}

func NewUploadHandler(storageService *service.StorageService) *UploadHandler {
	return &UploadHandler{storageService: storageService}
}

// Image 接收 multipart 表单里的单张图片，返回可访问 URL。
// 供后台编辑器粘贴上传使用，仅管理员可调。
func (h *UploadHandler) Image(c *gin.Context) {
	h.handle(c, "") // admin 默认目录
}

// ImageForUser 普通用户上传通道。subDir="note" → 落到 uploads/note/ 下与 admin 区分。
// 仅 Auth + RateGlobal + RateData 守卫，不要求 Admin/会员。
func (h *UploadHandler) ImageForUser(c *gin.Context) {
	h.handle(c, "note")
}

func (h *UploadHandler) handle(c *gin.Context, subDir string) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "未接收到图片")
		return
	}
	// 先按 multipart 头里的大小挡一道，避免大文件白白读进来
	if fh.Size > h.storageService.MaxSize() {
		response.BadRequest(c, "图片超过大小限制")
		return
	}
	f, err := fh.Open()
	if err != nil {
		response.ServerError(c, "读取图片失败")
		return
	}
	defer f.Close()

	url, err := h.storageService.UploadImage(c.Request.Context(), f, fh.Size, fh.Header.Get("Content-Type"), subDir)
	if err != nil {
		if errors.Is(err, service.ErrStorageNotConfigured) {
			response.Error(c, 503, "图片上传未启用：管理员需先配置对象存储")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"url": url})
}
