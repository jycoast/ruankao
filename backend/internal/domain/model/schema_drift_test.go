package model

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm/schema"
)

// baselineDDL 是仓库里唯一的建表脚本：000/001 是把历史 90+ 个迁移压平后的 baseline，
// 表结构以它为准。
const baselineDDL = "../../../migrations/000_baseline_schema.sql"

var (
	createTableRe = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + "`?" + `(\w+)` + "`?" + `\s*\(`)
	// 表级约束不是列定义，解析时跳过。
	tableConstraintRe = regexp.MustCompile(`(?i)^(PRIMARY\s+KEY|UNIQUE\s+(KEY|INDEX)|KEY|INDEX|CONSTRAINT|FOREIGN\s+KEY|CHECK|FULLTEXT|SPATIAL)\b`)
)

var timeType = reflect.TypeOf(time.Time{})

// TestModelsMatchBaselineDDL 锁住「模型映射的列名 == 建表脚本里的列名」。
//
// 这层校验为什么必须存在：GORM 的 `gorm:"column:x"` 标签一旦与实际列名不符，
// 读的时候 SELECT * 拿不到值会静默置零（不报错），写的时候 INSERT/UPDATE 直接
// 1054 Unknown column 报错。而 service 层单测全部用内存 stub 绕开 SQL，这类错配
// 在单测里完全不可见。
//
// 真实事故：UserSubjectStat.LastPracticeAt 的标签写成了 user_profiles 表的列名
// last_practice_date，而 user_subject_stats 的列是 last_practice_at，导致该表
// 写入 100% 失败、streak_days 对所有新用户恒为 0。
func TestModelsMatchBaselineDDL(t *testing.T) {
	raw, err := os.ReadFile(baselineDDL)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", baselineDDL, err)
	}
	tables := parseDDLTables(string(raw))
	if len(tables) == 0 {
		t.Fatal("没从 baseline 里解析出任何表，解析逻辑可能已失效")
	}

	models := []any{
		User{}, UserProfile{}, Subject{}, Question{}, Option{}, Paper{}, PaperQuestion{},
		KnowledgePoint{}, Article{}, WrongQuestion{}, Favorite{}, PracticeRecord{},
		PracticeSession{}, PaperPracticeSession{}, ExamRecord{}, Announcement{},
		ExamSchedule{}, AnnouncementRead{}, Notification{}, QuestionReport{}, Feedback{},
		PasswordReset{}, EssaySubmission{}, InviteCode{}, CardKey{}, DailyCheckin{},
		DailySession{}, QuestionNote{}, UserSubjectStat{}, AIConfig{}, SiteSettings{},
		AITrialUsage{}, UserAIConfig{}, AIConversation{}, AIMessage{}, AICallLog{},
		Membership{}, MembershipPlan{}, Order{},
	}

	var problems []string
	for _, m := range models {
		s, err := schema.Parse(m, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			problems = append(problems, fmt.Sprintf("%T 解析失败: %v", m, err))
			continue
		}
		cols, ok := tables[strings.ToLower(s.Table)]
		if !ok {
			problems = append(problems, fmt.Sprintf("表 %s: baseline 里没有这张表", s.Table))
			continue
		}
		for _, f := range s.Fields {
			// 关联字段（切片/结构体）不是列；被 gorm:"-" 忽略的字段 DBName 为空。
			if f.DBName == "" || !isColumnType(f.FieldType) {
				continue
			}
			if !cols[strings.ToLower(f.DBName)] {
				problems = append(problems, fmt.Sprintf("%s.%s 映射到列 %q，但该表没有这一列",
					s.Table, f.Name, f.DBName))
			}
		}
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		t.Fatalf("模型与建表脚本不一致（%d 处）:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
}

// parseDDLTables 从建表脚本里解析出 表名 → 列名集合。
func parseDDLTables(sql string) map[string]map[string]bool {
	tables := map[string]map[string]bool{}
	for _, loc := range createTableRe.FindAllStringSubmatchIndex(sql, -1) {
		name := strings.ToLower(sql[loc[2]:loc[3]])
		// loc[1] 是整段匹配的结束位置，正好落在左括号之后。
		body, ok := parenBody(sql[loc[1]:])
		if !ok {
			continue
		}
		cols := map[string]bool{}
		for _, raw := range strings.Split(body, "\n") {
			line := strings.TrimSpace(raw)
			if line == "" || strings.HasPrefix(line, "--") || tableConstraintRe.MatchString(line) {
				continue
			}
			cols[firstToken(line)] = true
		}
		tables[name] = cols
	}
	return tables
}

// parenBody 返回 s 中与起始左括号配对的右括号之前的内容（调用时 s 已跳过该左括号）。
func parenBody(s string) (string, bool) {
	depth := 1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[:i], true
			}
		}
	}
	return "", false
}

// firstToken 取一行 DDL 的首个标识符（列名），去掉反引号。
func firstToken(line string) string {
	i := strings.IndexAny(line, " \t(")
	if i < 0 {
		i = len(line)
	}
	return strings.Trim(line[:i], "`")
}

// isColumnType 判断字段是否是「落成数据库列」的基础类型。
// time.Time 本身是结构体、*time.Time 是指针，但它们确实是列，要单独放行；
// 其余结构体/切片一律当作 GORM 关联字段排除，避免误报。
func isColumnType(t reflect.Type) bool {
	if t == timeType {
		return true
	}
	if t.Kind() == reflect.Ptr {
		return t.Elem() == timeType
	}
	switch t.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.String:
		return true
	}
	return false
}
