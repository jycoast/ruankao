package service

import (
	"encoding/json"
	"errors"
	"strconv"

	"software-exam/internal/domain/model"
)

// 分类刷题的「从头开始 / 继续刷题」。
//
// 设计取舍：「从头开始」只丢弃这份草稿，不删 practice_records。
// 那张表是学习报告的趋势、正确率、做题速度的唯一数据源，
// 删掉等于用户点一下「重新开始」就把历史统计抹了 —— 代价远大于收益。
// 用户想要的「清空上次答题记录」指的是这次刷题的答题卡，语义上正是本草稿。

// allSessionQuestions 抽题数量用的「不设上限」标记。
// 分类列表显示的是该知识点全量题数（GetCategories 的 COUNT），
// 答题卡必须与之对齐 —— 旧版硬编码 20 道上限，37 题的知识点只发 20 题，
// 第 20 题就显示「完成」，剩下 17 道永远不会出现（用户反馈的 bug）。
const allSessionQuestions = -1 // GetRandomQuestions 的 count<=0 表示不设 LIMIT

// PracticeSessionData 一份刷题草稿的对外形态。
type PracticeSessionData struct {
	Questions []model.Question  `json:"questions"`         // 按上次的顺序
	Answers   map[string]string `json:"answers"`           // {question_id: 用户答案}
	Index     int               `json:"index"`             // 建议从第几题继续（0 基）
	Resumed   bool              `json:"resumed"`           // false 表示这是新开的一轮
}

// StartPractice 开一轮刷题。
// fresh=true 丢弃旧草稿重新抽题；fresh=false 且有旧草稿时原样恢复上次的题目与作答。
func (s *PracticeService) StartPractice(userID, subjectID uint64, qType int8, knowledgePoint string, fresh bool) (*PracticeSessionData, error) {
	if !fresh {
		if data, err := s.resumeSession(userID, subjectID, qType, knowledgePoint); err == nil && data != nil {
			return data, nil
		}
		// 草稿缺失或题目已被下架，回落到新开一轮
	}

	questions, err := s.questionRepo.GetRandomQuestions(qType, allSessionQuestions, knowledgePoint, subjectID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(questions))
	for _, q := range questions {
		ids = append(ids, q.ID)
	}
	if len(ids) > 0 {
		s.saveSession(userID, subjectID, qType, knowledgePoint, ids, map[string]string{})
	}
	return &PracticeSessionData{
		Questions: questions,
		Answers:   map[string]string{},
	}, nil
}

// resumeSession 还原草稿。题目按存下来的 id 顺序重排 —— FindByIDs 走 IN 查询，
// 返回顺序由数据库决定，直接用会让「继续刷题」的题序和上次对不上。
func (s *PracticeService) resumeSession(userID, subjectID uint64, qType int8, knowledgePoint string) (*PracticeSessionData, error) {
	sess, err := s.sessionRepo.FindOne(userID, subjectID, qType, knowledgePoint)
	if err != nil {
		return nil, err
	}
	var ids []uint64
	if err := json.Unmarshal([]byte(sess.QuestionIDs), &ids); err != nil || len(ids) == 0 {
		return nil, errors.New("草稿已损坏")
	}
	// 草稿必须覆盖当前候选集全量题数：旧版 20 道上限存下的残缺草稿（37 题只存 20）
	// 若原样恢复，答题卡还是残缺的，用户做完 20 就「完成」、剩下 17 道永远不出现。
	// 题数对不上就返回 nil（StartPractice 的 !fresh 分支会回落到重抽全量），
	// 而不是把残缺草稿端给用户。过滤条件与 GetRandomQuestions 一致，不能只数 status=1。
	full, err := s.questionRepo.CountQuestions(qType, knowledgePoint, subjectID)
	if err != nil {
		return nil, err
	}
	if int64(len(ids)) != full {
		return nil, nil
	}
	answers := map[string]string{}
	json.Unmarshal([]byte(sess.Answers), &answers)

	found, err := s.questionRepo.FindByIDs(ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint64]model.Question, len(found))
	for _, q := range found {
		byID[q.ID] = q
	}
	questions := make([]model.Question, 0, len(ids))
	for _, id := range ids {
		if q, ok := byID[id]; ok {
			questions = append(questions, q)
		}
	}
	if len(questions) == 0 {
		return nil, errors.New("草稿中的题目已全部下架")
	}

	// 从第一道没答过的题接着做；全答完则停在最后一题
	index := len(questions) - 1
	for i, q := range questions {
		if _, done := answers[uint64ToStr(q.ID)]; !done {
			index = i
			break
		}
	}
	return &PracticeSessionData{
		Questions: questions,
		Answers:   answers,
		Index:     index,
		Resumed:   true,
	}, nil
}

// SaveSessionAnswer 记一道题的作答到草稿里。答错也照记：
// 「继续刷题」要还原的是上次的答题卡原貌，不是只留对的。
func (s *PracticeService) SaveSessionAnswer(userID, subjectID uint64, qType int8, knowledgePoint string, questionID uint64, answer string) error {
	sess, err := s.sessionRepo.FindOne(userID, subjectID, qType, knowledgePoint)
	if err != nil {
		return nil // 没有草稿（比如错题复习场景）就不记，不算错
	}
	var ids []uint64
	if err := json.Unmarshal([]byte(sess.QuestionIDs), &ids); err != nil {
		return err
	}
	answers := map[string]string{}
	json.Unmarshal([]byte(sess.Answers), &answers)
	answers[uint64ToStr(questionID)] = answer
	return s.saveSession(userID, subjectID, qType, knowledgePoint, ids, answers)
}

func (s *PracticeService) saveSession(userID, subjectID uint64, qType int8, knowledgePoint string, ids []uint64, answers map[string]string) error {
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	answersJSON, err := json.Marshal(answers)
	if err != nil {
		return err
	}
	return s.sessionRepo.Save(&model.PracticeSession{
		UserID:         userID,
		SubjectID:      subjectID,
		QuestionType:   qType,
		KnowledgePoint: knowledgePoint,
		QuestionIDs:    string(idsJSON),
		Answers:        string(answersJSON),
	})
}

// HasSession 供列表页判断某知识点能不能「继续刷题」。
func (s *PracticeService) HasSession(userID, subjectID uint64, qType int8, knowledgePoint string) bool {
	_, err := s.sessionRepo.FindOne(userID, subjectID, qType, knowledgePoint)
	return err == nil
}

func uint64ToStr(v uint64) string {
	return strconv.FormatUint(v, 10)
}
