package service

import (
	"errors"
	"fmt"
	"log"

	"software-exam/config"
	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
	"software-exam/pkg/auth"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepo      repository.UserRepository
	profileRepo   repository.UserProfileRepository
	membershipSvc *MembershipService
	notifSvc      *NotificationService
	subjectRepo   repository.SubjectRepository
}

func NewUserService(userRepo repository.UserRepository, profileRepo repository.UserProfileRepository, membershipSvc *MembershipService, notifSvc *NotificationService, subjectRepo repository.SubjectRepository) *UserService {
	return &UserService{userRepo: userRepo, profileRepo: profileRepo, membershipSvc: membershipSvc, notifSvc: notifSvc, subjectRepo: subjectRepo}
}

func (s *UserService) Register(username, email, password, nickname string) (string, uint64, error) {
	if _, err := s.userRepo.FindByUsername(username); err == nil {
		return "", 0, errors.New("用户名已存在")
	}
	if _, err := s.userRepo.FindByEmail(email); err == nil {
		return "", 0, errors.New("邮箱已注册")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", 0, err
	}

	user := &model.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Nickname:     nickname,
		Role:         0,
		Status:       1,
	}
	if err := s.userRepo.Create(user); err != nil {
		return "", 0, err
	}

	profile := &model.UserProfile{
		UserID: user.ID,
	}
	s.profileRepo.Create(profile)

	token, err := auth.GenerateToken(user.ID, user.Username, user.Role)
	return token, user.ID, err
}

func (s *UserService) Login(username, password string) (string, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return "", errors.New("用户名或密码错误")
	}
	if user.Status != 1 {
		return "", errors.New("账号已被禁用")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("用户名或密码错误")
	}

	return auth.GenerateToken(user.ID, user.Username, user.Role)
}

// GuestLogin 虚拟游客登录:不注册、不落行,签一个 role=2 的只读短令牌。
func (s *UserService) GuestLogin() (string, error) {
	return auth.GenerateGuestToken()
}

func (s *UserService) GetProfile(userID uint64) (interface{}, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	profile, _ := s.profileRepo.FindByUserID(userID)
	return gin.H{
		"user":         user,
		"profile":      profile,
		"has_password": user.PasswordHash != "",
	}, nil
}

func (s *UserService) UpdateProfile(userID uint64, nickname, avatar, phone string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if nickname != "" {
		user.Nickname = nickname
	}
	if avatar != "" {
		user.Avatar = avatar
	}
	if phone != "" {
		user.Phone = phone
	}
	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	return nil
}

func (s *UserService) GetStats(userID uint64) (*model.UserProfile, error) {
	profile, err := s.profileRepo.FindByUserID(userID)
	if err != nil {
		profile = &model.UserProfile{
			UserID:        userID,
			TotalPractice: 0,
			TotalCorrect:  0,
			TotalTime:     0,
			StreakDays:    0,
		}
		// 真用户无行时懒建;虚拟游客(user_id=0)是内存主键,绝不落任何 DB 行
		if userID != 0 {
			s.profileRepo.Create(profile)
		}
	}
	return profile, nil
}

var ErrSubjectInvalid = errors.New("subject not found or disabled")

// UpdateSubject 登录用户的科目偏好落库。科目必须存在且启用，防脏数据进 users.subject_id。
func (s *UserService) UpdateSubject(userID, subjectID uint64) (uint64, error) {
	if subjectID == 0 {
		return 0, ErrSubjectInvalid
	}
	sub, err := s.subjectRepo.FindByID(subjectID)
	if err != nil || sub.Status != 1 {
		return 0, ErrSubjectInvalid
	}
	if err := s.userRepo.UpdateSubject(userID, subjectID); err != nil {
		return 0, err
	}
	return subjectID, nil
}

func (s *UserService) ChangePassword(userID uint64, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}
	// OAuth 注册的账号没有密码，此时是首次设置，不校验原密码
	if user.PasswordHash != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
			return errors.New("原密码错误")
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	return s.userRepo.Update(user)
}

