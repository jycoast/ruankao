package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// newRepoWithSQLMock 构造 KnowledgePointRepository 接口实现,后端连到 sqlmock 而非真 MySQL。
// gorm 用纯驱动 dialect(mysql)是为了让 Reorder 里的 clause.Locking / Transaction 走的
// 是 mysql 分支——和我们生产一致;sqlmock 不解析 SQL,但会校验事务 BEGIN/COMMIT 期望。
//
// 使用 sqlmock 默认的 QueryMatcherRegexp(期望字符串作正则匹配),而不是
// QueryMatcherEqual(GORM 生成的列名/反引号/WHERE 子句会让字面匹配极脆)。
func newRepoWithSQLMock(t *testing.T) (KnowledgePointRepository, sqlmock.Sqlmock) {
	t.Helper()
	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      conn,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	return NewKnowledgePointRepository(gormDB), mock
}

// 空 items 是最常见的"无操作"提交,Reorder 必须立即返回,不能进事务。
// 这是事务外早返回的第一个分支——TDD 第 1 个测试,验证基础设施可用。
func TestReorder_EmptyItems_ReturnsEmptyVersion(t *testing.T) {
	repo, mock := newRepoWithSQLMock(t)
	// 不调用 ExpectBegin:任何 BEGIN/UPDATE 都会让 ExpectationsWereMet 通过。
	// 反向断言——若生产代码误进事务,sqlmock 会因收到未期望的 SQL 而失败。

	version, err := repo.Reorder(nil, "")
	if err != nil {
		t.Fatalf("空 items 应无错返回,得到 err=%v", err)
	}
	if version != "" {
		t.Fatalf("空 items 应返回空版本号,得到 %q", version)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("空 items 不应触发任何 SQL,实际有: %v", err)
	}
}

// 拖拽树里出现"父级不在本批提交"的节点(典型场景:跨科目挂载、悬空父节点、
// 前端快照被分页截掉了父级)。整体拒绝,任何 UPDATE 都不能落库——否则会把子节点
// 永久顶到错误科目或变成孤儿。
func TestReorder_RejectsParentNotInBatch(t *testing.T) {
	repo, mock := newRepoWithSQLMock(t)
	// items: id=10 想挂到 id=999,但 id=999 不在本批
	items := []ReorderItem{
		{ID: 10, ParentID: 999, SortOrder: 0},
	}

	version, err := repo.Reorder(items, "")
	if !errors.Is(err, ErrReorderBadParent) {
		t.Fatalf("父级不在批内应返回 ErrReorderBadParent,得到 err=%v", err)
	}
	if version != "" {
		t.Fatalf("拒绝路径不应返回版本号,得到 %q", version)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("父级校验失败不应触发任何 SQL,实际有: %v", err)
	}
}

