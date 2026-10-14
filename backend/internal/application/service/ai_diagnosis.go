package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"software-exam/internal/domain/repository"
	"software-exam/pkg/aiclient"
)

// 会员专属「AI 诊断报告」（/my/diagnosis）的流式生成。
//
// 输入是 repo 聚合好的作答画像（按知识大类，每题最近一次）+ 最近错题样例，
// 组装成 prompt 交给上游生成个性化的薄弱点诊断与复习建议。
// 不做缓存、不落库：诊断是对当前统计的一次性解读，无历史回看需求。

// ErrDiagnosisNoData 表示用户还没有可诊断的数据（本科目没做过客观题）。
var ErrDiagnosisNoData = errors.New("暂无诊断数据，先完成几组刷题再来诊断吧")

const sysDiagnosis = `你是软考备考教练。请根据用户的学习数据诊断薄弱点并给出复习建议。
输出 Markdown，分节：「一句话诊断」「薄弱点画像」「分点复习建议」「优先行动清单」。
建议要具体可执行（结合知识点与真题考点，给出刷题+回顾导图的动作），不要空话。只输出正文，不要代码块围栏。用中文。`

// buildDiagnosisPrompt 把 repo 聚合行与错题样例组装成 user prompt。
// 纯函数：给定输入即有确定输出，便于单测；长文本按 rune 截断防 prompt 爆炸。
func buildDiagnosisPrompt(rows []repository.WeaknessRow, samples []repository.WrongSampleRow) string {
	var sb strings.Builder
	sb.WriteString("我最近的客观题作答情况（按知识大类，每题只算最近一次）：\n\n")
	if len(rows) == 0 {
		sb.WriteString("（暂无作答记录）\n")
	} else {
		sb.WriteString("| 知识大类 | 做题数 | 答对数 | 正确率 |\n")
		sb.WriteString("| --- | --- | --- | --- |\n")
		for _, r := range rows {
			acc := 0
			if r.Done > 0 {
				acc = int(float64(r.Correct) / float64(r.Done) * 100.0)
			}
			fmt.Fprintf(&sb, "| %s | %d | %d | %d%% |\n", r.Category, r.Done, r.Correct, acc)
		}
	}

	sb.WriteString("\n最近答错的题目（供参考）：\n")
	if len(samples) == 0 {
		sb.WriteString("（暂无）\n")
	} else {
		for i, s := range samples {
			fmt.Fprintf(&sb, "%d. %s（知识点：%s）", i+1, truncateRunes(s.Title, 60), s.KnowledgePoint)
			if s.Explanation != "" {
				sb.WriteString("。解析：" + truncateRunes(s.Explanation, 120))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n请根据以上数据，给我一份薄弱点诊断与针对性复习建议。")
	return sb.String()
}

// DiagnosisStream 流式生成诊断建议，逐块回调 onDelta。
// 无数据时返回 ErrDiagnosisNoData（handler 以 SSE error 帧下发，不调上游）。
// 返回值 truncated 含义同 ChatStream。
func (s *AIService) DiagnosisStream(ctx context.Context, userID, subjectID uint64, onDelta func(string) error) (bool, error) {
	cfg, err := s.resolveConfig(userID)
	if err != nil {
		return false, err
	}
	rows, err := s.practiceQueryRepo.CategoryAccuracyBySubject(userID, subjectID)
	if err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, ErrDiagnosisNoData
	}
	// 样例拉取失败不阻断主体诊断：给不了样例就少给点上下文。
	samples, _ := s.practiceQueryRepo.RecentWrongSamplesBySubject(userID, subjectID, 5)

	prompt := buildDiagnosisPrompt(rows, samples)
	messages := []aiclient.Message{
		{Role: "system", Content: sysDiagnosis},
		{Role: "user", Content: prompt},
	}
	var sb strings.Builder
	start := time.Now()
	res, streamErr := aiclient.ChatCompletionStream(ctx, cfg, messages, func(delta string) error {
		sb.WriteString(delta)
		return onDelta(delta)
	})
	s.logStream(userID, "diagnosis", cfg, messages, sb.String(), res.Usage, start, streamErr)
	return res.Truncated, streamErr
}
