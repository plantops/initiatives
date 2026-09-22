PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS md_department (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1))
);

CREATE TABLE IF NOT EXISTS md_employee (
    employee_no TEXT PRIMARY KEY,
    full_name TEXT NOT NULL,
    department_code TEXT NOT NULL REFERENCES md_department(code),
    position_title TEXT,
    manager_employee_no TEXT REFERENCES md_employee(employee_no),
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1))
);

CREATE TABLE IF NOT EXISTS md_problem_category (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1))
);

CREATE TABLE IF NOT EXISTS md_benefit_type (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    monetizable INTEGER NOT NULL DEFAULT 0 CHECK(monetizable IN (0,1)),
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1))
);

CREATE TABLE IF NOT EXISTS md_unit (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    symbol TEXT,
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1))
);

CREATE TABLE IF NOT EXISTS md_value_period (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS md_implementation_status (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS md_workflow_status (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    sort_order INTEGER NOT NULL,
    terminal INTEGER NOT NULL DEFAULT 0 CHECK(terminal IN (0,1))
);

CREATE TABLE IF NOT EXISTS md_document_type (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1))
);

CREATE TABLE IF NOT EXISTS md_review_decision (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS md_reward_level (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    reward_min_vnd INTEGER,
    reward_max_vnd INTEGER,
    annual_value_min_vnd INTEGER,
    annual_value_max_vnd INTEGER,
    requires_implemented INTEGER NOT NULL DEFAULT 0 CHECK(requires_implemented IN (0,1)),
    rule_note TEXT,
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1))
);

