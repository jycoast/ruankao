// Package repository 定义各业务域的 repository 接口与实现。
//
// 说明:本文件是历史巨人文件的"空壳"。原 repository.go 曾把 39 个 repo 的
// 接口 + 构造器 + 实现全部塞在单文件(2751 行),与 main.go/practice.go 的
// 拆分改造同源。现按域拆成独立文件,每个文件含一或多个相邻域的 repo:
//
//	user.go / question.go / wrong_question.go / favorite.go / practice.go
//	knowledge.go / article.go / paper.go / exam.go / user_stat.go / subject.go
//	announcement.go / notification.go / report.go / auth.go / essay.go
//	invite_code.go / card_key.go / daily.go / question_note.go / ai.go
//	membership.go / order.go / comment.go / exam_schedule.go
//	dashboard.go(先前) / practice_query.go(先前)
//
// 各文件 import 头按实际引用自动生成;断言跨包使用不受影响(全在 package
// repository 内,编译期可见)。
package repository
