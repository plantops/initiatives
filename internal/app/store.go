package app

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (a *App) options(table string) ([]Option, error) {
	allowed := map[string]string{
		"md_problem_category":      "SELECT code,name FROM md_problem_category WHERE active=1 ORDER BY name",
		"md_benefit_type":          "SELECT code,name FROM md_benefit_type WHERE active=1 ORDER BY name",
		"md_unit":                  "SELECT code,name FROM md_unit WHERE active=1 ORDER BY name",
		"md_value_period":          "SELECT code,name FROM md_value_period ORDER BY code",
		"md_implementation_status": "SELECT code,name FROM md_implementation_status ORDER BY sort_order",
		"md_reward_level":          "SELECT code,name FROM md_reward_level WHERE active=1 ORDER BY COALESCE(reward_min_vnd,0), code",
		"md_document_type":         "SELECT code,name FROM md_document_type WHERE active=1 ORDER BY name",
		"rbac_role":                "SELECT code,name FROM rbac_role ORDER BY code",
		"md_workflow_status":       "SELECT code,name FROM md_workflow_status ORDER BY sort_order",
		"md_review_decision":       "SELECT code,name FROM md_review_decision ORDER BY code",
	}
	q, ok := allowed[table]
	if !ok {
		return nil, fmt.Errorf("unsupported master table")
	}
	rows, err := a.DB.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Option
	for rows.Next() {
		var x Option
		if err := rows.Scan(&x.Code, &x.Name); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (a *App) nextInitiativeCode(tx *sql.Tx, year int) (string, error) {
	var n int
	err := tx.QueryRow(`
        INSERT INTO initiative_sequence(year,value) VALUES(?,1)
        ON CONFLICT(year) DO UPDATE SET value = value + 1
        RETURNING value
    `, year).Scan(&n)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("SI-%04d-%05d", year, n), nil
}

func (a *App) listInitiatives(u *User, search string) ([]Initiative, error) {
	base := `
        SELECT i.id,i.initiative_code,i.title,i.submitter_employee_no,e.full_name,
               i.department_code,d.name,
               COALESCE(i.problem_category_code,''),COALESCE(pc.name,''),
               i.problem_summary,i.problem_detail,i.solution_summary,i.solution_detail,
               i.implementation_status_code,ims.name,i.workflow_status_code,ws.name,
               i.expected_value_vnd,i.actual_value_vnd,i.value_period_code,i.value_calculation_method,
               i.actual_value_verified,i.actual_value_verified_by,i.actual_value_verified_at,
               i.manager_due_at,i.submitted_at,i.created_at,i.updated_at
        FROM initiative i
        JOIN md_employee e ON e.employee_no=i.submitter_employee_no
        JOIN md_department d ON d.code=i.department_code
        LEFT JOIN md_problem_category pc ON pc.code=i.problem_category_code
        JOIN md_implementation_status ims ON ims.code=i.implementation_status_code
        JOIN md_workflow_status ws ON ws.code=i.workflow_status_code
    `
	var where []string
	var args []any
	if !(u.Roles["HRD"] || u.Roles["COUNCIL"] || u.Roles["GM"] || u.Roles["ADMIN"]) {
		if u.Roles["MANAGER"] {
			where = append(where, `(i.submitter_employee_no=? OR i.department_code=?)`)
			args = append(args, u.EmployeeNo, u.DepartmentCode)
		} else {
			where = append(where, `i.submitter_employee_no=?`)
			args = append(args, u.EmployeeNo)
		}
	}
	search = strings.TrimSpace(search)
	if search != "" {
		where = append(where, `(i.initiative_code LIKE ? OR i.title LIKE ? OR e.full_name LIKE ?)`)
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}
	if len(where) > 0 {
		base += " WHERE " + strings.Join(where, " AND ")
	}
	base += " ORDER BY i.id DESC LIMIT 300"

	rows, err := a.DB.Query(base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Initiative
	for rows.Next() {
		i, err := scanInitiative(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

type scanner interface{ Scan(...any) error }

func scanInitiative(s scanner) (Initiative, error) {
	var i Initiative
	var pcCode, pcName string
	var verified int
	err := s.Scan(
		&i.ID, &i.Code, &i.Title, &i.SubmitterEmployeeNo, &i.SubmitterName,
		&i.DepartmentCode, &i.DepartmentName,
		&pcCode, &pcName,
		&i.ProblemSummary, &i.ProblemDetail, &i.SolutionSummary, &i.SolutionDetail,
		&i.ImplementationStatusCode, &i.ImplementationStatusName, &i.WorkflowStatusCode, &i.WorkflowStatusName,
		&i.ExpectedValueVND, &i.ActualValueVND, &i.ValuePeriodCode, &i.ValueCalculationMethod,
		&verified, &i.ActualValueVerifiedBy, &i.ActualValueVerifiedAt,
		&i.ManagerDueAt, &i.SubmittedAt, &i.CreatedAt, &i.UpdatedAt,
	)
	i.ProblemCategoryCode = pcCode
	i.ProblemCategoryName = pcName
	i.ActualValueVerified = verified == 1
	return i, err
}

func (a *App) getInitiative(id int64) (Initiative, error) {
	row := a.DB.QueryRow(`
        SELECT i.id,i.initiative_code,i.title,i.submitter_employee_no,e.full_name,
               i.department_code,d.name,
               COALESCE(i.problem_category_code,''),COALESCE(pc.name,''),
               i.problem_summary,i.problem_detail,i.solution_summary,i.solution_detail,
               i.implementation_status_code,ims.name,i.workflow_status_code,ws.name,
               i.expected_value_vnd,i.actual_value_vnd,i.value_period_code,i.value_calculation_method,
               i.actual_value_verified,i.actual_value_verified_by,i.actual_value_verified_at,
               i.manager_due_at,i.submitted_at,i.created_at,i.updated_at
        FROM initiative i
        JOIN md_employee e ON e.employee_no=i.submitter_employee_no
        JOIN md_department d ON d.code=i.department_code
        LEFT JOIN md_problem_category pc ON pc.code=i.problem_category_code
        JOIN md_implementation_status ims ON ims.code=i.implementation_status_code
        JOIN md_workflow_status ws ON ws.code=i.workflow_status_code
        WHERE i.id=?
    `, id)
	return scanInitiative(row)
}

func canView(u *User, i Initiative) bool {
	if u.Roles["HRD"] || u.Roles["COUNCIL"] || u.Roles["GM"] || u.Roles["ADMIN"] {
		return true
	}
	if i.SubmitterEmployeeNo == u.EmployeeNo {
		return true
	}
	return u.Roles["MANAGER"] && i.DepartmentCode == u.DepartmentCode
}

func (a *App) loadBenefits(id int64) ([]Benefit, error) {
	rows, err := a.DB.Query(`
        SELECT b.id,b.benefit_type_code,bt.name,b.description,b.baseline_value,b.target_value,b.actual_value,
               b.unit_code,b.expected_value_vnd,b.actual_value_vnd,b.value_period_code,b.calculation_method,b.verification_note
        FROM initiative_benefit b JOIN md_benefit_type bt ON bt.code=b.benefit_type_code
        WHERE b.initiative_id=? ORDER BY b.id
    `, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Benefit
	for rows.Next() {
		var b Benefit
		if err := rows.Scan(&b.ID, &b.BenefitTypeCode, &b.BenefitTypeName, &b.Description, &b.BaselineValue, &b.TargetValue, &b.ActualValue,
			&b.UnitCode, &b.ExpectedValueVND, &b.ActualValueVND, &b.ValuePeriodCode, &b.CalculationMethod, &b.VerificationNote); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (a *App) loadAttachments(id int64) ([]Attachment, error) {
	rows, err := a.DB.Query(`
        SELECT a.id,a.document_type_code,dt.name,a.original_filename,a.storage_key,a.mime_type,a.size_bytes,a.sha256_hex,a.uploaded_by,e.full_name,a.uploaded_at
        FROM initiative_attachment a
        JOIN md_document_type dt ON dt.code=a.document_type_code
        JOIN md_employee e ON e.employee_no=a.uploaded_by
        WHERE a.initiative_id=? ORDER BY a.id DESC
    `, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attachment
	for rows.Next() {
		var x Attachment
		if err := rows.Scan(&x.ID, &x.DocumentTypeCode, &x.DocumentTypeName, &x.OriginalFilename, &x.StorageKey, &x.MimeType, &x.SizeBytes, &x.SHA256Hex, &x.UploadedBy, &x.UploadedByName, &x.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (a *App) loadHistory(id int64) ([]History, error) {
	rows, err := a.DB.Query(`
        SELECT h.id,h.from_status_code,h.to_status_code,ws.name,h.changed_by,e.full_name,h.changed_at,h.note
        FROM initiative_status_history h
        JOIN md_workflow_status ws ON ws.code=h.to_status_code
        JOIN md_employee e ON e.employee_no=h.changed_by
        WHERE h.initiative_id=? ORDER BY h.id DESC
    `, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []History
	for rows.Next() {
		var h History
		if err := rows.Scan(&h.ID, &h.FromStatus, &h.ToStatus, &h.ToName, &h.ChangedBy, &h.ChangedName, &h.ChangedAt, &h.Note); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (a *App) loadManagerReviews(id int64) ([]ManagerReview, error) {
	rows, err := a.DB.Query(`SELECT e.full_name,r.decision_code,d.name,r.feasibility_comment,r.reason,r.reviewed_at
        FROM initiative_manager_review r JOIN md_employee e ON e.employee_no=r.reviewer_employee_no
        JOIN md_review_decision d ON d.code=r.decision_code WHERE r.initiative_id=? ORDER BY r.id DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ManagerReview
	for rows.Next() {
		var x ManagerReview
		if err := rows.Scan(&x.ReviewerName, &x.DecisionCode, &x.DecisionName, &x.Comment, &x.Reason, &x.ReviewedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (a *App) loadCouncilAssessments(id int64) ([]CouncilAssessment, error) {
	rows, err := a.DB.Query(`SELECT e.full_name,c.decision_code,d.name,c.novelty_comment,c.feasibility_comment,c.assessed_value_vnd,
        c.reward_level_code,rl.name,c.council_note,c.created_at
        FROM initiative_council_assessment c JOIN md_employee e ON e.employee_no=c.assessor_employee_no
        JOIN md_review_decision d ON d.code=c.decision_code LEFT JOIN md_reward_level rl ON rl.code=c.reward_level_code
        WHERE c.initiative_id=? ORDER BY c.id DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CouncilAssessment
	for rows.Next() {
		var x CouncilAssessment
		if err := rows.Scan(&x.AssessorName, &x.DecisionCode, &x.DecisionName, &x.NoveltyComment, &x.FeasibilityComment, &x.AssessedValueVND, &x.RewardLevelCode, &x.RewardLevelName, &x.CouncilNote, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (a *App) loadReward(id int64) (*Reward, error) {
	row := a.DB.QueryRow(`SELECT r.approved_level_code,rl.name,r.approved_amount_vnd,e.full_name,r.approved_at,r.paid_at,r.note
        FROM initiative_reward r LEFT JOIN md_reward_level rl ON rl.code=r.approved_level_code
        LEFT JOIN md_employee e ON e.employee_no=r.approved_by_employee_no
        WHERE r.initiative_id=? ORDER BY r.id DESC LIMIT 1`, id)
	var x Reward
	if err := row.Scan(&x.ApprovedLevelCode, &x.ApprovedLevelName, &x.ApprovedAmountVND, &x.ApprovedByName, &x.ApprovedAt, &x.PaidAt, &x.Note); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &x, nil
}

func (a *App) changeStatus(tx *sql.Tx, id int64, from, to, employeeNo, note string) error {
	res, err := tx.Exec(`UPDATE initiative SET workflow_status_code=?,updated_at=CURRENT_TIMESTAMP WHERE id=? AND workflow_status_code=?`, to, id, from)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("status changed by another operation")
	}
	_, err = tx.Exec(`INSERT INTO initiative_status_history(initiative_id,from_status_code,to_status_code,changed_by,note) VALUES(?,?,?,?,?)`, id, from, to, employeeNo, strings.TrimSpace(note))
	return err
}

func (a *App) masterDepartments() ([]Department, error) {
	rows, err := a.DB.Query(`SELECT code,name,active FROM md_department ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Department
	for rows.Next() {
		var x Department
		var active int
		if err := rows.Scan(&x.Code, &x.Name, &active); err != nil {
			return nil, err
		}
		x.Active = active == 1
		out = append(out, x)
	}
	return out, rows.Err()
}
func (a *App) masterEmployees() ([]Employee, error) {
	rows, err := a.DB.Query(`SELECT e.employee_no,e.full_name,e.department_code,d.name,e.position_title,e.active FROM md_employee e JOIN md_department d ON d.code=e.department_code ORDER BY e.full_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Employee
	for rows.Next() {
		var x Employee
		var active int
		if err := rows.Scan(&x.EmployeeNo, &x.FullName, &x.DepartmentCode, &x.DepartmentName, &x.PositionTitle, &active); err != nil {
			return nil, err
		}
		x.Active = active == 1
		out = append(out, x)
	}
	return out, rows.Err()
}
func (a *App) masterUsers() ([]AdminUser, error) {
	rows, err := a.DB.Query(`SELECT u.id,u.username,u.employee_no,e.full_name,u.active,COALESCE(GROUP_CONCAT(ur.role_code, ', '),'')
        FROM app_user u JOIN md_employee e ON e.employee_no=u.employee_no LEFT JOIN app_user_role ur ON ur.user_id=u.id
        GROUP BY u.id,u.username,u.employee_no,e.full_name,u.active ORDER BY u.username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminUser
	for rows.Next() {
		var x AdminUser
		var active int
		if err := rows.Scan(&x.ID, &x.Username, &x.EmployeeNo, &x.FullName, &active, &x.Roles); err != nil {
			return nil, err
		}
		x.Active = active == 1
		out = append(out, x)
	}
	return out, rows.Err()
}

func (a *App) createInitiative(u *User, form InitiativeForm) (int64, error) {
	tx, err := a.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	code, err := a.nextInitiativeCode(tx, time.Now().Year())
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(`INSERT INTO initiative(initiative_code,title,submitter_employee_no,department_code,problem_category_code,problem_summary,problem_detail,solution_summary,solution_detail,implementation_status_code,workflow_status_code,expected_value_vnd,value_period_code,value_calculation_method)
        VALUES(?,?,?,?,?,?,?,?,?,?,'DRAFT',?,?,?)`, code, form.Title, u.EmployeeNo, u.DepartmentCode, nullable(form.ProblemCategory), form.ProblemSummary, form.ProblemDetail, form.SolutionSummary, form.SolutionDetail, form.ImplementationStatus, nullableInt(form.ExpectedValue, form.HasExpectedValue), nullable(form.ValuePeriod), nullable(form.CalculationMethod))
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(`INSERT INTO initiative_benefit(initiative_id,benefit_type_code,description,expected_value_vnd,value_period_code,calculation_method) VALUES(?,?,?,?,?,?)`, id, form.BenefitType, form.BenefitDescription, nullableInt(form.ExpectedValue, form.HasExpectedValue), nullable(form.ValuePeriod), nullable(form.CalculationMethod)); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(`INSERT INTO initiative_status_history(initiative_id,to_status_code,changed_by,note) VALUES(?,'DRAFT',?,'Created')`, id, u.EmployeeNo); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

type InitiativeForm struct {
	Title, ProblemCategory, ProblemSummary, ProblemDetail, SolutionSummary, SolutionDetail, ImplementationStatus string
	BenefitType, BenefitDescription, ValuePeriod, CalculationMethod                                              string
	ExpectedValue                                                                                                int64
	HasExpectedValue                                                                                             bool
}

func nullable(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.TrimSpace(s)
}
func nullableInt(v int64, valid bool) any {
	if !valid {
		return nil
	}
	return v
}
