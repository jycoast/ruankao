package repository

import (
	"software-exam/internal/domain/model"

	"gorm.io/gorm"
)

type UserListReq struct {
	Page     int
	PageSize int
	Keyword  string // 模糊匹配用户名 / 邮箱 / 昵称
	Role     int8   // -1=不过滤；0=普通用户；1=管理员
	Status   int8   // -1=不过滤；0=禁用；1=正常
}

type UserRepository interface {
	Create(user *model.User) error
	FindByID(id uint64) (*model.User, error)
	FindByIDs(ids []uint64) ([]model.User, error)
	FindByUsername(username string) (*model.User, error)
	FindByEmail(email string) (*model.User, error)
	FindByLinuxDoID(linuxdoID uint64) (*model.User, error)
	Update(user *model.User) error
	UpdateSubject(userID, subjectID uint64) error
	List(req UserListReq) ([]model.User, int64, error)
}

type UserRepositoryImpl struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &UserRepositoryImpl{db: db}
}

func (r *UserRepositoryImpl) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepositoryImpl) FindByID(id uint64) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	return &user, err
}

// FindByIDs 批量查询多个用户（管理后台在线列表用，避免逐用户 N+1 点查）。
func (r *UserRepositoryImpl) FindByIDs(ids []uint64) ([]model.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var list []model.User
	err := r.db.Where("id IN ?", ids).Find(&list).Error
	return list, err
}

func (r *UserRepositoryImpl) FindByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ?", username).First(&user).Error
	return &user, err
}

func (r *UserRepositoryImpl) FindByEmail(email string) (*model.User, error) {
	var user model.User
	err := r.db.Where("email = ?", email).First(&user).Error
	return &user, err
}

func (r *UserRepositoryImpl) FindByLinuxDoID(linuxdoID uint64) (*model.User, error) {
	var user model.User
	err := r.db.Where("linuxdo_id = ? AND linuxdo_id != 0", linuxdoID).First(&user).Error
	return &user, err
}

func (r *UserRepositoryImpl) Update(user *model.User) error {
	return r.db.Save(user).Error
}

// UpdateSubject 只更新科目偏好列：Update 全量保存会与并发改昵称/头像互相覆盖。
func (r *UserRepositoryImpl) UpdateSubject(userID, subjectID uint64) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).Update("subject_id", subjectID).Error
}

func (r *UserRepositoryImpl) List(req UserListReq) ([]model.User, int64, error) {
	var users []model.User
	var total int64
	query := r.db.Model(&model.User{})
	if kw := req.Keyword; kw != "" {
		like := "%" + kw + "%"
		query = query.Where("username LIKE ? OR email LIKE ? OR nickname LIKE ?", like, like, like)
	}
	if req.Role != -1 {
		query = query.Where("role = ?", req.Role)
	}
	if req.Status != -1 {
		query = query.Where("status = ?", req.Status)
	}
	query.Count(&total)
	err := query.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id DESC").Find(&users).Error
	return users, total, err
}
