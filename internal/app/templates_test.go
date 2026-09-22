package app

import (
	"database/sql"
	"html/template"
	"io"
	"testing"
)

func testTemplates(t *testing.T) *template.Template {
	t.Helper()
	funcs := template.FuncMap{
		"hasRole":   func(u *User, role string) bool { return u != nil && u.Roles[role] },
		"money":     formatMoney,
		"nullMoney": func(v sql.NullInt64) string { return "" },
		"nullText":  func(v sql.NullString) string { return "" },
		"boolText":  func(v bool) string { return "" },
	}
	tt, err := template.New("root").Funcs(funcs).ParseFS(assets, "web/templates/*.html")
	if err != nil {
		t.Fatal(err)
	}
	return tt
}

func TestTemplatesParse(t *testing.T) {
	tt := testTemplates(t)
	for _, name := range []string{"login", "setup", "dashboard", "initiative_form", "initiative_edit", "initiative_detail", "master"} {
		if tt.Lookup(name) == nil {
			t.Fatalf("missing template %s", name)
		}
	}
}

func TestTemplatesExecuteSmoke(t *testing.T) {
	tt := testTemplates(t)
	u := &User{EmployeeNo: "E1", FullName: "Test User", DepartmentCode: "D1", Roles: map[string]bool{"ADMIN": true}, CSRFToken: "x"}
	i := &Initiative{ID: 1, Code: "SI-2026-00001", Title: "Test", ProblemSummary: "P", ProblemDetail: "P", SolutionSummary: "S", SolutionDetail: "S", ImplementationStatusCode: "IDEA", WorkflowStatusCode: "DRAFT"}
	base := PageData{
		User: u, Initiative: i, Benefits: []Benefit{{BenefitTypeCode: "TIME", Description: "Test"}},
		ProblemCategories:      []Option{{Code: "PROCESS", Name: "Process"}},
		BenefitTypes:           []Option{{Code: "TIME", Name: "Time"}},
		ValuePeriods:           []Option{{Code: "ANNUAL", Name: "Annual"}},
		ImplementationStatuses: []Option{{Code: "IDEA", Name: "Idea"}},
		RewardLevels:           []Option{{Code: "CERT_1", Name: "Cert"}},
		DocumentTypes:          []Option{{Code: "OTHER", Name: "Other"}},
		Roles:                  []Option{{Code: "ADMIN", Name: "Admin"}},
		Departments:            []Department{{Code: "D1", Name: "Dept", Active: true}},
		Employees:              []Employee{{EmployeeNo: "E1", FullName: "Test User", DepartmentCode: "D1", DepartmentName: "Dept", Active: true}},
		MasterGroups:           map[string][]Option{"md_unit": {{Code: "VND", Name: "VND"}}},
	}
	for _, name := range []string{"login", "setup", "dashboard", "initiative_form", "initiative_edit", "initiative_detail", "master"} {
		d := base
		d.Title = name
		if err := tt.ExecuteTemplate(io.Discard, name, d); err != nil {
			t.Fatalf("execute %s: %v", name, err)
		}
	}
}
