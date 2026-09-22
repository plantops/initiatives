package app

import (
	"database/sql"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type App struct {
	DB           *sql.DB
	Templates    *template.Template
	UploadDir    string
	CookieSecure bool
	Logger       *log.Logger
}

type User struct {
	ID             int64
	Username       string
	EmployeeNo     string
	FullName       string
	DepartmentCode string
	DepartmentName string
	Roles          map[string]bool
	CSRFToken      string
}

type Option struct {
	Code string
	Name string
}

type Initiative struct {
	ID                       int64
	Code                     string
	Title                    string
	SubmitterEmployeeNo      string
	SubmitterName            string
	DepartmentCode           string
	DepartmentName           string
	ProblemCategoryCode      string
	ProblemCategoryName      string
	ProblemSummary           string
	ProblemDetail            string
	SolutionSummary          string
	SolutionDetail           string
	ImplementationStatusCode string
	ImplementationStatusName string
	WorkflowStatusCode       string
	WorkflowStatusName       string
	ExpectedValueVND         sql.NullInt64
	ActualValueVND           sql.NullInt64
	ValuePeriodCode          sql.NullString
	ValueCalculationMethod   sql.NullString
	ActualValueVerified      bool
	ActualValueVerifiedBy    sql.NullString
	ActualValueVerifiedAt    sql.NullString
	ManagerDueAt             sql.NullString
	SubmittedAt              sql.NullString
	CreatedAt                string
	UpdatedAt                string
}

type Benefit struct {
	ID                int64
	BenefitTypeCode   string
	BenefitTypeName   string
	Description       string
	BaselineValue     sql.NullFloat64
	TargetValue       sql.NullFloat64
	ActualValue       sql.NullFloat64
	UnitCode          sql.NullString
	ExpectedValueVND  sql.NullInt64
	ActualValueVND    sql.NullInt64
	ValuePeriodCode   sql.NullString
	CalculationMethod sql.NullString
	VerificationNote  sql.NullString
}

type Attachment struct {
	ID               int64
	DocumentTypeCode string
	DocumentTypeName string
	OriginalFilename string
	StorageKey       string
	MimeType         sql.NullString
	SizeBytes        sql.NullInt64
	SHA256Hex        sql.NullString
	UploadedBy       string
	UploadedByName   string
	UploadedAt       string
}

type History struct {
	ID          int64
	FromStatus  sql.NullString
	ToStatus    string
	ToName      string
	ChangedBy   string
	ChangedName string
	ChangedAt   string
	Note        sql.NullString
}

type ManagerReview struct {
	ReviewerName string
	DecisionCode string
	DecisionName string
	Comment      sql.NullString
	Reason       sql.NullString
	ReviewedAt   string
}

type CouncilAssessment struct {
	AssessorName       string
	DecisionCode       string
	DecisionName       string
	NoveltyComment     sql.NullString
	FeasibilityComment sql.NullString
	AssessedValueVND   sql.NullInt64
	RewardLevelCode    sql.NullString
	RewardLevelName    sql.NullString
	CouncilNote        sql.NullString
	CreatedAt          string
}

type Reward struct {
	ApprovedLevelCode sql.NullString
	ApprovedLevelName sql.NullString
	ApprovedAmountVND sql.NullInt64
	ApprovedByName    sql.NullString
	ApprovedAt        sql.NullString
	PaidAt            sql.NullString
	Note              sql.NullString
}

type PageData struct {
	User                   *User
	Title                  string
	Error                  string
	Message                string
	Initiatives            []Initiative
	Initiative             *Initiative
	Benefits               []Benefit
	Attachments            []Attachment
	History                []History
	ManagerReviews         []ManagerReview
	CouncilAssessments     []CouncilAssessment
	Reward                 *Reward
	ProblemCategories      []Option
	BenefitTypes           []Option
	Units                  []Option
	ValuePeriods           []Option
	ImplementationStatuses []Option
	RewardLevels           []Option
	DocumentTypes          []Option
	Departments            []Department
	Employees              []Employee
	Users                  []AdminUser
	Roles                  []Option
	MasterGroups           map[string][]Option
	CanEdit                bool
	CanSubmit              bool
	CanManagerReview       bool
	CanHRDForward          bool
	CanCouncilAssess       bool
	CanGMApprove           bool
	CanVerify              bool
	CanReward              bool
	CanClose               bool
}

type Department struct {
	Code   string
	Name   string
	Active bool
}

type Employee struct {
	EmployeeNo     string
	FullName       string
	DepartmentCode string
	DepartmentName string
	PositionTitle  sql.NullString
	Active         bool
}

type AdminUser struct {
	ID         int64
	Username   string
	EmployeeNo string
	FullName   string
	Roles      string
	Active     bool
}

func New(dbPath, uploadDir string, cookieSecure bool, logger *log.Logger) (*App, error) {
	if logger == nil {
		logger = log.Default()
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err = db.Exec(`PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite pragmas: %w", err)
	}
	migration, err := fs.ReadFile(assets, "migrations/001_init.sql")
	if err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.Exec(string(migration)); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	funcs := template.FuncMap{
		"hasRole": func(u *User, role string) bool {
			return u != nil && u.Roles[role]
		},
		"money": formatMoney,
		"nullMoney": func(v sql.NullInt64) string {
			if !v.Valid {
				return "—"
			}
			return formatMoney(v.Int64)
		},
		"nullText": func(v sql.NullString) string {
			if !v.Valid || strings.TrimSpace(v.String) == "" {
				return "—"
			}
			return v.String
		},
		"boolText": func(v bool) string {
			if v {
				return "Có"
			}
			return "Không"
		},
	}
	t, err := template.New("root").Funcs(funcs).ParseFS(assets, "web/templates/*.html")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &App{DB: db, Templates: t, UploadDir: uploadDir, CookieSecure: cookieSecure, Logger: logger}, nil
}

func (a *App) Close() error { return a.DB.Close() }

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()

	staticFS, _ := fs.Sub(assets, "web/static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /setup", a.setupGet)
	mux.HandleFunc("POST /setup", a.setupPost)
	mux.HandleFunc("GET /login", a.loginGet)
	mux.HandleFunc("POST /login", a.loginPost)
	mux.HandleFunc("POST /logout", a.withUser(a.logoutPost))

	mux.HandleFunc("GET /", a.withUser(a.dashboard))
	mux.HandleFunc("GET /initiatives/new", a.withUser(a.initiativeNewGet))
	mux.HandleFunc("POST /initiatives", a.withUser(a.initiativeCreatePost))
	mux.HandleFunc("GET /initiatives/{id}", a.withUser(a.initiativeDetail))
	mux.HandleFunc("GET /initiatives/{id}/edit", a.withUser(a.initiativeEditGet))
	mux.HandleFunc("POST /initiatives/{id}/edit", a.withUser(a.initiativeEditPost))
	mux.HandleFunc("POST /initiatives/{id}/submit", a.withUser(a.initiativeSubmitPost))
	mux.HandleFunc("POST /initiatives/{id}/manager-review", a.withUser(a.managerReviewPost))
	mux.HandleFunc("POST /initiatives/{id}/hrd-forward", a.withUser(a.hrdForwardPost))
	mux.HandleFunc("POST /initiatives/{id}/council-assess", a.withUser(a.councilAssessPost))
	mux.HandleFunc("POST /initiatives/{id}/gm-approve", a.withUser(a.gmApprovePost))
	mux.HandleFunc("POST /initiatives/{id}/verify", a.withUser(a.verifyPost))
	mux.HandleFunc("POST /initiatives/{id}/reward", a.withUser(a.rewardPost))
	mux.HandleFunc("POST /initiatives/{id}/close", a.withUser(a.closePost))
	mux.HandleFunc("POST /initiatives/{id}/attachments", a.withUser(a.attachmentPost))
	mux.HandleFunc("GET /attachments/{id}/download", a.withUser(a.attachmentDownload))

	mux.HandleFunc("GET /admin/master", a.withRoles([]string{"ADMIN"}, a.masterGet))
	mux.HandleFunc("POST /admin/departments", a.withRoles([]string{"ADMIN"}, a.departmentPost))
	mux.HandleFunc("POST /admin/employees", a.withRoles([]string{"ADMIN"}, a.employeePost))
	mux.HandleFunc("POST /admin/users", a.withRoles([]string{"ADMIN"}, a.userPost))
	mux.HandleFunc("POST /account/password", a.withUser(a.changePasswordPost))

	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; img-src 'self' data:; form-action 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (a *App) render(w http.ResponseWriter, name string, data PageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.Templates.ExecuteTemplate(w, name, data); err != nil {
		a.Logger.Printf("render %s: %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 2*time.Second)
	defer cancel()
	if err := a.DB.PingContext(ctx); err != nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func formatMoney(v int64) string {
	s := fmt.Sprintf("%d", v)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	if neg {
		s = "-" + s
	}
	return s + " VND"
}
