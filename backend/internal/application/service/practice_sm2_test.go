package service

import (
	"testing"

	"software-exam/internal/domain/model"
)

// 原实现 int(float32(cur)*ef) 直接截断，EF 衰减到 1.3 后 1*1.3=1.3→1，
// 间隔永远停在 1 天，错题天天出现。这是付费功能，必须验证间隔真的会增长。
func TestIntervalGrowsAndNeverStalls(t *testing.T) {
	wq := &model.WrongQuestion{IntervalDays: 1, EaseFactor: minEaseFactor, Status: 1}

	prev := wq.IntervalDays
	for i := 0; i < 10; i++ {
		wq.CorrectCount = 0 // 避免提前毕业，只观察间隔演进
		applyReviewResult(wq, true)
		if wq.IntervalDays <= prev && prev < maxIntervalDays {
			t.Fatalf("第 %d 次复习间隔未增长：%d → %d（EF=%.2f）", i+1, prev, wq.IntervalDays, wq.EaseFactor)
		}
		prev = wq.IntervalDays
	}
	if prev != maxIntervalDays {
		t.Fatalf("间隔应最终达到上限 %d，实际 %d", maxIntervalDays, prev)
	}
}

func TestIntervalCappedAtMax(t *testing.T) {
	if got := nextInterval(maxIntervalDays, 2.5); got != maxIntervalDays {
		t.Fatalf("间隔不应超过上限，得到 %d", got)
	}
}

// 答对时 EF 必须回升，否则用户把题做熟了系统也感知不到，永远按最难的节奏推送。
func TestEaseFactorRecoversOnCorrect(t *testing.T) {
	ef := float32(minEaseFactor)
	for i := 0; i < 5; i++ {
		ef = adjustEase(ef, true)
	}
	if ef <= minEaseFactor {
		t.Fatalf("连续答对后 EF 应回升，仍为 %.2f", ef)
	}
	if ef > maxEaseFactor {
		t.Fatalf("EF 不应超过上限 %.2f，实际 %.2f", maxEaseFactor, ef)
	}
}

func TestEaseFactorFloor(t *testing.T) {
	ef := float32(defaultEaseFactor)
	for i := 0; i < 20; i++ {
		ef = adjustEase(ef, false)
	}
	if ef != minEaseFactor {
		t.Fatalf("EF 应止步于下限 %.2f，实际 %.2f", float32(minEaseFactor), ef)
	}
}

func TestWrongAnswerResetsSchedule(t *testing.T) {
	wq := &model.WrongQuestion{IntervalDays: 20, EaseFactor: 2.5, CorrectCount: 2, Status: 1}
	applyReviewResult(wq, false)

	if wq.IntervalDays != 1 {
		t.Fatalf("答错应重置为次日复习，实际 %d 天", wq.IntervalDays)
	}
	if wq.CorrectCount != 0 {
		t.Fatalf("答错应清零连对计数，实际 %d", wq.CorrectCount)
	}
	if wq.Status != 1 {
		t.Fatalf("答错应回到待复习状态，实际 %d", wq.Status)
	}
}

func TestGraduatesAfterConsecutiveCorrect(t *testing.T) {
	wq := &model.WrongQuestion{IntervalDays: 1, EaseFactor: defaultEaseFactor, Status: 1}
	for i := 0; i < graduateCorrectCount; i++ {
		applyReviewResult(wq, true)
	}
	if wq.Status != 2 {
		t.Fatalf("连对 %d 次应标记为已掌握，实际 status=%d", graduateCorrectCount, wq.Status)
	}
}
