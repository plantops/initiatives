package app

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"initiative-app/internal/security"
)

type userContextKey struct{}

func (a *App) usersExist() (bool, error) {
	var n int
	err := a.DB.QueryRow(`SELECT COUNT(*) FROM app_user`).Scan(&n)
	return n > 0, err
}

func (a *App) currentUser(r *http.Request) (*User, error) {
	c, err := r.Cookie("initiative_session")
	if err != nil || strings.TrimSpace(c.Value) == "" {
		return nil, sql.ErrNoRows
	}
	now := time.Now().UTC().Format(time.RFC3339)
	row := a.DB.QueryRow(`
        SELECT u.id, u.username, u.employee_no, e.full_name, e.department_code, d.name, s.csrf_token
        FROM app_session s
        JOIN app_user u ON u.id = s.user_id AND u.active = 1
        JOIN md_employee e ON e.employee_no = u.employee_no AND e.active = 1
        JOIN md_department d ON d.code = e.department_code
        WHERE s.token_hash = ? AND s.expires_at > ?
    `, tokenHash(c.Value), now)
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.EmployeeNo, &u.FullName, &u.DepartmentCode, &u.DepartmentName, &u.CSRFToken); err != nil {
		return nil, err
	}
	u.Roles = map[string]bool{}
	rows, err := a.DB.Query(`SELECT role_code FROM app_user_role WHERE user_id = ?`, u.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		u.Roles[role] = true
	}
	return &u, rows.Err()
}

func (a *App) withUser(next func(http.ResponseWriter, *http.Request, *User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
		}
		u, err := a.currentUser(r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if r.Method == http.MethodPost && !validCSRF(r, u.CSRFToken) {
			http.Error(w, "invalid CSRF token", http.StatusForbidden)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, u)), u)
	}
}

func (a *App) withRoles(roles []string, next func(http.ResponseWriter, *http.Request, *User)) http.HandlerFunc {
	return a.withUser(func(w http.ResponseWriter, r *http.Request, u *User) {
		for _, role := range roles {
			if u.Roles[role] {
				next(w, r, u)
				return
			}
		}
		http.Error(w, "forbidden", http.StatusForbidden)
	})
}

func validCSRF(r *http.Request, expected string) bool {
	if err := r.ParseMultipartForm(12 << 20); err != nil && err != http.ErrNotMultipart {
		_ = r.ParseForm()
	} else {
		_ = r.ParseForm()
	}
	got := r.FormValue("_csrf")
	if len(got) != len(expected) || got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

func (a *App) createSession(w http.ResponseWriter, userID int64) error {
	token, err := randomToken(32)
	if err != nil {
		return err
	}
	csrf, err := randomToken(24)
	if err != nil {
		return err
	}
	expires := time.Now().UTC().Add(12 * time.Hour)
	if _, err := a.DB.Exec(`INSERT INTO app_session(token_hash,user_id,csrf_token,expires_at) VALUES(?,?,?,?)`,
		tokenHash(token), userID, csrf, expires.Format(time.RFC3339)); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: "initiative_session", Value: token, Path: "/",
		HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteStrictMode,
		Expires: expires,
	})
	return nil
}

func (a *App) loginGet(w http.ResponseWriter, r *http.Request) {
	exists, err := a.usersExist()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if !exists {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	if _, err := a.currentUser(r); err == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	a.render(w, "login", PageData{Title: "Đăng nhập"})
}

func (a *App) loginPost(w http.ResponseWriter, r *http.Request) {
	exists, err := a.usersExist()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if !exists {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	var id int64
	var hash string
	err = a.DB.QueryRow(`SELECT id,password_hash FROM app_user WHERE username = ? COLLATE NOCASE AND active = 1`, username).Scan(&id, &hash)
	if err != nil || !security.VerifyPassword(hash, password) {
		time.Sleep(250 * time.Millisecond)
		a.render(w, "login", PageData{Title: "Đăng nhập", Error: "Sai tài khoản hoặc mật khẩu."})
		return
	}
	_, _ = a.DB.Exec(`DELETE FROM app_session WHERE expires_at <= ?`, time.Now().UTC().Format(time.RFC3339))
	if err := a.createSession(w, id); err != nil {
		http.Error(w, "cannot create session", 500)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) logoutPost(w http.ResponseWriter, r *http.Request, u *User) {
	if c, err := r.Cookie("initiative_session"); err == nil {
		_, _ = a.DB.Exec(`DELETE FROM app_session WHERE token_hash = ?`, tokenHash(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "initiative_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) setupGet(w http.ResponseWriter, r *http.Request) {
	exists, err := a.usersExist()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if exists {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	a.render(w, "setup", PageData{Title: "Thiết lập lần đầu"})
}

func (a *App) setupPost(w http.ResponseWriter, r *http.Request) {
	exists, err := a.usersExist()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if exists {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	deptCode := cleanCode(r.FormValue("department_code"))
	deptName := strings.TrimSpace(r.FormValue("department_name"))
	employeeNo := cleanCode(r.FormValue("employee_no"))
	fullName := strings.TrimSpace(r.FormValue("full_name"))
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	if deptCode == "" || deptName == "" || employeeNo == "" || fullName == "" || username == "" {
		a.render(w, "setup", PageData{Title: "Thiết lập lần đầu", Error: "Thiếu trường bắt buộc."})
		return
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		a.render(w, "setup", PageData{Title: "Thiết lập lần đầu", Error: err.Error()})
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO md_department(code,name,active) VALUES(?,?,1)`, deptCode, deptName); err != nil {
		a.render(w, "setup", PageData{Title: "Thiết lập lần đầu", Error: "Không tạo được phòng ban: " + err.Error()})
		return
	}
	if _, err = tx.Exec(`INSERT INTO md_employee(employee_no,full_name,department_code,position_title,active) VALUES(?,?,?,?,1)`, employeeNo, fullName, deptCode, "Administrator"); err != nil {
		a.render(w, "setup", PageData{Title: "Thiết lập lần đầu", Error: "Không tạo được nhân viên: " + err.Error()})
		return
	}
	res, err := tx.Exec(`INSERT INTO app_user(username,password_hash,employee_no,active) VALUES(?,?,?,1)`, username, hash, employeeNo)
	if err != nil {
		a.render(w, "setup", PageData{Title: "Thiết lập lần đầu", Error: "Không tạo được user: " + err.Error()})
		return
	}
	userID, _ := res.LastInsertId()
	if _, err = tx.Exec(`INSERT INTO app_user_role(user_id,role_code) VALUES(?, 'ADMIN'), (?, 'EMPLOYEE')`, userID, userID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := a.createSession(w, userID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/master", http.StatusSeeOther)
}

func (a *App) changePasswordPost(w http.ResponseWriter, r *http.Request, u *User) {
	newPassword := r.FormValue("new_password")
	hash, err := security.HashPassword(newPassword)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if _, err := a.DB.Exec(`UPDATE app_user SET password_hash = ? WHERE id = ?`, hash, u.ID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	_, _ = a.DB.Exec(`DELETE FROM app_session WHERE user_id = ?`, u.ID)
	http.SetCookie(w, &http.Cookie{Name: "initiative_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
