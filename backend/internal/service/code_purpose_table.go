// 本文件：验证码用途声明表——「用途」是验证码状态机的分区键。
//
// 六个消费面都从这张表取值（新增用途 = 加一行）：
//  1. 匿名发码入口白名单（api 层，刻意保留为显式清单，见 api/anonymousSendPurposes）
//  2. 目标占用校验规则与错误文案（send）
//  3. 邮件主题与操作词（EmailChannel.Render）
//  4. 短信模板键与参数量（TencentSMSProvider.templateFor）
//  5. 短信启动自检的模板集合（TencentSMSProvider.ValidateReady）
//  6. 短信配置就绪断言的模板集合（config.SMSConfig.Configured）
//
// 表里只有纯数据，不放闭包——邮件正文拼接、短信 API 调用、占用校验动作仍留在各消费面。
// 完整性由 code_purpose_table_test.go 的三条锁把住：结构完整性、行为级逐字、单向不变式。
package service

import (
	"strings"

	"forklift-training/internal/config"
)

// codeTargetRule 目标占用校验规则：发送验证码前对目标账号的检查口径。
type codeTargetRule string

const (
	// codeTargetFree 目标必须未被占用（注册 / 绑定）。
	codeTargetFree codeTargetRule = "must_be_free"
	// codeTargetExist 目标必须已注册（登录 / 找回密码）。
	codeTargetExist codeTargetRule = "must_exist"
	// codeTargetNone 无需占用校验——目标是当前用户自己的账号（改账号 / 改密码）。
	codeTargetNone codeTargetRule = "none"
)

// codePurposeSpec 用途声明行：7 个纯数据槽位。
type codePurposeSpec struct {
	// RequiresSession 完成该用途的动作是否需要已登录会话。
	RequiresSession bool
	// TargetRule 发送时的目标占用校验规则。
	TargetRule codeTargetRule
	// TargetRuleMsg 占用校验不通过时的错误文案，%s 为通道目标名（「邮箱」/「手机号」）。
	// TargetRule 为 codeTargetNone 时本槽位必须为空字符串——不允许留死数据。
	TargetRuleMsg string
	// EmailTitle 邮件主题。
	EmailTitle string
	// EmailOp 邮件正文里的操作词（「您正在进行<op>操作」）。
	EmailOp string
	// SMSTemplate 短信模板键（键的取值域见 config.SMSTemplateKey）。
	SMSTemplate config.SMSTemplateKey
	// SMSParamCount 短信模板参数量（登录模板为 2：验证码 + 有效分钟数；其余为 1）。
	SMSParamCount int
}

// codePurposeEntry 表的一行（用途 + 声明），顺序即派生顺序。
type codePurposeEntry struct {
	Purpose CodePurpose
	Spec    codePurposeSpec
}

// codePurposeTable 验证码用途声明表：六种用途，一种一行。
var codePurposeTable = []codePurposeEntry{
	{CodePurposeRegister, codePurposeSpec{
		RequiresSession: false,
		TargetRule:      codeTargetFree,
		TargetRuleMsg:   "该%s已注册，请直接登录",
		EmailTitle:      "【和润天下】注册验证码",
		EmailOp:         "注册",
		SMSTemplate:     config.SMSTemplateKeyRegister,
		SMSParamCount:   1,
	}},
	{CodePurposeLogin, codePurposeSpec{
		RequiresSession: false,
		TargetRule:      codeTargetExist,
		TargetRuleMsg:   "该%s尚未注册",
		EmailTitle:      "【和润天下】登录验证码",
		EmailOp:         "登录",
		SMSTemplate:     config.SMSTemplateKeyLogin,
		SMSParamCount:   2,
	}},
	{CodePurposeBind, codePurposeSpec{
		RequiresSession: true,
		TargetRule:      codeTargetFree,
		TargetRuleMsg:   "该%s已被其他账号使用",
		EmailTitle:      "【和润天下】邮箱绑定验证码",
		EmailOp:         "绑定/修改邮箱",
		SMSTemplate:     config.SMSTemplateKeyBindPhone,
		SMSParamCount:   1,
	}},
	{CodePurposeAccountChange, codePurposeSpec{
		RequiresSession: true,
		TargetRule:      codeTargetNone,
		EmailTitle:      "【和润天下】修改登录账号验证码",
		EmailOp:         "修改登录账号",
		SMSTemplate:     config.SMSTemplateKeyBindPhone,
		SMSParamCount:   1,
	}},
	{CodePurposeResetPassword, codePurposeSpec{
		RequiresSession: false,
		TargetRule:      codeTargetExist,
		TargetRuleMsg:   "该%s尚未注册",
		EmailTitle:      "【和润天下】找回密码验证码",
		EmailOp:         "找回密码",
		SMSTemplate:     config.SMSTemplateKeyPassword,
		SMSParamCount:   1,
	}},
	{CodePurposeChangePassword, codePurposeSpec{
		RequiresSession: true,
		TargetRule:      codeTargetNone,
		EmailTitle:      "【和润天下】修改密码验证码",
		EmailOp:         "修改密码",
		SMSTemplate:     config.SMSTemplateKeyPassword,
		SMSParamCount:   1,
	}},
}

// codePurposeSpecs 用途 → 声明行（由 codePurposeTable 派生，供 O(1) 查找）。
var codePurposeSpecs = func() map[CodePurpose]codePurposeSpec {
	m := make(map[CodePurpose]codePurposeSpec, len(codePurposeTable))
	for _, e := range codePurposeTable {
		m[e.Purpose] = e.Spec
	}
	return m
}()

// codePurposeSpecFor 按用途取声明行；未声明的用途返回 ok=false（调用方 fail-closed）。
func codePurposeSpecFor(p CodePurpose) (codePurposeSpec, bool) {
	spec, ok := codePurposeSpecs[p]
	return spec, ok
}

// CodePurposeRequiresSession 该用途的动作是否需要已登录会话。
// api 层的匿名发码白名单用它做单向不变式检查（白名单 ⊆ 非 RequiresSession 用途）。
func CodePurposeRequiresSession(p CodePurpose) bool {
	return codePurposeSpecs[p].RequiresSession
}

// CodePurposeSMSTemplates 表中引用的全部短信模板键（去重，按表顺序）。
// 短信启动自检与配置就绪断言由它派生「需要哪些模板」，不再硬编码四个。
func CodePurposeSMSTemplates() []config.SMSTemplateKey {
	seen := make(map[config.SMSTemplateKey]bool, len(codePurposeTable))
	keys := make([]config.SMSTemplateKey, 0, len(codePurposeTable))
	for _, e := range codePurposeTable {
		k := e.Spec.SMSTemplate
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	return keys
}

// targetRuleMessage 渲染占用校验失败文案（%s 为通道目标名，如「邮箱」/「手机号」）。
func targetRuleMessage(spec codePurposeSpec, noun string) string {
	return strings.ReplaceAll(spec.TargetRuleMsg, "%s", noun)
}
