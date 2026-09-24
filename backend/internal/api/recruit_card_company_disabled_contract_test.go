// 契约测（ADR-0064 决策 5 / 移动端 #1267 第 3 条）：「企业已停用」这一维必须在**驱动角标的卡面**
// 上也可分辨，不能只挂在明文位置。
//
// 缺陷形状：`company_disabled` 只声明在 `contact_service.go` 的 ContactRequestDTO 上，
// 而招聘者工作区的列表角标读的是 `RecruitResumeCard.contact_state` ⇒ 企业被禁用（处置动作）后，
// 后端仍按授权事实把 contact_state 投影成 approved，卡面没有任何一格说「明文取不到」。
// 于是消费方出现「角标『已授权』/ 正文『尚未授权 · 去发起交换』」的矛盾态，且**列表页修不了**——
// 它压根没收到那一维（Web 与移动端同形，#1267 的退回诉求就是这条）。
//
// 判据（词表「授权有效态」）：授权存在 ≠ 授权可用。徽章继续按授权事实投影**不变**，
// 可用性另用同一格 key 说；**不**用「禁用时把 contact_state 降级」来统一，
// 那会把「被禁用」与「从没授权」在列表上重新压回同值（决策 5 的否备选）。
//
// seam = 后端 HTTP 契约层；夹具复用 newContactDisableEnv（真实链路：建企业→登录→发起→学员同意）。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

// recruitCardRow 招聘者简历卡（列表与详情共用；只取本票关心的三格）。
// company_disabled 用指针，以区分「缺席」与「显式 false」——契约是 omitempty 的可选槽。
type recruitCardRow struct {
	UserID          int    `json:"user_id"`
	ContactState    string `json:"contact_state"`
	CompanyDisabled *bool  `json:"company_disabled"`
}

// readRecruitCard 以招聘者身份打列表，返回该学员那一行。
func readRecruitCard(t *testing.T, env *contactDisableEnv) recruitCardRow {
	t.Helper()
	rec := doWithToken(t, env.router, env.recruiterTok, http.MethodGet, "/api/recruit/resumes", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("招聘者拉简历列表失败 %d %s", rec.Code, rec.Body.String())
	}
	var page struct {
		Data struct {
			Items []recruitCardRow `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("解析列表失败: %v (%s)", err, rec.Body.String())
	}
	for _, it := range page.Data.Items {
		if it.UserID == env.studentID {
			return it
		}
	}
	t.Fatalf("列表里没有该学员的卡（items=%d 条）", len(page.Data.Items))
	return recruitCardRow{}
}

// TestRecruitCardNoDisabledFlagWhenCompanyUsable 企业可用 ⇒ 卡面上**不得出现** company_disabled
// （缺席即正常态；出现即恒 true 是 #1265 第 3 条定下的契约形状）。
func TestRecruitCardNoDisabledFlagWhenCompanyUsable(t *testing.T) {
	env := newContactDisableEnv(t)

	row := readRecruitCard(t, env)
	if row.ContactState != "approved" {
		t.Fatalf("夹具应造出一条 approved 授权，实际 contact_state=%q", row.ContactState)
	}
	if row.CompanyDisabled != nil {
		t.Fatalf("企业可用时不该出现 company_disabled，实际 %v", *row.CompanyDisabled)
	}
}

// TestRecruitCardMarksCompanyDisabledAfterAdminDisable 管理员禁用这家企业后：徽章仍按授权事实
// 投影为 approved（处置不改写授权事实，ADR-0051 的「不追溯改写」），但卡面必须给出具名说明，
// 否则消费方只能继续显一个说谎的角标。
func TestRecruitCardMarksCompanyDisabledAfterAdminDisable(t *testing.T) {
	env := newContactDisableEnv(t)
	toggleCompany(t, env)

	row := readRecruitCard(t, env)
	if row.ContactState != "approved" {
		t.Fatalf("徽章应仍按授权事实投影为 approved（不得靠降级表达可用性），实际 %q", row.ContactState)
	}
	if row.CompanyDisabled == nil || !*row.CompanyDisabled {
		t.Fatalf("卡面要有具名说明「企业已停用」而不是静默缺席，实际 company_disabled=%v", row.CompanyDisabled)
	}
}

// TestRecruitCardDetailAlsoMarksDisabled 详情卡与列表卡同一装配点：只回填列表会让详情面继续说谎
// （「统一」不能只做一半）。
func TestRecruitCardDetailAlsoMarksDisabled(t *testing.T) {
	env := newContactDisableEnv(t)
	toggleCompany(t, env)

	rec := doWithToken(t, env.router, env.recruiterTok, http.MethodGet,
		"/api/recruit/resumes/"+strconv.Itoa(env.studentID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("招聘者拉详情卡失败 %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data recruitCardRow `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析详情卡失败: %v (%s)", err, rec.Body.String())
	}
	if body.Data.CompanyDisabled == nil || !*body.Data.CompanyDisabled {
		t.Fatalf("详情卡也要带 company_disabled，实际 %v（body=%s）", body.Data.CompanyDisabled, rec.Body.String())
	}
}