// FindOrCreateByLinuxDo 用于 OAuth 登录：按 LinuxDo ID 查找，或按邮箱合并，或新建账号
func (s *UserService) FindOrCreateByLinuxDo(linuxdoID uint64, ldUsername, email, nickname, avatar string) (*model.User, string, error) {
	// 1. 按 linuxdo_id 查找
	user, err := s.userRepo.FindByLinuxDoID(linuxdoID)
	if err == nil {
		if user.Status != 1 {
			return nil, "", errors.New("账号已被禁用")
		}
		// 更新头像和昵称
		if avatar != "" {
			user.Avatar = avatar
		}
		if nickname != "" {
			user.Nickname = nickname
		}
		_ = s.userRepo.Update(user)
		token, err := auth.GenerateToken(user.ID, user.Username, user.Role)
		return user, token, err
	}

	// 2. 按邮箱合并已有账号
	if email != "" {
		user, err = s.userRepo.FindByEmail(email)
		if err == nil {
			if user.Status != 1 {
				return nil, "", errors.New("账号已被禁用")
			}
			user.LinuxDoID = linuxdoID
			if nickname != "" && user.Nickname == "" {
				user.Nickname = nickname
			}
			if avatar != "" && user.Avatar == "" {
				user.Avatar = avatar
			}
			_ = s.userRepo.Update(user)
			token, err := auth.GenerateToken(user.ID, user.Username, user.Role)
			return user, token, err
		}
	}

	// 3. 新建账号：用户名冲突时追加 _1 _2 …
	finalUsername := ldUsername
	for i := 1; ; i++ {
		if _, err := s.userRepo.FindByUsername(finalUsername); err != nil {
			break // 可用
		}
		finalUsername = fmt.Sprintf("%s_%d", ldUsername, i)
	}

	finalEmail := email
	if finalEmail == "" {
		finalEmail = fmt.Sprintf("linuxdo_%d@linuxdo.connect", linuxdoID)
	}

	newUser := &model.User{
		Username:     finalUsername,
		Email:        finalEmail,
		PasswordHash: "",
		Nickname:     nickname,
		Avatar:       avatar,
		Role:         0,
		Status:       1,
		LinuxDoID:    linuxdoID,
	}
	if err := s.userRepo.Create(newUser); err != nil {
		return nil, "", err
	}
	_ = s.profileRepo.Create(&model.UserProfile{UserID: newUser.ID})

	// 新建账号即首次登录：发放首登月度会员（受配置开关与幂等约束保护）。
	s.grantFirstLoginBonus(newUser.ID)

	token, err := auth.GenerateToken(newUser.ID, newUser.Username, newUser.Role)
	return newUser, token, err
}

// grantFirstLoginBonus LinuxDo OAuth 首次登录赠送会员。
// 仅在配置开启、且该用户在 memberships 表里从未有过记录时发放一次；
// 已存在记录（含之前靠付费/卡密拿过会员、或并发已发过）则跳过，天然幂等。
// 发放失败只记日志、不影响登录主流程——会员是福利，登录本身必须成功。
func (s *UserService) grantFirstLoginBonus(userID uint64) {
	if s == nil || s.membershipSvc == nil || userID == 0 {
		return
	}
	bonus := config.AppConfig.OAuth.FirstLoginBonus
	if !bonus.Enable {
		return
	}
	plan := bonus.Plan
	if plan == "" {
		plan = "month"
	}
	days := bonus.Days
	if days <= 0 {
		days = 30
	}

	exists := s.membershipSvc.ExistsByUserID(userID)
	if exists {
		return
	}

	if err := s.membershipSvc.Grant(userID, plan, days); err != nil {
		log.Printf("[first-login-bonus] grant failed user=%d plan=%s days=%d: %v", userID, plan, days, err)
		return
	}

	title := bonus.Title
	if title == "" {
		title = "首登福利到账"
	}
	content := fmt.Sprintf("欢迎通过 LINUX DO 登录！已为您赠送 %d 天月度会员，祝备考顺利。", days)
	if s.notifSvc != nil {
		s.notifSvc.Notify(userID, title, content, "first_login_bonus", 0, "")
	}
	log.Printf("[first-login-bonus] granted user=%d plan=%s days=%d", userID, plan, days)
}
