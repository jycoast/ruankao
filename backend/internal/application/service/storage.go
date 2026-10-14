package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"software-exam/config"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// ErrStorageNotConfigured OSS 未配置。上传接口据此返回可读提示，
// 而不是让 SDK 抛出一堆签名错误。
var ErrStorageNotConfigured = errors.New("对象存储未配置")

// allowedImageTypes 允许上传的图片类型 → 扩展名。
// 白名单而非黑名单：只认这几种能被浏览器安全内联渲染的位图格式。
// 特意不含 svg —— SVG 可内嵌 <script>，从图床域名加载等于给自己开了个 XSS 入口。
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
	"image/bmp":  ".bmp",
}

const (
	defaultMaxUploadMB = 5
	defaultUploadDir   = "uploads"
)

// StorageService 腾讯云 COS 图片上传。
type StorageService struct {
	client  *cos.Client
	baseURL string // 不带结尾 /
	dir     string // 不带首尾 /
	maxSize int64  // 字节
}

// NewStorageService 未配置（缺 key/bucket/region）时返回一个 client 为 nil 的实例，
// 服务照常启动，只是上传接口会明确报「未配置」。
// 这样本地开发和没有图床需求的部署都不必被迫填 OSS。
func NewStorageService(cfg config.OSSConfig) *StorageService {
	s := &StorageService{
		dir:     strings.Trim(strings.TrimSpace(cfg.Dir), "/"),
		maxSize: int64(cfg.MaxSize) * 1024 * 1024,
	}
	if s.dir == "" {
		s.dir = defaultUploadDir
	}
	if s.maxSize <= 0 {
		s.maxSize = defaultMaxUploadMB * 1024 * 1024
	}

	bucketURL := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if bucketURL == "" && cfg.Bucket != "" && cfg.Region != "" {
		bucketURL = fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cfg.Bucket, cfg.Region)
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" || bucketURL == "" {
		return s
	}
	u, err := url.Parse(bucketURL)
	if err != nil {
		return s
	}

	s.baseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if s.baseURL == "" {
		s.baseURL = bucketURL
	}
	s.client = cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Timeout: 30 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.AccessKey,
			SecretKey: cfg.SecretKey,
		},
	})
	return s
}

func (s *StorageService) Enabled() bool { return s.client != nil }

func (s *StorageService) MaxSize() int64 { return s.maxSize }

// UploadImage 上传图片并返回可公开访问的 URL。
// contentType 取自浏览器上报的 MIME，仅用于选扩展名和挡掉非图片；
// 真正的安全边界是白名单本身 —— 即便类型被伪造，存进去的也只会是
// 一个以图片扩展名结尾、Content-Type 被我们改写过的对象，不会被当脚本执行。
// subDir 用于按用途分子目录（admin/note 等），防止子路径穿越（".."、"/" 等会被去掉）。
func (s *StorageService) UploadImage(ctx context.Context, r io.Reader, size int64, contentType string, subDir string) (string, error) {
	if s.client == nil {
		return "", ErrStorageNotConfigured
	}
	ext, ok := allowedImageTypes[normalizeMIME(contentType)]
	if !ok {
		return "", errors.New("仅支持 jpg / png / gif / webp / bmp 格式的图片")
	}
	if size <= 0 {
		return "", errors.New("图片内容为空")
	}
	if size > s.maxSize {
		return "", fmt.Errorf("图片超过 %d MB 限制", s.maxSize/1024/1024)
	}

	key := s.objectKey(ext, subDir)
	// LimitReader 兜底：size 来自 multipart 头，不可全信，
	// 真实流超长时截断而不是让它写满存储桶。
	_, err := s.client.Object.Put(ctx, key, io.LimitReader(r, s.maxSize), &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType:  normalizeMIME(contentType),
			CacheControl: "public, max-age=31536000",
		},
	})
	if err != nil {
		return "", fmt.Errorf("上传失败: %w", err)
	}
	return s.baseURL + "/" + key, nil
}

// objectKey 按日期分目录 + 随机名。
// 随机名而非原文件名：粘贴上传本就没有文件名，且可避免覆盖与路径穿越。
// subDir 非空时拼在 s.dir 之后（已 sanitize，去掉 "/" 与 ".."）。
func (s *StorageService) objectKey(ext, subDir string) string {
	var b [8]byte
	// crypto/rand 在标准环境下不会失败；退化时用时间戳兜底，避免 panic
	if _, err := rand.Read(b[:]); err != nil {
		now := time.Now()
		return path.Join(s.dir, sanitizeSubDir(subDir), now.Format("2006/01/02"),
			fmt.Sprintf("%d%s", now.UnixNano(), ext))
	}
	now := time.Now()
	return path.Join(s.dir, sanitizeSubDir(subDir), now.Format("2006/01/02"),
		fmt.Sprintf("%d_%s%s", now.UnixNano(), hex.EncodeToString(b[:]), ext))
}

// sanitizeSubDir 防御性清洗：去除前导/、路径分隔符与 ".."，避免拼接后跑出 s.dir。
// 按段解析：`..` 弹掉前一段（栈空时静默吞掉），最终 join。段长上限 32。
// 示例：note/u/../../etc → etc；a/b/../c → a/c；../escape → escape。
func sanitizeSubDir(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "/")
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "/")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "." {
			continue
		}
		if p == ".." {
			if len(clean) > 0 {
				clean = clean[:len(clean)-1]
			}
			continue
		}
		if len(p) > 32 {
			p = p[:32]
		}
		clean = append(clean, p)
	}
	return strings.Join(clean, "/")
}

// normalizeMIME 去掉 "image/png; charset=..." 这类参数并转小写
func normalizeMIME(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.ToLower(strings.TrimSpace(ct))
}
