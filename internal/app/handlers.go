package app

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"initiative-app/internal/security"
)

func (a *App) dashboard(w http.ResponseWriter, r *http.Request, u *User) {
	rows, err := a.listInitiatives(u, r.URL.Query().Get("q"))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	a.render(w, "dashboard", PageData{User: u, Title: "Sáng kiến", Initiatives: rows})
}

func (a *App) formMasters() (pc, bt, units, periods, impl []Option, err error) {
	if pc, err = a.options("md_problem_category"); err != nil {
		return
	}
	if bt, err = a.options("md_benefit_type"); err != nil {
		return
	}
	if units, err = a.options("md_unit"); err != nil {
		return
	}
	if periods, err = a.options("md_value_period"); err != nil {
		return
	}
	impl, err = a.options("md_implementation_status")
	return
}

func (a *App) initiativeNewGet(w http.ResponseWriter, r *http.Request, u *User) {
	pc, bt, units, periods, impl, err := a.formMasters()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	a.render(w, "initiative_form", PageData{User: u, Title: "Sáng kiến mới", ProblemCategories: pc, BenefitTypes: bt, Units: units, ValuePeriods: periods, ImplementationStatuses: impl})
}

func parseInitiativeForm(r *http.Request) (InitiativeForm, error) {
	if err := r.ParseForm(); err != nil {
		return InitiativeForm{}, err
	}
	var f InitiativeForm
	f.Title = strings.TrimSpace(r.FormValue("title"))
	f.ProblemCategory = strings.TrimSpace(r.FormValue("problem_category"))
	f.ProblemSummary = strings.TrimSpace(r.FormValue("problem_summary"))
	f.ProblemDetail = strings.TrimSpace(r.FormValue("problem_detail"))
	f.SolutionSummary = strings.TrimSpace(r.FormValue("solution_summary"))
	f.SolutionDetail = strings.TrimSpace(r.FormValue("solution_detail"))
	f.ImplementationStatus = strings.TrimSpace(r.FormValue("implementation_status"))
	f.BenefitType = strings.TrimSpace(r.FormValue("benefit_type"))
	f.BenefitDescription = strings.TrimSpace(r.FormValue("benefit_description"))
	f.ValuePeriod = strings.TrimSpace(r.FormValue("value_period"))
	f.CalculationMethod = strings.TrimSpace(r.FormValue("calculation_method"))
	if f.Title == "" || f.ProblemSummary == "" || f.ProblemDetail == "" || f.SolutionSummary == "" || f.SolutionDetail == "" || f.ImplementationStatus == "" || f.BenefitType == "" || f.BenefitDescription == "" {
		return f, fmt.Errorf("thiếu trường bắt buộc")
	}
	v, valid, err := parseVND(r.FormValue("expected_value_vnd"))
	if err != nil {
		return f, err
	}
	f.ExpectedValue = v
	f.HasExpectedValue = valid
	return f, nil
}

func (a *App) initiativeCreatePost(w http.ResponseWriter, r *http.Request, u *User) {
	f, err := parseInitiativeForm(r)
	if err != nil {
		pc, bt, units, periods, impl, _ := a.formMasters()
		a.render(w, "initiative_form", PageData{User: u, Title: "Sáng kiến mới", Error: err.Error(), ProblemCategories: pc, BenefitTypes: bt, Units: units, ValuePeriods: periods, ImplementationStatuses: impl})
		return
	}
	id, err := a.createInitiative(u, f)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectInitiative(w, r, id)
}

