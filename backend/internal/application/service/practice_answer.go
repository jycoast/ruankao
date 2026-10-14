package service

import (
	"sort"
	"strings"

	"software-exam/internal/domain/model"
)

// blankSize 软考上午题多空题通常每空一组 A/B/C/D。
const blankSize = 4

func sameLabels(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// orderOptionsForBlanks 为多空题恢复「按空位」的选项顺序。
//
// 优先保留仓库返回的原始顺序（通常即导入顺序 / id 递增）。
// 仅当 sort_order 在整题范围内唯一时，才按 sort_order 排序；
// 因为多空题导入常把每组都标成 0..3，此时按 sort_order 会把各空的选项交错在一起。
func orderOptionsForBlanks(opts []model.Option) []model.Option {
	if len(opts) <= 1 {
		return opts
	}
	ordered := make([]model.Option, len(opts))
	copy(ordered, opts)

	seen := map[int]struct{}{}
	unique := true
	for _, o := range opts {
		if _, ok := seen[o.SortOrder]; ok {
			unique = false
			break
		}
		seen[o.SortOrder] = struct{}{}
	}
	if unique {
		sort.SliceStable(ordered, func(i, j int) bool {
			if ordered[i].SortOrder != ordered[j].SortOrder {
				return ordered[i].SortOrder < ordered[j].SortOrder
			}
			return ordered[i].ID < ordered[j].ID
		})
	}
	return ordered
}

// buildCorrectAnswer 按空位顺序拼出标准答案。
//
// 多空题导入脏数据常见形态：
//  1. 每组 sort_order 都是 0..3（重复），不能按 sort_order 全局排序；
//  2. 整题正确答案若是 CD，则每一组 A/B/C/D 都被标成 C、D 正确。
//     这时不能「每组取第一个正确项」得到 CC，而应识别为跨空位的 CD。
func buildCorrectAnswer(opts []model.Option) string {
	if len(opts) == 0 {
		return ""
	}
	ordered := orderOptionsForBlanks(opts)

	type groupCorrects []string
	var groups []groupCorrects
	for i := 0; i < len(ordered); i += blankSize {
		end := i + blankSize
		if end > len(ordered) {
			end = len(ordered)
		}
		var corrects []string
		for _, o := range ordered[i:end] {
			if o.IsCorrect == 1 {
				corrects = append(corrects, o.Label)
			}
		}
		groups = append(groups, corrects)
	}
	if len(groups) == 0 {
		return ""
	}

	// 脏数据：每一组都标了相同的多个正确项，且数量正好等于空位数。
	// 例如两组都是 [C,D] → 答案 CD。
	if len(groups) > 1 && len(groups[0]) == len(groups) && len(groups[0]) > 1 {
		same := true
		for _, g := range groups[1:] {
			if !sameLabels(g, groups[0]) {
				same = false
				break
			}
		}
		if same {
			return strings.Join(groups[0], "")
		}
	}

	// 常规：每空取该组第一个正确项。
	var b strings.Builder
	for _, g := range groups {
		if len(g) == 0 {
			b.WriteString("?")
			continue
		}
		b.WriteString(g[0])
	}
	return b.String()
}
