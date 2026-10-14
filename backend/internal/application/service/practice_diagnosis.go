package service

import (
	"sort"
)

// 诊断报告（会员专属页 /my/diagnosis）的 service 层。
//
// 折叠口径：数据全部来自 PracticeQueryRepository 的四个聚合查询，此处只做
// map 合并（做题画像 + 题量补全）、薄弱度分层与排序。「薄弱」的判据与前端
// accColor 保持一致（客观题最近一次正确率 <60%）。

// 薄弱度分层档位。0-3 与前端 accColor 四档同阈（>=80 强 / >=60 中 / >=40 弱 /
// <40 极弱），4 表示「该大类从未做过客观题」。
const (
	diagLevelStrong    = int8(0)
	diagLevelMid       = int8(1)
	diagLevelWeak      = int8(2)
	diagLevelCritical  = int8(3)
	diagLevelUntouched = int8(4)
)

// diagnosisLevel 薄弱度分层。done=0 视为未开始；其余按正确率分档。
func diagnosisLevel(acc int, done int64) int8 {
	if done == 0 {
		return diagLevelUntouched
	}
	switch {
	case acc >= 80:
		return diagLevelStrong
	case acc >= 60:
		return diagLevelMid
	case acc >= 40:
		return diagLevelWeak
	default:
		return diagLevelCritical
	}
}

// DiagnosisCategory 薄弱点画像里一个大类的统计。
type DiagnosisCategory struct {
	Category string `json:"category"`
	Total    int64  `json:"total"`    // 该大类客观题总量（含未做过的）
	Done     int64  `json:"done"`     // 做过（每题最近一次）
	Correct  int64  `json:"correct"`  // 其中答对的
	Wrong    int64  `json:"wrong"`    // 其中答错的
	Accuracy int    `json:"accuracy"` // 正确率 %
	Level    int8   `json:"level"`    // 0强 1中 2弱 3极弱 4未开始
}

// DiagnosisSummary 诊断报告聚合结果。
type DiagnosisSummary struct {
	SubjectID     uint64              `json:"subject_id"`
	WrongTotal    int                 `json:"wrong_total"`     // 错题本存量总数
	DueCount      int                 `json:"due_count"`       // 错题本待复习数
	WeakCategoryN int                 `json:"weak_category_n"` // 薄弱大类的个数（做了且正确率<60%）
	Done          int                 `json:"done"`            // 客观题做过总数
	Accuracy      int                 `json:"accuracy"`        // 客观题整体正确率 %
	HasData       bool                `json:"has_data"`        // 是否有可诊断的数据
	Categories    []DiagnosisCategory `json:"categories"`      // 各大类画像，未开始排最后
}

// GetDiagnosisSummary 生成某科目的诊断数据。
// 三个聚合查询里任一个失败都直接透传错误（本地库查询，失败即异常）。
func (s *PracticeService) GetDiagnosisSummary(userID, subjectID uint64) (*DiagnosisSummary, error) {
	accRows, err := s.practiceQueryRepo.CategoryAccuracyBySubject(userID, subjectID)
	if err != nil {
		return nil, err
	}
	countRows, err := s.practiceQueryRepo.CategoryQuestionCountsBySubject(subjectID)
	if err != nil {
		return nil, err
	}
	stats, err := s.practiceQueryRepo.WrongBookStatsBySubject(userID, subjectID)
	if err != nil {
		return nil, err
	}

	sum := &DiagnosisSummary{
		SubjectID:  subjectID,
		WrongTotal: int(stats.Total),
		DueCount:   int(stats.Due),
	}

	// 先并「做过」的大类画像，再补「没做过」的大类（未开始，Level=4）。
	byCat := make(map[string]*DiagnosisCategory, len(accRows)+len(countRows))
	totalDone, totalCorrect := 0, 0
	for _, r := range accRows {
		acc := 0
		if r.Done > 0 {
			acc = int(float64(r.Correct) / float64(r.Done) * 100.0)
		}
		c := &DiagnosisCategory{
			Category: r.Category, Done: r.Done, Correct: r.Correct, Wrong: r.Wrong,
			Accuracy: acc, Level: diagnosisLevel(acc, r.Done),
		}
		byCat[c.Category] = c
		totalDone += int(r.Done)
		totalCorrect += int(r.Correct)
	}
	for _, r := range countRows {
		if c, ok := byCat[r.Category]; ok {
			c.Total = r.Total
			continue
		}
		byCat[r.Category] = &DiagnosisCategory{Category: r.Category, Total: r.Total, Level: diagLevelUntouched}
	}

	sum.Done = totalDone
	if totalDone > 0 {
		sum.Accuracy = int(float64(totalCorrect) / float64(totalDone) * 100.0)
	}
	sum.HasData = totalDone > 0 || sum.WrongTotal > 0

	list := make([]DiagnosisCategory, 0, len(byCat))
	for _, c := range byCat {
		if c.Done > 0 && c.Accuracy < 60 {
			sum.WeakCategoryN++
		}
		list = append(list, *c)
	}
	// 未开始排最后；已开始的按薄弱度从高到低（先弱后强），平手按错题数/题量。
	sort.Slice(list, func(i, j int) bool {
		a, b := list[i], list[j]
		ai, bi := a.Level == diagLevelUntouched, b.Level == diagLevelUntouched
		if ai != bi {
			return !ai
		}
		if a.Level != b.Level {
			return a.Level > b.Level
		}
		if a.Wrong != b.Wrong {
			return a.Wrong > b.Wrong
		}
		return a.Done > b.Done
	})
	sum.Categories = list
	return sum, nil
}
