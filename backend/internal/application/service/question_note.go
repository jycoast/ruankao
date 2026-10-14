package service

import (
	"errors"
	"regexp"
	"strings"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"

	"gorm.io/gorm"
)

type QuestionNoteService struct {
	repo         repository.QuestionNoteRepository
	questionRepo repository.QuestionRepository
}

func NewQuestionNoteService(repo repository.QuestionNoteRepository, questionRepo repository.QuestionRepository) *QuestionNoteService {
	return &QuestionNoteService{repo: repo, questionRepo: questionRepo}
}

func (s *QuestionNoteService) Get(userID, questionID uint64) (*model.QuestionNote, error) {
	return s.repo.FindByUserAndQuestion(userID, questionID)
}

func (s *QuestionNoteService) Save(userID, questionID uint64, content string) error {
	// 后端 sanitize 兜底：剥危险协议/标签，限定 <img src> 仅允许 https:/data:
	// 前端已 sanitize 一道（QuestionNoteEditor handleSave），此处再做一次保底：
	// 直接 PUT 旧笔记/绕过前端的攻击面都会被拦。
	return s.repo.Upsert(&model.QuestionNote{
		UserID:     userID,
		QuestionID: questionID,
		Content:    sanitizeNoteHTML(content),
	})
}

func (s *QuestionNoteService) Delete(userID, questionID uint64) error {
	return s.repo.Delete(userID, questionID)
}

// ListByUser 分页返回用户的笔记（含对应题目概要），按更新时间倒序；keyword 模糊匹配笔记/题干/套卷。
func (s *QuestionNoteService) ListByUser(userID uint64, page, pageSize int, keyword string) ([]model.NoteWithQuestion, int64, error) {
	return s.repo.ListByUser(userID, page, pageSize, keyword)
}

// ErrNoteNotFound 该用户在这个题目上没有笔记。
// handler 靠它回 404 —— 这个 404 是接口的安全边界，不能退化成「空对象 + 200」，
// 否则调用方分不清「不是我的笔记」和「是空的笔记」。
var ErrNoteNotFound = errors.New("笔记不存在")

// Detail 返回「我的笔记」单条详情：笔记归属信息 + 完整题目（含选项）。
//
// 顺序不能颠倒：**先查笔记，查不到就直接返回，不去碰题目表**。
// 这保证了接口不会对「没写过笔记的题」吐题面。
//
// 但别把这条读成「无法枚举题库」：写一条笔记再读详情，两步都不经过 TrackQA，
// 所以逐题「写→读」仍能取到题面。这条残余风险是有意接受的（缓解：写请求受
// RateData 限速约束，且会留下 question_notes 记录），详见 spec 的「反爬边界」一节。
func (s *QuestionNoteService) Detail(userID, questionID uint64) (*model.NoteWithQuestion, *model.Question, error) {
	note, err := s.repo.FindWithQuestion(userID, questionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrNoteNotFound
		}
		return nil, nil, err
	}
	question, err := s.questionRepo.FindByID(questionID)
	if err != nil {
		return nil, nil, err
	}
	return note, question, nil
}

// 危险协议：javascript:/vbscript:/data:text-html 等，前端即使 sanitize 也要再过一道。
// RE2 不支持 lookahead，所以用消费式匹配：把 scheme 后的整段 payload（到下一个引号/空白）
// 一并吃掉，再替换成 =""。
//
// 分两支：单段 scheme（javascript: / vbscript: / mocha: / livescript:）需要结尾的 :；
// data:URL 自带 :（data:text/html、data:application/javascript）就不重复要求。
// 合法 URL 像 /javascript-faq.html 不会走 scheme 分支（其"javascript"后跟 - 而不是 :），
// 也不会被 data 分支匹配。
var noteDangerousSchemeRe = regexp.MustCompile(`(?i)(\s+(?:href|src|xlink:href|formaction)\s*=\s*["']?\s*)(?:(?:javascript|vbscript|mocha|livescript):|(?:data\s*:\s*(?:text/html|application/javascript)))[^"'\s]*`)

// 危险事件属性（onclick/onerror/...）。正则匹配属性名形式。
var noteDangerousEventRe = regexp.MustCompile(`(?i)\s+on[a-z]+\s*=\s*["'][^"']*["']`)

// noteImgSrcRe 把 <img src="..."> 抽出来检查白名单 scheme。
var noteImgSrcRe = regexp.MustCompile(`(?i)<img\b[^>]*\bsrc\s*=\s*["']([^"']+)["']`)

// sanitizeNoteHTML 服务端最简白名单 sanitize：剔除事件属性与危险协议，
// 把 <img> 的 src 限制到 https:/data:image/...。富文本结构由前端 sanitize 保障。
// 这里用纯正则保守处理，避免引入 html 解析库的服务端依赖。
func sanitizeNoteHTML(in string) string {
	if in == "" {
		return ""
	}
	// 1) 剥 on* 事件属性
	out := noteDangerousEventRe.ReplaceAllString(in, "")
	// 2) 抽 <img> 的 src，非白名单的 src 清空（保持 <img> 标签结构，避免被编辑器误删）
	out = noteImgSrcRe.ReplaceAllStringFunc(out, func(tag string) string {
		m := noteImgSrcRe.FindStringSubmatch(tag)
		if len(m) < 2 {
			return tag
		}
		src := strings.TrimSpace(m[1])
		if isSafeImgSrc(src) {
			return tag
		}
		// 直接拼出干净的 <img src="">：避免用 $1 反向引用（语义混乱、且会把 src 拼回去）
		return `<img src="">`
	})
	// 3) 剥剩余危险协议（href/src/xlink:href/formaction 上的 javascript: 等）
	//    替换为 $1=""：保留属性名前缀（ href=），把整段 scheme:payload 清空。
	out = noteDangerousSchemeRe.ReplaceAllString(out, `$1=""`)
	return out
}

// isSafeImgSrc 限定 https:// 与 data:image/...（前端已收窄；服务端再保底）。
func isSafeImgSrc(src string) bool {
	low := strings.ToLower(src)
	if strings.HasPrefix(low, "https://") {
		return true
	}
	if strings.HasPrefix(low, "data:image/") {
		return true
	}
	return false
}