CREATE TABLE IF NOT EXISTS rbac_role (
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS app_user (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    employee_no TEXT NOT NULL UNIQUE REFERENCES md_employee(employee_no),
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS app_user_role (
    user_id INTEGER NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    role_code TEXT NOT NULL REFERENCES rbac_role(code),
    PRIMARY KEY(user_id, role_code)
);

CREATE TABLE IF NOT EXISTS app_session (
    token_hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    csrf_token TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS initiative_sequence (
    year INTEGER PRIMARY KEY,
    value INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS initiative (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    initiative_code TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL CHECK(length(title) <= 240),
    submitter_employee_no TEXT NOT NULL REFERENCES md_employee(employee_no),
    department_code TEXT NOT NULL REFERENCES md_department(code),
    problem_category_code TEXT REFERENCES md_problem_category(code),
    problem_summary TEXT NOT NULL CHECK(length(problem_summary) <= 240),
    problem_detail TEXT NOT NULL,
    solution_summary TEXT NOT NULL CHECK(length(solution_summary) <= 240),
    solution_detail TEXT NOT NULL,
    implementation_status_code TEXT NOT NULL REFERENCES md_implementation_status(code),
    workflow_status_code TEXT NOT NULL REFERENCES md_workflow_status(code),
    expected_value_vnd INTEGER CHECK(expected_value_vnd IS NULL OR expected_value_vnd >= 0),
    actual_value_vnd INTEGER CHECK(actual_value_vnd IS NULL OR actual_value_vnd >= 0),
    value_period_code TEXT REFERENCES md_value_period(code),
    value_calculation_method TEXT,
    actual_value_verified INTEGER NOT NULL DEFAULT 0 CHECK(actual_value_verified IN (0,1)),
    actual_value_verified_by TEXT REFERENCES md_employee(employee_no),
    actual_value_verified_at TEXT,
    manager_due_at TEXT,
    submitted_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS initiative_participant (
    initiative_id INTEGER NOT NULL REFERENCES initiative(id) ON DELETE RESTRICT,
    employee_no TEXT NOT NULL REFERENCES md_employee(employee_no),
    role_code TEXT NOT NULL DEFAULT 'CO_AUTHOR',
    PRIMARY KEY(initiative_id, employee_no)
);

CREATE TABLE IF NOT EXISTS initiative_benefit (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    initiative_id INTEGER NOT NULL REFERENCES initiative(id) ON DELETE RESTRICT,
    benefit_type_code TEXT NOT NULL REFERENCES md_benefit_type(code),
    description TEXT NOT NULL,
    baseline_value REAL,
    target_value REAL,
    actual_value REAL,
    unit_code TEXT REFERENCES md_unit(code),
    expected_value_vnd INTEGER CHECK(expected_value_vnd IS NULL OR expected_value_vnd >= 0),
    actual_value_vnd INTEGER CHECK(actual_value_vnd IS NULL OR actual_value_vnd >= 0),
    value_period_code TEXT REFERENCES md_value_period(code),
    calculation_method TEXT,
    verification_note TEXT
);

CREATE TABLE IF NOT EXISTS initiative_attachment (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    initiative_id INTEGER NOT NULL REFERENCES initiative(id) ON DELETE RESTRICT,
    document_type_code TEXT NOT NULL REFERENCES md_document_type(code),
    original_filename TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    mime_type TEXT,
    size_bytes INTEGER CHECK(size_bytes IS NULL OR size_bytes >= 0),
    sha256_hex TEXT,
    uploaded_by TEXT NOT NULL REFERENCES md_employee(employee_no),
    uploaded_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS initiative_manager_review (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    initiative_id INTEGER NOT NULL REFERENCES initiative(id) ON DELETE RESTRICT,
    reviewer_employee_no TEXT NOT NULL REFERENCES md_employee(employee_no),
    decision_code TEXT NOT NULL REFERENCES md_review_decision(code),
    feasibility_comment TEXT,
    reason TEXT,
    reviewed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS initiative_council_assessment (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    initiative_id INTEGER NOT NULL REFERENCES initiative(id) ON DELETE RESTRICT,
    assessor_employee_no TEXT NOT NULL REFERENCES md_employee(employee_no),
    decision_code TEXT NOT NULL REFERENCES md_review_decision(code),
    novelty_comment TEXT,
    feasibility_comment TEXT,
    assessed_value_vnd INTEGER CHECK(assessed_value_vnd IS NULL OR assessed_value_vnd >= 0),
    reward_level_code TEXT REFERENCES md_reward_level(code),
    council_note TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS initiative_reward (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    initiative_id INTEGER NOT NULL REFERENCES initiative(id) ON DELETE RESTRICT,
    recommended_level_code TEXT REFERENCES md_reward_level(code),
    approved_level_code TEXT REFERENCES md_reward_level(code),
    approved_amount_vnd INTEGER CHECK(approved_amount_vnd IS NULL OR approved_amount_vnd >= 0),
    approved_by_employee_no TEXT REFERENCES md_employee(employee_no),
    approved_at TEXT,
    paid_at TEXT,
    note TEXT
);

CREATE TABLE IF NOT EXISTS initiative_status_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    initiative_id INTEGER NOT NULL REFERENCES initiative(id) ON DELETE RESTRICT,
    from_status_code TEXT REFERENCES md_workflow_status(code),
    to_status_code TEXT NOT NULL REFERENCES md_workflow_status(code),
    changed_by TEXT NOT NULL REFERENCES md_employee(employee_no),
    changed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    note TEXT
);

CREATE INDEX IF NOT EXISTS idx_initiative_department ON initiative(department_code);
CREATE INDEX IF NOT EXISTS idx_initiative_submitter ON initiative(submitter_employee_no);
CREATE INDEX IF NOT EXISTS idx_initiative_workflow ON initiative(workflow_status_code);
CREATE INDEX IF NOT EXISTS idx_initiative_impl ON initiative(implementation_status_code);
CREATE INDEX IF NOT EXISTS idx_status_history_initiative ON initiative_status_history(initiative_id, changed_at);

INSERT OR IGNORE INTO rbac_role(code, name) VALUES
('EMPLOYEE', 'Nhân viên'),
('MANAGER', 'Trưởng phòng / Trưởng khối'),
('HRD', 'Phòng Nhân sự'),
('COUNCIL', 'Hội đồng đánh giá'),
('GM', 'Tổng Giám đốc'),
('ADMIN', 'Quản trị hệ thống');

INSERT OR IGNORE INTO md_implementation_status(code, name, sort_order) VALUES
('IDEA', 'Chỉ là ý tưởng / chưa triển khai', 10),
('PILOT', 'Đã thử nghiệm', 20),
('IMPLEMENTED', 'Đã áp dụng', 30);

INSERT OR IGNORE INTO md_workflow_status(code, name, sort_order, terminal) VALUES
('DRAFT', 'Nháp', 10, 0),
('MANAGER_REVIEW', 'Quản lý xem xét', 20, 0),
('MANAGER_REJECTED', 'Quản lý không đồng ý', 30, 1),
('HRD_REVIEW', 'HRD tiếp nhận', 40, 0),
('COUNCIL_REVIEW', 'Hội đồng đánh giá', 50, 0),
('GM_APPROVAL', 'Tổng Giám đốc phê duyệt', 60, 0),
('APPROVED', 'Đã phê duyệt', 70, 0),
('VERIFICATION', 'Xác minh kết quả', 80, 0),
('REWARDED', 'Đã khen thưởng', 90, 0),
('CLOSED', 'Đóng hồ sơ', 100, 1);

INSERT OR IGNORE INTO md_review_decision(code, name) VALUES
('APPROVE', 'Đồng ý'),
('REJECT', 'Không đồng ý'),
('NEED_INFO', 'Yêu cầu bổ sung');

INSERT OR IGNORE INTO md_value_period(code, name) VALUES
('ONE_TIME', 'Một lần'),
('MONTHLY', 'Theo tháng'),
('ANNUAL', 'Theo năm');

INSERT OR IGNORE INTO md_benefit_type(code, name, monetizable) VALUES
('QUALITY', 'Chất lượng', 0),
('COST_SAVING', 'Tiết kiệm chi phí', 1),
('COST_AVOIDANCE', 'Tránh chi phí', 1),
('TIME', 'Thời gian', 0),
('SAFETY', 'An toàn', 0),
('ENVIRONMENT', 'Môi trường', 0),
('ENERGY', 'Năng lượng', 1),
('PRODUCTIVITY', 'Năng suất', 1),
('REVENUE', 'Tăng doanh thu', 1),
('OTHER', 'Khác', 0);

INSERT OR IGNORE INTO md_problem_category(code, name) VALUES
('PROCESS', 'Quy trình'),
('QUALITY', 'Chất lượng'),
('COST', 'Chi phí'),
('TIME', 'Thời gian'),
('SAFETY', 'An toàn'),
('ENVIRONMENT', 'Môi trường'),
('ENERGY', 'Năng lượng'),
('PRODUCTIVITY', 'Năng suất'),
('DIGITAL', 'Số hóa / dữ liệu'),
('OTHER', 'Khác');

INSERT OR IGNORE INTO md_unit(code, name, symbol) VALUES
('VND', 'Vietnamese Dong', 'VND'),
('PERCENT', 'Percent', '%'),
('HOUR', 'Hour', 'h'),
('DAY', 'Day', 'day'),
('KWH', 'Kilowatt-hour', 'kWh'),
('TON', 'Tonne', 't'),
('KG', 'Kilogram', 'kg'),
('COUNT', 'Count', '');

INSERT OR IGNORE INTO md_document_type(code, name) VALUES
('PROBLEM_EVIDENCE', 'Bằng chứng hiện trạng'),
('CALCULATION', 'Tính toán lợi ích'),
('DRAWING', 'Bản vẽ / sơ đồ'),
('PHOTO', 'Hình ảnh'),
('SOP', 'Quy trình / SOP'),
('QUOTATION', 'Báo giá'),
('OTHER', 'Khác');

INSERT OR IGNORE INTO md_reward_level(
    code, name, reward_min_vnd, reward_max_vnd,
    annual_value_min_vnd, annual_value_max_vnd,
    requires_implemented, rule_note
) VALUES
('PROPOSAL_INCENTIVE', 'Thưởng khuyến khích giai đoạn đề xuất', 200000, 200000, NULL, NULL, 0,
 'Hội đồng đánh giá khả thi và có thể triển khai; chưa áp dụng thực tế.'),
('CERT_1', 'Khen ngợi bằng văn bản 1 lần', 500000, 500000, 0, 19999999, 1,
 'Đã triển khai; giá trị kinh tế < 20 triệu VND/năm.'),
('CERT_2', 'Khen ngợi bằng văn bản 2 lần', 1000000, 1000000, 20000000, 49999999, 1,
 'Đã triển khai; giá trị kinh tế từ 20 đến < 50 triệu VND/năm.'),
('MERIT_1', 'Ghi công nhỏ 1 lần', 1500000, 1500000, 50000000, 99999999, 1,
 'Đã triển khai; giá trị kinh tế từ 50 đến < 100 triệu VND/năm.'),
('MERIT_2', 'Ghi công nhỏ 2 lần', 3000000, 3000000, 100000000, 199999999, 1,
 'Đã triển khai; giá trị kinh tế từ 100 đến < 200 triệu VND/năm.'),
('MAJOR_MERIT_1', 'Ghi công lớn 1 lần', 4500000, 4500000, 200000000, 299999999, 1,
 'Đã triển khai; giá trị kinh tế từ 200 đến < 300 triệu VND/năm.'),
('MAJOR_MERIT_2', 'Ghi công lớn 2 lần', 9000000, 9000000, 300000000, NULL, 1,
 'Đã triển khai; >=300 triệu VND/năm hoặc ảnh hưởng lâu dài/phạm vi rộng.'),
('SPECIAL', 'Giải thưởng đặc biệt', 10000000, 50000000, 300000001, NULL, 1,
 'Đã triển khai; >300 triệu VND/năm. Điều kiện chồng lấn MAJOR_MERIT_2; HRD quyết định.');
