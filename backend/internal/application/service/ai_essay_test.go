package service

import (
	"strings"
	"testing"
)

// 页面只提供主题与题干，其余字段必须带【待补充】标记 ——
// 若漏标，模型会把「【填写】」这类模板占位符当成真实项目事实写进论文。
func TestEssayInputMarksMissingFields(t *testing.T) {
	got := essayInput("论云原生架构及其应用", "")

	if !strings.Contains(got, "论文题目：论云原生架构及其应用") {
		t.Errorf("主题未填入：\n%s", got)
	}
	if !strings.Contains(got, "完整题干及子问题：【待补充】") {
		t.Errorf("空题干应标记【待补充】：\n%s", got)
	}
	for _, field := range []string{
		"考试类别", "项目背景与业务流程", "本人职责",
		"技术架构与技术栈", "主要问题及实际解决方案", "项目规模、约束与效果数据",
	} {
		if !strings.Contains(got, field+"：【待补充】") {
			t.Errorf("%s 未标记【待补充】：\n%s", field, got)
		}
	}
	// 未提供资料时要明确可写模拟范文，否则整篇都是【待补充】占位符
	if !strings.Contains(got, "模拟设定") {
		t.Errorf("缺少模拟设定默认要求：\n%s", got)
	}
}

func TestEssayInputTrimsPoints(t *testing.T) {
	got := essayInput("题目", "  \n问题1：……\n问题2：……\n  ")
	if !strings.Contains(got, "完整题干及子问题：问题1：……") {
		t.Errorf("题干未去掉首尾空白：\n%q", got)
	}
	if strings.Contains(got, "完整题干及子问题：【待补充】") {
		t.Errorf("给了题干却仍标【待补充】：\n%s", got)
	}
}
