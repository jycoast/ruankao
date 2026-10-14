package service

import (
	"encoding/json"
	"errors"
	"testing"

	"software-exam/internal/domain/repository"
)

// 统计口径重构后,service 只做「结果转换 + 窗口编排」,SQL 在 repo 层已测。
// 这组测试锁住 service 层的转换/编排/错误传播语义,不再接触任何 DB。
// stub 见 practice_submit_test.go 的 stubPracticeQueryRepo。

// GetAccuracy:正确率=Correct/Done*100,零除保护返回 0(done=0 不 panic)。
func TestGetAccuracy_CalculatesPercentage(t *testing.T) {
	pq := &stubPracticeQueryRepo{acc: repository.AccuracyRow{Done: 20, Correct: 15}}
	svc := newPracticeQuerySvc(pq)

	got := svc.GetAccuracy(7)
	if got.Done != 20 || got.Correct != 15 || got.Accuracy != 75 {
		t.Fatalf("应为 20/15/75,得到 %+v", got)
	}
}

func TestGetAccuracy_ZeroDivision(t *testing.T) {
	pq := &stubPracticeQueryRepo{acc: repository.AccuracyRow{}} // done=0
	svc := newPracticeQuerySvc(pq)

	got := svc.GetAccuracy(7)
	if got.Accuracy != 0 {
		t.Fatalf("done=0 时 Accuracy 应为 0,得到 %d", got.Accuracy)
	}
	if got.Done != 0 || got.Correct != 0 {
		t.Fatalf("done/correct 应为 0,得到 %+v", got)
	}
}

// GetAccuracy 对 repo 错误选择忽略错误取零值(与重构前行为一致:
// 原实现用 type assertion 直接 Scan 且不检查错)。保行为不回归。
func TestGetAccuracy_IgnoresRepoError(t *testing.T) {
	pq := &stubPracticeQueryRepo{accErr: errors.New("db down")}
	svc := newPracticeQuerySvc(pq)

	got := svc.GetAccuracy(7)
	if got.Done != 0 || got.Correct != 0 || got.Accuracy != 0 {
		t.Fatalf("repo 出错应返零值,得到 %+v", got)
	}
}

// GetCategoryProgress:repo 行拷入类型化 DTO,字段与前端契约逐字一致
// (knowledge_point/total/done/correct)。
func TestGetCategoryProgress_MapsRows(t *testing.T) {
	pq := &stubPracticeQueryRepo{progr: []repository.ProgressRow{
		{KnowledgePoint: "算法", Total: 50, Done: 20, Correct: 15},
		{KnowledgePoint: "网络", Total: 30, Done: 10, Correct: 8},
	}}
	svc := newPracticeQuerySvc(pq)

	got, err := svc.GetCategoryProgress(7)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if len(got) != 2 {
		t.Fatalf("期望 2 行,得到 %d", len(got))
	}
	row := got[0]
	if row.KnowledgePoint != "算法" || row.Total != 50 || row.Done != 20 || row.Correct != 15 {
		t.Fatalf("首行字段错: %+v", row)
	}
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	for _, k := range []string{"knowledge_point", "total", "done", "correct"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("JSON 缺 key %q: %s", k, b)
		}
	}
}

func TestGetCategoryProgress_PropagatesError(t *testing.T) {
	pq := &stubPracticeQueryRepo{progrErr: errors.New("db down")}
	svc := newPracticeQuerySvc(pq)

	if _, err := svc.GetCategoryProgress(7); err == nil {
		t.Fatalf("应传播 repo 错误,得到 nil")
	}
}

// GetReport:趋势/知识点/speed 三块按 repo 行拼进 map,正确率按累计口径算。
func TestGetReport_Assembles(t *testing.T) {
	pq := &stubPracticeQueryRepo{
		acc:   repository.AccuracyRow{Done: 10, Correct: 8},
		trend: []repository.TrendRow{{Day: "2026-08-30", Total: 5, Correct: 4}},
		kp:    []repository.KpRow{{KnowledgePoint: "算法", Done: 10, Correct: 8}},
		speed: repository.SpeedRow{Cnt: 10, AvgSec: 40},
	}
	svc := newPracticeQuerySvc(pq)

	report, err := svc.GetReport(7, 3)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if report.DistinctDone != 10 || report.DistinctCorrect != 8 {
		t.Fatalf("累计口径错: %+v", report)
	}
	if report.Accuracy != 80 {
		t.Fatalf("Accuracy 应为 80,得到 %d", report.Accuracy)
	}
	if len(report.Trend) != 1 || report.Trend[0].Date != "2026-08-30" ||
		report.Trend[0].Total != 5 || report.Trend[0].Correct != 4 {
		t.Fatalf("Trend 拼错: %+v", report.Trend)
	}
	if len(report.Knowledge) != 1 || report.Knowledge[0].KnowledgePoint != "算法" ||
		report.Knowledge[0].Done != 10 || report.Knowledge[0].Correct != 8 ||
		report.Knowledge[0].Accuracy != 80 {
		t.Fatalf("Knowledge 拼错: %+v", report.Knowledge)
	}
	if report.Speed == nil || report.Speed.AvgSeconds != 40 {
		t.Fatalf("Speed 拼错: %+v", report.Speed)
	}
}

