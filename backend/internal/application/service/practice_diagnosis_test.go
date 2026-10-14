package service

import (
	"errors"
	"testing"

	"software-exam/internal/domain/repository"
)

// 诊断汇总的 service 测试:分层阈值、map 合并、「未开始」补全、排序与 has_data。
// SQL 形态在 repo 层已测,这里只锁 service 的转换/编排语义。
// stub 见 practice_submit_test.go 的 stubPracticeQueryRepo。

func catNames(list []DiagnosisCategory) []string {
	names := make([]string, 0, len(list))
	for _, c := range list {
		names = append(names, c.Category)
	}
	return names
}

// 全层级覆盖 + 排序:0%/50%/70%/90% 四档 + 一个未开始大类。
func TestGetDiagnosisSummary_LevelsMergeAndOrder(t *testing.T) {
	pq := &stubPracticeQueryRepo{
		weak: []repository.WeaknessRow{
			{Category: "极弱", Done: 3, Correct: 0, Wrong: 3}, // 0% → level 3
			{Category: "弱", Done: 10, Correct: 5, Wrong: 5}, // 50% → level 2
			{Category: "中", Done: 10, Correct: 7, Wrong: 3}, // 70% → level 1
			{Category: "强", Done: 10, Correct: 9, Wrong: 1}, // 90% → level 0
		},
		counts: []repository.CategoryCountRow{
			{Category: "极弱", Total: 5},
			{Category: "弱", Total: 12},
			{Category: "中", Total: 15},
			{Category: "强", Total: 20},
			{Category: "未开始", Total: 40}, // 没做过 → level 4 排最后
		},
		stats: repository.WrongBookStats{Total: 13, Due: 5},
	}
	svc := newPracticeQuerySvc(pq)

	sum, err := svc.GetDiagnosisSummary(7, 1)
	if err != nil {
		t.Fatalf("应无错返回,得到 err=%v", err)
	}
	if sum.SubjectID != 1 {
		t.Fatalf("SubjectID 应为 1,得到 %d", sum.SubjectID)
	}
	if sum.WrongTotal != 13 || sum.DueCount != 5 {
		t.Fatalf("错题本统计错: total=%d due=%d", sum.WrongTotal, sum.DueCount)
	}
	if sum.Done != 33 || sum.Accuracy != 63 {
		t.Fatalf("整体口径错: done=%d accuracy=%d(应 21/33=63)", sum.Done, sum.Accuracy)
	}
	if sum.WeakCategoryN != 2 {
		t.Fatalf("薄弱大类应为 2(0%%/50%%),得到 %d", sum.WeakCategoryN)
	}
	if !sum.HasData {
		t.Fatal("有数据应 has_data=true")
	}
	if len(sum.Categories) != 5 {
		t.Fatalf("期望 5 个大类,得到 %d", len(sum.Categories))
	}
	// 排序:按薄弱度从高到低,未开始排最后
	wantOrder := []string{"极弱", "弱", "中", "强", "未开始"}
	for i, want := range wantOrder {
		if sum.Categories[i].Category != want {
			t.Fatalf("第 %d 个大类应为 %q,得到 %q(全集 %v)", i, want, sum.Categories[i].Category, catNames(sum.Categories))
		}
	}
	// 总题量补齐:已开始大类 total 应被题量填上;未开始大类是纯题量行
	for _, c := range sum.Categories {
		if c.Category == "未开始" {
			if c.Total != 40 || c.Level != diagLevelUntouched || c.Done != 0 {
				t.Fatalf("未开始大类字段错: %+v", c)
			}
		} else if c.Total == 0 {
			t.Fatalf("已开始大类 %q 的 total 应被题量补齐: %+v", c.Category, c)
		}
	}
}

// 60% 整是「中」不算薄弱:weak_category_n 的边界。
func TestGetDiagnosisSummary_WeakBoundaryAt60(t *testing.T) {
	pq := &stubPracticeQueryRepo{
		weak: []repository.WeaknessRow{
			{Category: "边界", Done: 10, Correct: 6, Wrong: 4}, // 60% → level 1
		},
		counts: []repository.CategoryCountRow{{Category: "边界", Total: 50}},
	}
	svc := newPracticeQuerySvc(pq)

	sum, err := svc.GetDiagnosisSummary(7, 1)
	if err != nil {
		t.Fatalf("应无错返回,得到 err=%v", err)
	}
	if sum.WeakCategoryN != 0 {
		t.Fatalf("60%% 不应算薄弱,得到 %d", sum.WeakCategoryN)
	}
	if sum.Categories[0].Level != diagLevelMid || sum.Categories[0].Accuracy != 60 {
		t.Fatalf("60%% 应为 level=1 accuracy=60,得到 %+v", sum.Categories[0])
	}
}

func TestGetDiagnosisSummary_NoData(t *testing.T) {
	svc := newPracticeQuerySvc(&stubPracticeQueryRepo{})

	sum, err := svc.GetDiagnosisSummary(7, 1)
	if err != nil {
		t.Fatalf("应无错返回,得到 err=%v", err)
	}
	if sum.HasData {
		t.Fatal("空数据应 has_data=false")
	}
	if sum.Done != 0 || sum.Accuracy != 0 || sum.WeakCategoryN != 0 || len(sum.Categories) != 0 {
		t.Fatalf("空数据各字段应为零: %+v", sum)
	}
}

// 错题本有存量但客观题没做过:has_data=true(页面引导已够),画像保持空。
func TestGetDiagnosisSummary_WrongBookOnly(t *testing.T) {
	pq := &stubPracticeQueryRepo{stats: repository.WrongBookStats{Total: 3, Due: 1}}
	svc := newPracticeQuerySvc(pq)

	sum, err := svc.GetDiagnosisSummary(7, 1)
	if err != nil {
		t.Fatalf("应无错返回,得到 err=%v", err)
	}
	if !sum.HasData {
		t.Fatal("仅有错题本也应有 has_data=true")
	}
	if sum.Done != 0 || len(sum.Categories) != 0 {
		t.Fatalf("无客观题画像应为空: done=%d cats=%d", sum.Done, len(sum.Categories))
	}
}

func TestGetDiagnosisSummary_PropagatesWeaknessError(t *testing.T) {
	pq := &stubPracticeQueryRepo{weakErr: errors.New("db down")}
	svc := newPracticeQuerySvc(pq)

	if _, err := svc.GetDiagnosisSummary(7, 1); err == nil {
		t.Fatal("应传播 repo 错误,得到 nil")
	}
}