// 父子循环引用:整批里构造一条 A→B→A 的环。buildTree 对环的处理是让环内节点
// 全部从树里消失(无根可达),显示不出也管理不到——必须在前置校验阶段就拒绝。
func TestReorder_RejectsCycle(t *testing.T) {
	repo, mock := newRepoWithSQLMock(t)
	// id=1 父级=2,id=2 父级=1:从任一节点出发都会回到自身
	items := []ReorderItem{
		{ID: 1, ParentID: 2, SortOrder: 0},
		{ID: 2, ParentID: 1, SortOrder: 0},
	}

	_, err := repo.Reorder(items, "")
	if !errors.Is(err, ErrReorderCycle) {
		t.Fatalf("环引用应返回 ErrReorderCycle,得到 err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("环检测失败不应触发任何 SQL,实际有: %v", err)
	}
}

// baseVersion 必须是 RFC3339 字符串(前端从 MAX(updated_at) 直接取)。
// 收到非法字符串(如数字、ISO 无时区、日期无时间)应立刻返回 time.ParseError,
// 不能进事务——否则会把"数据脏"误判成"并发冲突"。
func TestReorder_RejectsMalformedBaseVersion(t *testing.T) {
	repo, mock := newRepoWithSQLMock(t)
	// 单个合法父级(顶级)绕过父级/环校验,让 baseVersion 解析成为下一个分支
	items := []ReorderItem{
		{ID: 1, ParentID: 0, SortOrder: 0},
	}

	_, err := repo.Reorder(items, "not-a-timestamp")
	var parseErr *time.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("非法 baseVersion 应返回 *time.ParseError,得到 err=%v (%T)", err, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("baseVersion 解析失败不应触发任何 SQL,实际有: %v", err)
	}
}

// Happy path:同一父级下改 sort_order(parent 没动),多个 item 验证差量更新
// 只 UPDATE 真正变化的行——item 2 没变化,不应触发 UPDATE。
// item 1 parent 指向 item 2,触发"父级必须 0 或在本批"的前置校验通过。
func TestReorder_HappyPath_UpdatesOnlyChangedRows(t *testing.T) {
	repo, mock := newRepoWithSQLMock(t)
	now := time.Now()
	parent := uint64(2)
	newVersionTime := now.Add(time.Second)
	newVersion := newVersionTime.Format(time.RFC3339)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id", "sort_order", "updated_at"}).
			AddRow(uint64(1), parent, 0, now).    // item 1: 改 sort 0→5,parent 不变
			AddRow(uint64(2), uint64(0), 0, now)) // item 2: 什么都没改
	mock.ExpectExec("UPDATE").
		WithArgs(parent, 5, sqlmock.AnyArg(), uint64(1)). // SET parent_id, sort_order, updated_at, WHERE id=1
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT MAX").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(newVersionTime))

	items := []ReorderItem{
		{ID: 1, ParentID: parent, SortOrder: 5},
		{ID: 2, ParentID: 0, SortOrder: 0}, // 顶级,不参与 UPDATE
	}
	version, err := repo.Reorder(items, "")
	if err != nil {
		t.Fatalf("happy path 应无错返回,得到 err=%v", err)
	}
	if version != newVersion {
		t.Fatalf("应返回新版本号 %q,得到 %q", newVersion, version)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// 并发覆写防护:提交方带着 baseVersion=09-01 进入事务,但库里某行的
// updated_at 已经晚于 baseVersion(其他管理员改过)。整树拒绝并回滚,
// 防止旧快照把别人刚整理好的层级悄悄改回去。
func TestReorder_RejectsStaleBaseVersion(t *testing.T) {
	repo, mock := newRepoWithSQLMock(t)
	baseVersion, _ := time.Parse(time.RFC3339, "2026-09-01T00:00:00Z")
	newerUpdate, _ := time.Parse(time.RFC3339, "2026-09-05T00:00:00Z") // 比 baseVersion 新

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id", "sort_order", "updated_at"}).
			AddRow(uint64(1), uint64(2), 0, newerUpdate). // 库里的 updated_at 比 baseVersion 新
			AddRow(uint64(2), uint64(0), 0, newerUpdate)) // item 2 在批内(parent=2 指向本批)
	mock.ExpectRollback() // callback 返回 ErrReorderConflict → GORM 回滚

	items := []ReorderItem{
		{ID: 1, ParentID: 2, SortOrder: 5},
		{ID: 2, ParentID: 0, SortOrder: 0}, // 顶级,绕过"父级不在批"前置校验
	}
	_, err := repo.Reorder(items, baseVersion.Format(time.RFC3339))
	if !errors.Is(err, ErrReorderConflict) {
		t.Fatalf("过期 baseVersion 应返回 ErrReorderConflict,得到 err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}

// 截断快照防线:item 1 想改挂到顶级(parent=0),但它当前库里的父级(999)不在
// 本批——典型场景:前端列表被分页截断,父节点没拉进来。整体拒绝并回滚,
// 防止把子节点永久顶成顶级节点(层级破坏永久化)。
func TestReorder_RejectsTruncatedSnapshot(t *testing.T) {
	repo, mock := newRepoWithSQLMock(t)
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id", "sort_order", "updated_at"}).
			AddRow(uint64(1), uint64(999), 0, now). // 库里父级=999,本批不含 999
			AddRow(uint64(2), uint64(0), 0, now))   // item 2 在批内让前置校验通过
	mock.ExpectRollback() // callback 返回 ErrReorderIncomplete → GORM 回滚

	items := []ReorderItem{
		{ID: 1, ParentID: 0, SortOrder: 5}, // 想改到顶级
		{ID: 2, ParentID: 0, SortOrder: 0}, // 顶级,前置校验需要它在批里
	}
	_, err := repo.Reorder(items, "")
	if !errors.Is(err, ErrReorderIncomplete) {
		t.Fatalf("截断快照应返回 ErrReorderIncomplete,得到 err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sqlmock 期望未满足: %v", err)
	}
}
