package service

// EvaluationExportColumn 描述评估导出的一列（#229 列描述单点）。
//
// 该 spec 是评估导出的唯一列序真值：repository 的 SELECT 列序与 position Scan、
// service 的 CSV 表头与取值均由此派生——任何列增删/重排都在此处单点完成。
// 一旦顺序漂移，既有 TestExportEvaluations 与评估导出 shape-lock 门禁立即变红，
// 从而把「SQL 返回序」与「表头序」永久绑定、无法各自漂移。
type EvaluationExportColumn struct {
	// Header 是 CSV 表头（首行中文列名）。
	Header string
	// Select 是 repository 的 SELECT 表达式（含别名），顺序即 SQL 返回列序。
	Select string
	// Scan 返回该列在 rows.Scan 中的位置落位（dest）与扫描完成后的提交动作（commit）。
	// dest 按 spec 顺序收集后一次传给 rows.Scan；Scan 成功后依序执行 commit 落位
	// （可空列如系数/置信区间/报告路径在此处理 NULL→零值/空串）。
	Scan func(r *EvaluationExportRow) (dest any, commit func())
	// Value 将该列转为导出单元格值，保持既有中间形状逐字一致：
	// ID/数值等原始字段直接透传（测试锁定 ID 为 int64），bool/指针/时间列走
	// yesNo / coeff / nullableFloat / formatISO 格式化。
	Value func(r EvaluationExportRow) any
}

