package service

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
)

// fakeSubjectRepo 只实现 UpdateSubject 路径用到的 FindByID。
type fakeSubjectRepo struct {
	rows map[uint64]*model.Subject
}

func (f *fakeSubjectRepo) List() ([]model.Subject, error) { return nil, nil }
func (f *fakeSubjectRepo) FindByID(id uint64) (*model.Subject, error) {
	if s, ok := f.rows[id]; ok {
		return s, nil
	}
	return nil, gorm.ErrRecordNotFound
}

// fakeUserRepoForSubject 只实现 UpdateSubject 路径用到的 FindByID/UpdateSubject。
type fakeUserRepoForSubject struct {
	users      map[uint64]*model.User
	savedSubID uint64
	savedCount int
}

func (f *fakeUserRepoForSubject) Create(*model.User) error { return nil }
func (f *fakeUserRepoForSubject) FindByID(id uint64) (*model.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeUserRepoForSubject) FindByIDs([]uint64) ([]model.User, error) { return nil, nil }
func (f *fakeUserRepoForSubject) FindByUsername(string) (*model.User, error) {
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeUserRepoForSubject) FindByEmail(string) (*model.User, error) {
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeUserRepoForSubject) FindByLinuxDoID(uint64) (*model.User, error) {
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeUserRepoForSubject) Update(*model.User) error { return nil }
func (f *fakeUserRepoForSubject) List(repository.UserListReq) ([]model.User, int64, error) {
	return nil, 0, nil
}
func (f *fakeUserRepoForSubject) UpdateSubject(userID, subjectID uint64) error {
	f.savedSubID = subjectID
	f.savedCount++
	return nil
}

func newSubjectServiceForTest() (*UserService, *fakeUserRepoForSubject) {
	userRepo := &fakeUserRepoForSubject{users: map[uint64]*model.User{
		7: {ID: 7, SubjectID: 1},
	}}
	// id=3 启用、id=4 停用、id=999 不存在
	subjectRepo := &fakeSubjectRepo{rows: map[uint64]*model.Subject{
		3: {ID: 3, Name: "软件设计师", Status: 1},
		4: {ID: 4, Name: "信息系统项目管理师", Status: 0},
	}}
	svc := &UserService{userRepo: userRepo, subjectRepo: subjectRepo}
	return svc, userRepo
}

func TestUpdateSubject_OK(t *testing.T) {
	svc, userRepo := newSubjectServiceForTest()
	got, err := svc.UpdateSubject(7, 3)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 3 {
		t.Fatalf("subject_id = %d, want 3", got)
	}
	if userRepo.savedSubID != 3 || userRepo.savedCount != 1 {
		t.Fatalf("repo not called with 3: subID=%d count=%d", userRepo.savedSubID, userRepo.savedCount)
	}
}

func TestUpdateSubject_InvalidSubject(t *testing.T) {
	svc, _ := newSubjectServiceForTest()
	if _, err := svc.UpdateSubject(7, 999); !errors.Is(err, ErrSubjectInvalid) {
		t.Fatalf("want ErrSubjectInvalid, got %v", err)
	}
}

func TestUpdateSubject_DisabledSubject(t *testing.T) {
	svc, _ := newSubjectServiceForTest()
	if _, err := svc.UpdateSubject(7, 4); !errors.Is(err, ErrSubjectInvalid) {
		t.Fatalf("want ErrSubjectInvalid, got %v", err)
	}
}

func TestUpdateSubject_ZeroID(t *testing.T) {
	svc, _ := newSubjectServiceForTest()
	if _, err := svc.UpdateSubject(7, 0); !errors.Is(err, ErrSubjectInvalid) {
		t.Fatalf("want ErrSubjectInvalid, got %v", err)
	}
}