func (a *App) initiativeDetail(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := a.getInitiative(id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	if !canView(u, i) {
		http.Error(w, "forbidden", 403)
		return
	}
	benefits, _ := a.loadBenefits(id)
	atts, _ := a.loadAttachments(id)
	hist, _ := a.loadHistory(id)
	mrs, _ := a.loadManagerReviews(id)
	cas, _ := a.loadCouncilAssessments(id)
	reward, _ := a.loadReward(id)
	rewardLevels, _ := a.options("md_reward_level")
	docTypes, _ := a.options("md_document_type")
	impl, _ := a.options("md_implementation_status")
	d := PageData{User: u, Title: i.Code + " — " + i.Title, Initiative: &i, Benefits: benefits, Attachments: atts, History: hist, ManagerReviews: mrs, CouncilAssessments: cas, Reward: reward, RewardLevels: rewardLevels, DocumentTypes: docTypes, ImplementationStatuses: impl}
	d.CanEdit = (i.WorkflowStatusCode == "DRAFT" && (i.SubmitterEmployeeNo == u.EmployeeNo || u.Roles["ADMIN"]))
	d.CanSubmit = d.CanEdit
	d.CanManagerReview = i.WorkflowStatusCode == "MANAGER_REVIEW" && (u.Roles["ADMIN"] || (u.Roles["MANAGER"] && i.DepartmentCode == u.DepartmentCode))
	d.CanHRDForward = i.WorkflowStatusCode == "HRD_REVIEW" && (u.Roles["HRD"] || u.Roles["ADMIN"])
	d.CanCouncilAssess = i.WorkflowStatusCode == "COUNCIL_REVIEW" && (u.Roles["COUNCIL"] || u.Roles["ADMIN"])
	d.CanGMApprove = i.WorkflowStatusCode == "GM_APPROVAL" && (u.Roles["GM"] || u.Roles["ADMIN"])
	d.CanVerify = (i.WorkflowStatusCode == "APPROVED" || i.WorkflowStatusCode == "VERIFICATION") && (u.Roles["HRD"] || u.Roles["ADMIN"])
	d.CanReward = i.WorkflowStatusCode == "VERIFICATION" && i.ActualValueVerified && (u.Roles["HRD"] || u.Roles["ADMIN"])
	d.CanClose = i.WorkflowStatusCode == "REWARDED" && (u.Roles["HRD"] || u.Roles["ADMIN"])
	a.render(w, "initiative_detail", d)
}

func (a *App) initiativeEditGet(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := a.getInitiative(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if i.WorkflowStatusCode != "DRAFT" || !(i.SubmitterEmployeeNo == u.EmployeeNo || u.Roles["ADMIN"]) {
		http.Error(w, "forbidden", 403)
		return
	}
	pc, bt, units, periods, impl, _ := a.formMasters()
	benefits, _ := a.loadBenefits(id)
	a.render(w, "initiative_edit", PageData{User: u, Title: "Sửa " + i.Code, Initiative: &i, Benefits: benefits, ProblemCategories: pc, BenefitTypes: bt, Units: units, ValuePeriods: periods, ImplementationStatuses: impl})
}

func (a *App) initiativeEditPost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := a.getInitiative(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if i.WorkflowStatusCode != "DRAFT" || !(i.SubmitterEmployeeNo == u.EmployeeNo || u.Roles["ADMIN"]) {
		http.Error(w, "forbidden", 403)
		return
	}
	f, err := parseInitiativeForm(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer tx.Rollback()
	_, err = tx.Exec(`UPDATE initiative SET title=?,problem_category_code=?,problem_summary=?,problem_detail=?,solution_summary=?,solution_detail=?,implementation_status_code=?,expected_value_vnd=?,value_period_code=?,value_calculation_method=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		f.Title, nullable(f.ProblemCategory), f.ProblemSummary, f.ProblemDetail, f.SolutionSummary, f.SolutionDetail, f.ImplementationStatus, nullableInt(f.ExpectedValue, f.HasExpectedValue), nullable(f.ValuePeriod), nullable(f.CalculationMethod), id)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var bid int64
	err = tx.QueryRow(`SELECT id FROM initiative_benefit WHERE initiative_id=? ORDER BY id LIMIT 1`, id).Scan(&bid)
	if err == sql.ErrNoRows {
		_, err = tx.Exec(`INSERT INTO initiative_benefit(initiative_id,benefit_type_code,description,expected_value_vnd,value_period_code,calculation_method) VALUES(?,?,?,?,?,?)`, id, f.BenefitType, f.BenefitDescription, nullableInt(f.ExpectedValue, f.HasExpectedValue), nullable(f.ValuePeriod), nullable(f.CalculationMethod))
	} else if err == nil {
		_, err = tx.Exec(`UPDATE initiative_benefit SET benefit_type_code=?,description=?,expected_value_vnd=?,value_period_code=?,calculation_method=? WHERE id=?`, f.BenefitType, f.BenefitDescription, nullableInt(f.ExpectedValue, f.HasExpectedValue), nullable(f.ValuePeriod), nullable(f.CalculationMethod), bid)
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectInitiative(w, r, id)
}

func (a *App) initiativeSubmitPost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := a.getInitiative(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if i.WorkflowStatusCode != "DRAFT" || !(i.SubmitterEmployeeNo == u.EmployeeNo || u.Roles["ADMIN"]) {
		http.Error(w, "forbidden", 403)
		return
	}
	var n int
	if err := a.DB.QueryRow(`SELECT COUNT(*) FROM initiative_benefit WHERE initiative_id=?`, id).Scan(&n); err != nil || n == 0 {
		http.Error(w, "Cần ít nhất một lợi ích dự kiến", 400)
		return
	}
	now := time.Now()
	due := addBusinessDays(now, 5).Format(time.RFC3339)
	tx, err := a.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE initiative SET submitted_at=?,manager_due_at=? WHERE id=?`, now.UTC().Format(time.RFC3339), due, id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err = a.changeStatus(tx, id, "DRAFT", "MANAGER_REVIEW", u.EmployeeNo, "Submitted"); err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectInitiative(w, r, id)
}

func (a *App) managerReviewPost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := a.getInitiative(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if i.WorkflowStatusCode != "MANAGER_REVIEW" || !(u.Roles["ADMIN"] || (u.Roles["MANAGER"] && i.DepartmentCode == u.DepartmentCode)) {
		http.Error(w, "forbidden", 403)
		return
	}
	decision := r.FormValue("decision")
	if decision != "APPROVE" && decision != "REJECT" {
		http.Error(w, "invalid decision", 400)
		return
	}
	comment := strings.TrimSpace(r.FormValue("comment"))
	reason := strings.TrimSpace(r.FormValue("reason"))
	to := "HRD_REVIEW"
	if decision == "REJECT" {
		to = "MANAGER_REJECTED"
	}
	tx, err := a.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO initiative_manager_review(initiative_id,reviewer_employee_no,decision_code,feasibility_comment,reason) VALUES(?,?,?,?,?)`, id, u.EmployeeNo, decision, nullable(comment), nullable(reason)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err = a.changeStatus(tx, id, "MANAGER_REVIEW", to, u.EmployeeNo, comment); err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectInitiative(w, r, id)
}

func (a *App) hrdForwardPost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !(u.Roles["HRD"] || u.Roles["ADMIN"]) {
		http.Error(w, "forbidden", 403)
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer tx.Rollback()
	if err = a.changeStatus(tx, id, "HRD_REVIEW", "COUNCIL_REVIEW", u.EmployeeNo, r.FormValue("note")); err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectInitiative(w, r, id)
}

func (a *App) councilAssessPost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !(u.Roles["COUNCIL"] || u.Roles["ADMIN"]) {
		http.Error(w, "forbidden", 403)
		return
	}
	assessed, has, err := parseVND(r.FormValue("assessed_value_vnd"))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	reward := strings.TrimSpace(r.FormValue("reward_level"))
	novelty := strings.TrimSpace(r.FormValue("novelty_comment"))
	feas := strings.TrimSpace(r.FormValue("feasibility_comment"))
	note := strings.TrimSpace(r.FormValue("note"))
	tx, err := a.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO initiative_council_assessment(initiative_id,assessor_employee_no,decision_code,novelty_comment,feasibility_comment,assessed_value_vnd,reward_level_code,council_note) VALUES(?,?,'APPROVE',?,?,?,?,?)`, id, u.EmployeeNo, nullable(novelty), nullable(feas), nullableInt(assessed, has), nullable(reward), nullable(note)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err = a.changeStatus(tx, id, "COUNCIL_REVIEW", "GM_APPROVAL", u.EmployeeNo, note); err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectInitiative(w, r, id)
}

func (a *App) gmApprovePost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !(u.Roles["GM"] || u.Roles["ADMIN"]) {
		http.Error(w, "forbidden", 403)
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer tx.Rollback()
	if err = a.changeStatus(tx, id, "GM_APPROVAL", "APPROVED", u.EmployeeNo, r.FormValue("note")); err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectInitiative(w, r, id)
}

func (a *App) verifyPost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !(u.Roles["HRD"] || u.Roles["ADMIN"]) {
		http.Error(w, "forbidden", 403)
		return
	}
	i, err := a.getInitiative(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if i.WorkflowStatusCode != "APPROVED" && i.WorkflowStatusCode != "VERIFICATION" {
		http.Error(w, "invalid state", 409)
		return
	}
	actual, has, err := parseVND(r.FormValue("actual_value_vnd"))
	if err != nil || !has {
		http.Error(w, "actual value required", 400)
		return
	}
	impl := r.FormValue("implementation_status")
	if impl == "" {
		http.Error(w, "implementation status required", 400)
		return
	}
	note := strings.TrimSpace(r.FormValue("note"))
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := a.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`UPDATE initiative SET implementation_status_code=?,actual_value_vnd=?,actual_value_verified=1,actual_value_verified_by=?,actual_value_verified_at=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, impl, actual, u.EmployeeNo, now, id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if i.WorkflowStatusCode == "APPROVED" {
		if err = a.changeStatus(tx, id, "APPROVED", "VERIFICATION", u.EmployeeNo, note); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
	} else {
		_, err = tx.Exec(`INSERT INTO initiative_status_history(initiative_id,from_status_code,to_status_code,changed_by,note) VALUES(?, 'VERIFICATION','VERIFICATION',?,?)`, id, u.EmployeeNo, nullable(note))
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectInitiative(w, r, id)
}

func (a *App) rewardPost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !(u.Roles["HRD"] || u.Roles["ADMIN"]) {
		http.Error(w, "forbidden", 403)
		return
	}
	i, err := a.getInitiative(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if i.WorkflowStatusCode != "VERIFICATION" || !i.ActualValueVerified {
		http.Error(w, "initiative must be verified first", 409)
		return
	}
	amount, has, err := parseVND(r.FormValue("approved_amount_vnd"))
	if err != nil || !has {
		http.Error(w, "approved amount required", 400)
		return
	}
	level := strings.TrimSpace(r.FormValue("approved_level"))
	if level == "" {
		http.Error(w, "reward level required", 400)
		return
	}
	note := strings.TrimSpace(r.FormValue("note"))
	now := time.Now().UTC().Format(time.RFC3339)
	var approvedBy string
	err = a.DB.QueryRow(`SELECT changed_by FROM initiative_status_history WHERE initiative_id=? AND from_status_code='GM_APPROVAL' AND to_status_code='APPROVED' ORDER BY id DESC LIMIT 1`, id).Scan(&approvedBy)
	if err != nil {
		http.Error(w, "GM approval history missing", 409)
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO initiative_reward(initiative_id,approved_level_code,approved_amount_vnd,approved_by_employee_no,approved_at,note) VALUES(?,?,?,?,?,?)`, id, level, amount, approvedBy, now, nullable(note)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err = a.changeStatus(tx, id, "VERIFICATION", "REWARDED", u.EmployeeNo, note); err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectInitiative(w, r, id)
}

func (a *App) closePost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !(u.Roles["HRD"] || u.Roles["ADMIN"]) {
		http.Error(w, "forbidden", 403)
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer tx.Rollback()
	if err = a.changeStatus(tx, id, "REWARDED", "CLOSED", u.EmployeeNo, r.FormValue("note")); err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirectInitiative(w, r, id)
}

func (a *App) attachmentPost(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := a.getInitiative(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !canView(u, i) {
		http.Error(w, "forbidden", 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "file too large or invalid form", 400)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", 400)
		return
	}
	defer file.Close()
	docType := r.FormValue("document_type")
	if docType == "" {
		docType = "OTHER"
	}
	token, _ := randomToken(12)
	dir := filepath.Join(a.UploadDir, i.Code)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	stored := token + filepath.Ext(safeFilename(hdr.Filename))
	path := filepath.Join(dir, stored)
	out, err := os.Create(path)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, h), file)
	cerr := out.Close()
	if err != nil || cerr != nil {
		_ = os.Remove(path)
		http.Error(w, "cannot save file", 500)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	storageKey := filepath.ToSlash(filepath.Join(i.Code, stored))
	_, err = a.DB.Exec(`INSERT INTO initiative_attachment(initiative_id,document_type_code,original_filename,storage_key,mime_type,size_bytes,sha256_hex,uploaded_by) VALUES(?,?,?,?,?,?,?,?)`, id, docType, safeFilename(hdr.Filename), storageKey, hdr.Header.Get("Content-Type"), n, sha, u.EmployeeNo)
	if err != nil {
		_ = os.Remove(path)
		http.Error(w, err.Error(), 500)
		return
	}
	redirectInitiative(w, r, id)
}

func (a *App) attachmentDownload(w http.ResponseWriter, r *http.Request, u *User) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var initiativeID int64
	var storage, original, mime string
	err = a.DB.QueryRow(`SELECT initiative_id,storage_key,original_filename,COALESCE(mime_type,'application/octet-stream') FROM initiative_attachment WHERE id=?`, id).Scan(&initiativeID, &storage, &original, &mime)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := a.getInitiative(initiativeID)
	if err != nil || !canView(u, i) {
		http.Error(w, "forbidden", 403)
		return
	}
	path := filepath.Join(a.UploadDir, filepath.FromSlash(storage))
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeFilename(original)))
	http.ServeFile(w, r, path)
}

func (a *App) masterGet(w http.ResponseWriter, r *http.Request, u *User) {
	depts, err := a.masterDepartments()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	emps, err := a.masterEmployees()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	users, err := a.masterUsers()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	roles, _ := a.options("rbac_role")
	groups := map[string][]Option{}
	for _, name := range []string{"md_problem_category", "md_benefit_type", "md_unit", "md_value_period", "md_implementation_status", "md_workflow_status", "md_document_type", "md_reward_level"} {
		groups[name], _ = a.options(name)
	}
	a.render(w, "master", PageData{User: u, Title: "Master data & RBAC", Departments: depts, Employees: emps, Users: users, Roles: roles, MasterGroups: groups})
}

func (a *App) departmentPost(w http.ResponseWriter, r *http.Request, u *User) {
	code := cleanCode(r.FormValue("code"))
	name := strings.TrimSpace(r.FormValue("name"))
	if code == "" || name == "" {
		http.Error(w, "code/name required", 400)
		return
	}
	_, err := a.DB.Exec(`INSERT INTO md_department(code,name,active) VALUES(?,?,1)`, code, name)
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	http.Redirect(w, r, "/admin/master", 303)
}
func (a *App) employeePost(w http.ResponseWriter, r *http.Request, u *User) {
	no := cleanCode(r.FormValue("employee_no"))
	name := strings.TrimSpace(r.FormValue("full_name"))
	dept := cleanCode(r.FormValue("department_code"))
	pos := strings.TrimSpace(r.FormValue("position_title"))
	if no == "" || name == "" || dept == "" {
		http.Error(w, "required fields missing", 400)
		return
	}
	_, err := a.DB.Exec(`INSERT INTO md_employee(employee_no,full_name,department_code,position_title,active) VALUES(?,?,?,?,1)`, no, name, dept, nullable(pos))
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	http.Redirect(w, r, "/admin/master", 303)
}
func (a *App) userPost(w http.ResponseWriter, r *http.Request, u *User) {
	username := strings.TrimSpace(r.FormValue("username"))
	emp := cleanCode(r.FormValue("employee_no"))
	hash, err := security.HashPassword(r.FormValue("password"))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	roles := r.Form["roles"]
	if username == "" || emp == "" || len(roles) == 0 {
		http.Error(w, "username/employee/roles required", 400)
		return
	}
	tx, err := a.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO app_user(username,password_hash,employee_no,active) VALUES(?,?,?,1)`, username, hash, emp)
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	id, _ := res.LastInsertId()
	for _, role := range roles {
		if _, err = tx.Exec(`INSERT INTO app_user_role(user_id,role_code) VALUES(?,?)`, id, role); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/admin/master", 303)
}

func (a *App) parseInt64Field(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(r.FormValue(name)), 10, 64)
}
