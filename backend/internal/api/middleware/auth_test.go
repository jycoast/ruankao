package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"software-exam/config"
	"software-exam/internal/application/service"
	"software-exam/internal/domain/model"
	"software-exam/internal/domain/repository"
	"software-exam/pkg/auth"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// fakeUserRepo 只实现 FindByID，其余方法用不到。
type fakeUserRepo struct {
	user  *model.User
	calls int
}

func (f *fakeUserRepo) FindByID(id uint64) (*model.User, error) {
	f.calls++
	if f.user == nil || f.user.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *f.user
	return &cp, nil
}

func (f *fakeUserRepo) Create(*model.User) error                    { panic("unused") }
func (f *fakeUserRepo) FindByUsername(string) (*model.User, error)  { panic("unused") }
func (f *fakeUserRepo) FindByEmail(string) (*model.User, error)     { panic("unused") }
func (f *fakeUserRepo) FindByLinuxDoID(uint64) (*model.User, error) { panic("unused") }
func (f *fakeUserRepo) Update(*model.User) error                    { panic("unused") }
func (f *fakeUserRepo) List(repository.UserListReq) ([]model.User, int64, error) { panic("unused") }
func (f *fakeUserRepo) FindByIDs([]uint64) ([]model.User, error)                { panic("unused") }
func (f *fakeUserRepo) UpdateSubject(uint64, uint64) error                      { panic("unused") }

func authTestRouter(repo *fakeUserRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/t", AuthMiddleware(service.NewUserStateService(repo)), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"role": c.GetInt("role_probe"), "ok": true})
	})
	return r
}

func doAuthed(r *gin.Engine, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/t", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	return w
}

func tokenFor(t *testing.T, u *model.User) string {
	t.Helper()
	config.AppConfig.JWT.Secret = "middleware-test-secret-at-least-32-chars!!"
	config.AppConfig.JWT.ExpireHour = 72
	tok, err := auth.GenerateToken(u.ID, u.Username, u.Role)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// 核心：token 仍在有效期内，但账号已在库里被禁用 —— 必须拒绝。
// 这正是原实现的漏洞：封号后对方 token 还能用满 72 小时。
func TestAuthRejectsBannedUserWithValidToken(t *testing.T) {
	u := &model.User{ID: 7, Username: "u7", Role: 0, Status: 1}
	tok := tokenFor(t, u)

	repo := &fakeUserRepo{user: u}
	r := authTestRouter(repo)

	if w := doAuthed(r, tok); w.Code != http.StatusOK {
		t.Fatalf("active user should pass, got %d %s", w.Code, w.Body.String())
	}

	u.Status = 0 // 管理员封号
	w := doAuthed(r, tok)
	// 业务错误恒为 HTTP 200，错误码在 body 里
	if body := w.Body.String(); !strings.Contains(body, `"code":401`) {
		t.Fatalf("banned user should be rejected with 401, got %s", body)
	}
}

// 降权同理：token 里的 role 是签发时的快照，必须以库为准。
func TestAuthUsesDatabaseRoleNotToken(t *testing.T) {
	u := &model.User{ID: 8, Username: "u8", Role: 1, Status: 1}
	tok := tokenFor(t, u) // 以管理员身份签发

	u.Role = 0 // 随后被降权
	repo := &fakeUserRepo{user: u}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin", AuthMiddleware(service.NewUserStateService(repo)), AdminMiddleware(),
		func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)

	if body := w.Body.String(); !strings.Contains(body, `"code":403`) {
		t.Fatalf("demoted admin should be forbidden, got %s", body)
	}
}

// 用户已被删除时不应放行。
func TestAuthRejectsDeletedUser(t *testing.T) {
	u := &model.User{ID: 9, Username: "u9", Role: 0, Status: 1}
	tok := tokenFor(t, u)
	r := authTestRouter(&fakeUserRepo{user: nil})

	if body := doAuthed(r, tok).Body.String(); !strings.Contains(body, `"code":401`) {
		t.Fatalf("deleted user should be rejected, got %s", body)
	}
}