// GetReport speed:三窗口都返同一样本(Cnt=10, Avg=40)时,本周/上周可比且 Faster=0。
// 窗口编排(0/0、7/0、14/7)在 repo 层已用 sqlmock 锁住,这里只验聚合。
func TestGetReport_SpeedComparison(t *testing.T) {
	pq := &stubPracticeQueryRepo{speed: repository.SpeedRow{Cnt: 10, AvgSec: 40}}
	svc := newPracticeQuerySvc(pq)

	report, err := svc.GetReport(7, 3)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if report.Speed == nil {
		t.Fatalf("Cnt>=5 应返 Speed,得到 nil")
	}
	if report.Speed.ThisWeekAvg != 40 || report.Speed.LastWeekAvg != 40 {
		t.Fatalf("周均 40 应为 40/40,得到 %+v", report.Speed)
	}
	if !report.Speed.HasComparison {
		t.Fatalf("两周都有样本应可比,得到 %+v", report.Speed)
	}
	if report.Speed.FasterPercent != 0 {
		t.Fatalf("同样本 Faster 应为 0,得到 %+v", report.Speed)
	}
}

// GetReport speed:样本不足(Cnt<5)应返 nil,不 panic。
func TestGetReport_SpeedInsufficientSamples(t *testing.T) {
	pq := &stubPracticeQueryRepo{speed: repository.SpeedRow{Cnt: 3, AvgSec: 40}}
	svc := newPracticeQuerySvc(pq)

	report, err := svc.GetReport(7, 3)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if report.Speed != nil {
		t.Fatalf("Cnt<5 应返 nil Speed,得到 %+v", report.Speed)
	}
}

// GetReport:趋势查询出错应整个失败(不留半填报告)。
func TestGetReport_PropagatesTrendError(t *testing.T) {
	pq := &stubPracticeQueryRepo{
		acc:      repository.AccuracyRow{Done: 10, Correct: 8},
		trendErr: errors.New("db down"),
	}
	svc := newPracticeQuerySvc(pq)

	if _, err := svc.GetReport(7, 3); err == nil {
		t.Fatalf("应传播趋势查询错误,得到 nil")
	}
}

// GetReport:知识点排行出错应整个失败。
func TestGetReport_PropagatesKnowledgeError(t *testing.T) {
	pq := &stubPracticeQueryRepo{
		acc:   repository.AccuracyRow{Done: 10, Correct: 8},
		trend: []repository.TrendRow{{Day: "2026-08-30", Total: 5, Correct: 4}},
		kpErr: errors.New("db down"),
	}
	svc := newPracticeQuerySvc(pq)

	if _, err := svc.GetReport(7, 3); err == nil {
		t.Fatalf("应传播知识点查询错误,得到 nil")
	}
}

// GetReport 的累积概览来自 subjectStatRepo;行不存在(用户未练过该科目)时,
// 累计字段保持 0,不 panic(stubSubjectStatRepo 默认 errNotFound)。
func TestGetReport_NoSubjectStat_KeepsZeros(t *testing.T) {
	pq := &stubPracticeQueryRepo{
		acc:   repository.AccuracyRow{Done: 10, Correct: 8},
		trend: []repository.TrendRow{{Day: "2026-08-30", Total: 5, Correct: 4}},
		kp:    []repository.KpRow{{KnowledgePoint: "算法", Done: 10, Correct: 8}},
		speed: repository.SpeedRow{Cnt: 10, AvgSec: 40},
	}
	// 单独换成带 pre 注入但 FindOne 返 not found 的 profile?这里直接用默认
	// stubSubjectStatRepo(FindOne 返 errNotFound)即可。
	svc := newPracticeQuerySvc(pq)

	report, err := svc.GetReport(7, 3)
	if err != nil {
		t.Fatalf("应无错,得到 err=%v", err)
	}
	if report.TotalPractice != 0 || report.TotalTime != 0 || report.StreakDays != 0 {
		t.Fatalf("无科目统计时累计应 0,得到 %+v", report)
	}
}