// EvaluationExportColumns 评估导出 31 列（顺序即唯一列序真值，勿重排）。
var EvaluationExportColumns = []EvaluationExportColumn{
	{Header: "ID", Select: "e.id",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.ID, nil },
		Value: func(r EvaluationExportRow) any { return r.ID }},
	{Header: "账号", Select: "COALESCE(u.account, '') AS account",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.Account, nil },
		Value: func(r EvaluationExportRow) any { return r.Account }},
	{Header: "昵称", Select: "COALESCE(u.username, '') AS username",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.Username, nil },
		Value: func(r EvaluationExportRow) any { return r.Username }},
	{Header: "品牌", Select: "e.brand",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.Brand, nil },
		Value: func(r EvaluationExportRow) any { return r.Brand }},
	{Header: "车型", Select: "e.vehicle_type",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.VehicleType, nil },
		Value: func(r EvaluationExportRow) any { return r.VehicleType }},
	{Header: "系列", Select: "e.series",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.Series, nil },
		Value: func(r EvaluationExportRow) any { return r.Series }},
	{Header: "吨位", Select: "e.tonnage",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.Tonnage, nil },
		Value: func(r EvaluationExportRow) any { return r.Tonnage }},
	{Header: "配置", Select: "e.config_type",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.ConfigType, nil },
		Value: func(r EvaluationExportRow) any { return r.ConfigType }},
	{Header: "门架类型", Select: "e.mast_type",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.MastType, nil },
		Value: func(r EvaluationExportRow) any { return r.MastType }},
	{Header: "门架高度mm", Select: "e.mast_height_mm",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.MastHeightMM, nil },
		Value: func(r EvaluationExportRow) any { return r.MastHeightMM }},
	{Header: "出厂年份", Select: "e.factory_year",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.FactoryYear, nil },
		Value: func(r EvaluationExportRow) any { return r.FactoryYear }},
	{Header: "销售年份", Select: "e.sale_year",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.SaleYear, nil },
		Value: func(r EvaluationExportRow) any { return r.SaleYear }},
	{Header: "工时", Select: "e.usage_hours",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.UsageHours, nil },
		Value: func(r EvaluationExportRow) any { return r.UsageHours }},
	{Header: "原厂漆", Select: "e.original_paint",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.OriginalPaint, nil },
		Value: func(r EvaluationExportRow) any { return yesNo(r.OriginalPaint) }},
	{Header: "省份", Select: "e.province",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.Province, nil },
		Value: func(r EvaluationExportRow) any { return r.Province }},
	{Header: "城市", Select: "e.city",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.City, nil },
		Value: func(r EvaluationExportRow) any { return r.City }},
	{Header: "有牌照", Select: "e.has_license_plate",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.HasLicensePlate, nil },
		Value: func(r EvaluationExportRow) any { return yesNo(r.HasLicensePlate) }},
	{Header: "有登记证", Select: "e.has_registration_certificate AS has_registration_cert",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.HasRegistrationCert, nil },
		Value: func(r EvaluationExportRow) any { return yesNo(r.HasRegistrationCert) }},
	{Header: "有维保记录", Select: "e.has_maintenance_records",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.HasMaintenanceRecords, nil },
		Value: func(r EvaluationExportRow) any { return yesNo(r.HasMaintenanceRecords) }},
	{Header: "车况", Select: "e.condition_rating",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.ConditionRating, nil },
		Value: func(r EvaluationExportRow) any { return r.ConditionRating }},
	{Header: "原价", Select: "e.original_price",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.OriginalPrice, nil },
		Value: func(r EvaluationExportRow) any { return r.OriginalPrice }},
	{Header: "Kt", Select: "e.k_time",
		Scan: func(r *EvaluationExportRow) (any, func()) {
			var p *float64
			return &p, func() { r.KTime = p }
		},
		Value: func(r EvaluationExportRow) any { return coeff(r.KTime) }},
	{Header: "Kh", Select: "e.k_hours",
		Scan: func(r *EvaluationExportRow) (any, func()) {
			var p *float64
			return &p, func() { r.KHours = p }
		},
		Value: func(r EvaluationExportRow) any { return coeff(r.KHours) }},
	{Header: "Kb", Select: "e.k_brand",
		Scan: func(r *EvaluationExportRow) (any, func()) {
			var p *float64
			return &p, func() { r.KBrand = p }
		},
		Value: func(r EvaluationExportRow) any { return coeff(r.KBrand) }},
	{Header: "Kc", Select: "e.k_condition",
		Scan: func(r *EvaluationExportRow) (any, func()) {
			var p *float64
			return &p, func() { r.KCondition = p }
		},
		Value: func(r EvaluationExportRow) any { return coeff(r.KCondition) }},
	{Header: "Km", Select: "e.k_market",
		Scan: func(r *EvaluationExportRow) (any, func()) {
			var p *float64
			return &p, func() { r.KMarket = p }
		},
		Value: func(r EvaluationExportRow) any { return coeff(r.KMarket) }},
	{Header: "评估值", Select: "e.estimated_value",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.EstimatedValue, nil },
		Value: func(r EvaluationExportRow) any { return r.EstimatedValue }},
	{Header: "置信下限", Select: "e.confidence_low",
		Scan: func(r *EvaluationExportRow) (any, func()) {
			var p *float64
			return &p, func() { r.ConfidenceLow = p }
		},
		Value: func(r EvaluationExportRow) any { return nullableFloat(r.ConfidenceLow) }},
	{Header: "置信上限", Select: "e.confidence_high",
		Scan: func(r *EvaluationExportRow) (any, func()) {
			var p *float64
			return &p, func() { r.ConfidenceHigh = p }
		},
		Value: func(r EvaluationExportRow) any { return nullableFloat(r.ConfidenceHigh) }},
	{Header: "报告PDF", Select: "e.report_pdf_path",
		Scan: func(r *EvaluationExportRow) (any, func()) {
			var p *string
			return &p, func() {
				if p != nil {
					r.ReportPDFPath = *p
				}
			}
		},
		Value: func(r EvaluationExportRow) any { return r.ReportPDFPath }},
	{Header: "创建时间", Select: "e.created_at",
		Scan:  func(r *EvaluationExportRow) (any, func()) { return &r.CreatedAt, nil },
		Value: func(r EvaluationExportRow) any { return formatISO(r.CreatedAt) }},
}

// ScanEvalExportDestinations 依 spec 顺序收集该行各列的 Scan 落位与提交动作。
// repository 用它按 spec 顺序执行 position Scan；提交动作接在 Scan 成功后依序执行（#229）。
func ScanEvalExportDestinations(r *EvaluationExportRow) (dests []any, commits []func()) {
	dests = make([]any, 0, len(EvaluationExportColumns))
	commits = make([]func(), 0, len(EvaluationExportColumns))
	for _, col := range EvaluationExportColumns {
		d, c := col.Scan(r)
		dests = append(dests, d)
		commits = append(commits, c)
	}
	return dests, commits
}

// BuildEvalExportSelect 依 spec 顺序生成评估导出的 SELECT 列清单（repository 的唯一来源，#229）。
func BuildEvalExportSelect() string {
	parts := make([]string, 0, len(EvaluationExportColumns))
	for _, col := range EvaluationExportColumns {
		parts = append(parts, col.Select)
	}
	sel := ""
	for i, p := range parts {
		if i > 0 {
			sel += ", "
		}
		sel += p
	}
	return sel
}
