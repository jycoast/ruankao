package service

import (
	"strings"
	"testing"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type fakeResetUserRepo struct {
	users   []*model.User
	updated *model.User
}

func (f *fakeResetUserRepo) find(pred func(*model.User) bool) (*model.User, error) {
	for _, u := range f.users {
		if pred(u) {
			cp := *u
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeResetUserRepo) FindByID(id uint64) (*model.User, error) {
	return f.find(func(u *model.User) bool { return u.ID == id })
}
func (f *fakeResetUserRepo) FindByUsername(n string) (*model.User, error) {
	return f.find(func(u *model.User) bool { return u.Username == n })
}
func (f *fakeResetUserRepo) FindByEmail(e string) (*model.User, error) {
	return f.find(func(u *model.User) bool { return u.Email == e })
}
func (f *fakeResetUserRepo) Update(u *model.User) error {
	f.updated = u
	return nil
}
func (f *fakeResetUserRepo) Create(*model.User) error                    { panic("unused") }
func (f *fakeResetUserRepo) FindByLinuxDoID(uint64) (*model.User, error) { panic("unused") }
func (f *fakeResetUserRepo) List(repository.UserListReq) ([]model.User, int64, error) { panic("unused") }
func (f *fakeResetUserRepo) FindByIDs([]uint64) ([]model.User, error)               { panic("unused") }
func (f *fakeResetUserRepo) UpdateSubject(uint64, uint64) error                      { panic("unused") }

type fakeResetRepo struct {
	rows   []model.PasswordReset
	nextID uint64
}

func (f *fakeResetRepo) Create(p *model.PasswordReset) error {
	f.nextID++
	p.ID = f.nextID
	f.rows = append(f.rows, *p)
	return nil
}
func (f *fakeResetRepo) FindByID(id uint64) (*model.PasswordReset, error) {
	for i := range f.rows {
		if f.rows[i].ID == id {
			return &f.rows[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeResetRepo) UpdateStatus(id uint64, status int8, note string) error {
	for i := range f.rows {
		if f.rows[i].ID == id {
			f.rows[i].Status = status
			f.rows[i].AdminNote = note
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}
func (f *fakeResetRepo) CountPendingByUser(userID uint64) (int64, error) {
	var n int64
	for _, r := range f.rows {
		if r.UserID == userID && r.Status == 0 {
			n++
		}
	}
	return n, nil
}
func (f *fakeResetRepo) CountPending() (int64, error) { return 0, nil }
func (f *fakeResetRepo) ListAll(int, int, int) ([]repository.PasswordResetView, int64, error) {
	return nil, 0, nil
}

func newResetSvc() (*PasswordResetService, *fakeResetRepo, *fakeResetUserRepo) {
	users := &fakeResetUserRepo{users: []*model.User{
		{ID: 1, Username: "alice", Email: "alice@example.com", Status: 1},
		{ID: 2, Username: "banned", Email: "banned@example.com", Status: 0},
	}}
	repo := &fakeResetRepo{}
	return NewPasswordResetService(repo, users), repo, users
}

// 账号不存在时不能建行也不能报错 —— 任一差异都能被拿来枚举站内账号，
// 而管理员看到的列表本身就是一份「这些账号真实存在」的名单。
func TestSubmitSilentForUnknownAccount(t *testing.T) {
	svc, repo, _ := newResetSvc()

	if err := svc.Submit("nobody", "qq:1", "忘了", "1.2.3.4"); err != nil {
		t.Fatalf("未知账号不应报错: %v", err)
	}
	if len(repo.rows) != 0 {
		t.Error("未知账号不应建行")
	}
}

func TestSubmitAcceptsUsernameAndEmail(t *testing.T) {
	for _, acct := range []string{"alice", "alice@example.com"} {
		svc, repo, _ := newResetSvc()
		if err := svc.Submit(acct, "qq:1", "忘了", "1.2.3.4"); err != nil {
			t.Fatalf("%s: %v", acct, err)
		}
		if len(repo.rows) != 1 || repo.rows[0].UserID != 1 {
			t.Errorf("%s 未正确关联用户: %+v", acct, repo.rows)
		}
	}
}

// 封禁账号处理了也登不进来，不该占后台人力。
func TestSubmitIgnoresBannedAccount(t *testing.T) {
	svc, repo, _ := newResetSvc()
	_ = svc.Submit("banned", "", "", "1.2.3.4")
	if len(repo.rows) != 0 {
		t.Error("封禁账号不应建行")
	}
}

// 限流按 IP，换 IP 就能绕过；单账号待处理上限是第二道闸。
func TestSubmitCapsPendingPerUser(t *testing.T) {
	svc, repo, _ := newResetSvc()
	for i := 0; i < pendingPerUser+3; i++ {
		_ = svc.Submit("alice", "", "", "1.2.3.4")
	}
	if len(repo.rows) != pendingPerUser {
		t.Errorf("expected %d rows, got %d", pendingPerUser, len(repo.rows))
	}
}

func TestApproveResetsPasswordAndClosesRequest(t *testing.T) {
	svc, repo, users := newResetSvc()
	_ = svc.Submit("alice", "", "", "1.2.3.4")

	if err := svc.Approve(1, "newpass123", "已电话核实"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if users.updated == nil {
		t.Fatal("密码未被更新")
	}
	if bcrypt.CompareHashAndPassword([]byte(users.updated.PasswordHash), []byte("newpass123")) != nil {
		t.Error("新密码哈希不匹配")
	}
	if repo.rows[0].Status != 1 || repo.rows[0].AdminNote != "已电话核实" {
		t.Errorf("申请未标记为已处理: %+v", repo.rows[0])
	}
}

// 明文密码不能落进申请表 —— 存下来等于自建一张明文密码表。
func TestApproveDoesNotStorePlaintextPassword(t *testing.T) {
	svc, repo, _ := newResetSvc()
	_ = svc.Submit("alice", "qq:1", "忘了", "1.2.3.4")
	_ = svc.Approve(1, "newpass123", "已核实")

	r := repo.rows[0]
	for _, f := range []string{r.Account, r.Contact, r.Reason, r.AdminNote} {
		if strings.Contains(f, "newpass123") {
			t.Errorf("明文密码泄漏进申请表: %q", f)
		}
	}
}

// 已处理的申请重复通过 = 二次改密码，必须拒掉。
func TestApproveRejectsAlreadyHandled(t *testing.T) {
	svc, _, _ := newResetSvc()
	_ = svc.Submit("alice", "", "", "1.2.3.4")
	_ = svc.Approve(1, "newpass123", "")

	if err := svc.Approve(1, "another123", ""); err == nil {
		t.Error("重复处理应被拒绝")
	}
	if err := svc.Reject(1, ""); err == nil {
		t.Error("已处理的申请不应可忽略")
	}
}

func TestApproveRejectsShortPassword(t *testing.T) {
	svc, _, users := newResetSvc()
	_ = svc.Submit("alice", "", "", "1.2.3.4")

	if err := svc.Approve(1, "123", ""); err == nil {
		t.Error("弱密码应被拒绝")
	}
	if users.updated != nil {
		t.Error("校验失败时不应改动用户")
	}
}

func TestRejectLeavesPasswordUntouched(t *testing.T) {
	svc, repo, users := newResetSvc()
	_ = svc.Submit("alice", "", "", "1.2.3.4")

	if err := svc.Reject(1, "无法核实身份"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if users.updated != nil {
		t.Error("忽略申请不应改密码")
	}
	if repo.rows[0].Status != 2 {
		t.Errorf("状态应为已忽略，实际 %d", repo.rows[0].Status)
	}
}
