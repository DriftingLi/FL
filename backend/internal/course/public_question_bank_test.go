package course

import (
	"net/http"
	"strings"
	"testing"

	"forklift-training/internal/model"
)

// questionBankFixture 题库目录判据的夹具：四个证件分类，覆盖「有题有课 / 有题无课 /
// 无题 / 不合格」四种形态。
type questionBankFixture struct {
	*publicSchoolFixture

	withCourseCat   model.TrainingCategory // 有题也有公开课
	bankOnlyCat     model.TrainingCategory // 只有题，没有公开课（US-12 的正向夹具）
	draftOnlyCat    model.TrainingCategory // 只有草稿题 ⇒ 不出现在目录里
	unmappedCat     model.TrainingCategory // 无证件映射 ⇒ 不合格
	disabledCat     model.TrainingCategory // 分类停用 ⇒ 不合格
	disabledCredCat model.TrainingCategory // 映射到停用证件 ⇒ 不合格
}

// setupQuestionBankFixture 建题库目录夹具：每个分类对应一条判据。
//
// 在 publicSchoolFixture 之上追加（复用其合格分类 = 有题有课），并把反例分类补齐：
// 题目落库走原始 model.Question，不经题库域服务——本用例测的是**读面聚合**，
// 写面的审核流程与这里无关，走服务反而要绕一圈审核动作。
func setupQuestionBankFixture(t *testing.T) *questionBankFixture {
	t.Helper()
	base := setupPublicSchoolFixture(t)
	f := &questionBankFixture{publicSchoolFixture: base}

	// ① 有题有课：直接用 base.activeCategory（它底下已有 can 的公开课）
	f.withCourseCat = base.activeCategory

	// ② 有题无课：新证件 + 新分类，但**不建任何课程**
	bankCred := model.Credential{Code: "BANK", Name: "只有题库的证件", Category: "special_operation", Status: 1, SortOrder: 10}
	if err := base.db.Create(&bankCred).Error; err != nil {
		t.Fatal(err)
	}
	bankOnly := model.TrainingCategory{Name: "只有题库的证件", SortOrder: 10, Status: 1, LegacyCredentialID: &bankCred.ID}
	if err := base.db.Create(&bankOnly).Error; err != nil {
		t.Fatal(err)
	}
	f.bankOnlyCat = bankOnly

	// ③ 只有草稿题
	draftCred := model.Credential{Code: "DRAFT", Name: "只有草稿题的证件", Category: "special_operation", Status: 1, SortOrder: 11}
	if err := base.db.Create(&draftCred).Error; err != nil {
		t.Fatal(err)
	}
	draftOnly := model.TrainingCategory{Name: "只有草稿题的证件", SortOrder: 11, Status: 1, LegacyCredentialID: &draftCred.ID}
	if err := base.db.Create(&draftOnly).Error; err != nil {
		t.Fatal(err)
	}
	f.draftOnlyCat = draftOnly

	// ④ 无证件映射的分类（有已发布题）
	unmapped := model.TrainingCategory{Name: "无映射题库分类", SortOrder: 12, Status: 1}
	if err := base.db.Create(&unmapped).Error; err != nil {
		t.Fatal(err)
	}
	f.unmappedCat = unmapped

	// ⑤ 停用分类（有已发布题）
	offCred := model.Credential{Code: "OFFCAT", Name: "停用分类的证件", Category: "special_operation", Status: 1, SortOrder: 13}
	if err := base.db.Create(&offCred).Error; err != nil {
		t.Fatal(err)
	}
	offCat := model.TrainingCategory{Name: "停用的题库分类", SortOrder: 13, Status: 1, LegacyCredentialID: &offCred.ID}
	if err := base.db.Create(&offCat).Error; err != nil {
		t.Fatal(err)
	}
	if err := base.db.Model(&offCat).Update("status", int16(0)).Error; err != nil {
		t.Fatal(err)
	}
	f.disabledCat = offCat

	// ⑥ 映射到停用证件的分类（有已发布题）
	deadCred := model.Credential{Code: "DEADCRED", Name: "停用证件", Category: "special_operation", Status: 1, SortOrder: 14}
	if err := base.db.Create(&deadCred).Error; err != nil {
		t.Fatal(err)
	}
	deadCat := model.TrainingCategory{Name: "停用证件的题库分类", SortOrder: 14, Status: 1, LegacyCredentialID: &deadCred.ID}
	if err := base.db.Create(&deadCat).Error; err != nil {
		t.Fatal(err)
	}
	if err := base.db.Model(&deadCred).Update("status", int16(0)).Error; err != nil {
		t.Fatal(err)
	}
	f.disabledCredCat = deadCat

	// 来源标记标签（真题）：命中它的题必须从题库池里排除（US-15）
	sourceTag := model.QuestionTag{Code: "real_exam", Name: "真题", SortOrder: 1, Status: 1, IsSourceTag: true}
	if err := base.db.Create(&sourceTag).Error; err != nil {
		t.Fatal(err)
	}

	addQuestion := func(categoryID *int, status, qType, content string, tagIDs ...int) model.Question {
		t.Helper()
		q := model.Question{Type: qType, Content: content, Answer: "A", Status: status, CategoryID: categoryID}
		if err := base.db.Create(&q).Error; err != nil {
			t.Fatal(err)
		}
		for _, tagID := range tagIDs {
			if err := base.db.Create(&model.QuestionTagRelation{QuestionID: q.ID, TagID: tagID}).Error; err != nil {
				t.Fatal(err)
			}
		}
		return q
	}

	// ① 有题有课：2 single_choice + 1 true_false，另加 1 道草稿 + 1 道真题标记
	withCourseID := f.withCourseCat.ID
	addQuestion(&withCourseID, "published", "single_choice", "池内单选题甲")
	addQuestion(&withCourseID, "published", "single_choice", "池内单选题乙")
	addQuestion(&withCourseID, "published", "true_false", "池内判断题")
	addQuestion(&withCourseID, "draft", "single_choice", "草稿题不该出现")
	addQuestion(&withCourseID, "published", "multi_choice", "真题选项不该计入", sourceTag.ID)

	// ② 有题无课：1 道已发布
	bankOnlyID := f.bankOnlyCat.ID
	addQuestion(&bankOnlyID, "published", "single_choice", "有题无课的题")

	// ③ 只有草稿题
	draftID := f.draftOnlyCat.ID
	addQuestion(&draftID, "draft", "single_choice", "只有草稿题的分类")

	// ④⑤⑥ 三类不合格分类各有 1 道已发布题（证明「被挡」不是因为没题）
	unmappedID := f.unmappedCat.ID
	offCatID := f.disabledCat.ID
	deadCatID := f.disabledCredCat.ID
	addQuestion(&unmappedID, "published", "single_choice", "无映射分类的题")
	addQuestion(&offCatID, "published", "single_choice", "停用分类的题")
	addQuestion(&deadCatID, "published", "single_choice", "停用证件分类的题")

	return f
}

// TestPublicQuestionBanksAggregatePoolByCategory 聚合口径 + 资格判据 + 有题无课
// （票 Testing Decisions：「两个有效证件、无映射与停用分类、停用证件、草稿和来源标记真题」）。
func TestPublicQuestionBanksAggregatePoolByCategory(t *testing.T) {
	f := setupQuestionBankFixture(t)
	w := f.get(t, "/api/public-school/question-banks")
	if w.Code != http.StatusOK {
		t.Fatalf("题库目录 = %d: %s", w.Code, w.Body)
	}
	list := decodeData[PublicQuestionBankListDTO](t, w)

	byID := make(map[int]PublicQuestionBankCategoryDTO, len(list.QuestionBanks))
	for _, item := range list.QuestionBanks {
		byID[item.CategoryID] = item
	}

	// ① 有题有课的证件：total 与分型数量都要对
	withCourse, ok := byID[f.withCourseCat.ID]
	if !ok {
		t.Fatalf("有题有课的分类 %d 没出现在题库目录里: %+v", f.withCourseCat.ID, list.QuestionBanks)
	}
	// 夹具：2 道 single_choice + 1 道 true_false = 3 道池内题；另有 1 道 draft、1 道真题标记
	if withCourse.Total != 3 {
		t.Fatalf("池内总数 = %d, want 3（草稿与真题标记题都不该计入）", withCourse.Total)
	}
	if withCourse.ByType["single_choice"] != 2 || withCourse.ByType["true_false"] != 1 {
		t.Fatalf("分型数量错: %+v", withCourse.ByType)
	}
	if withCourse.CategoryName != f.withCourseCat.Name {
		t.Fatalf("分类显示名 = %q, want %q", withCourse.CategoryName, f.withCourseCat.Name)
	}

	// ② 有题无课：必须出现（US-12）
	bankOnly, ok := byID[f.bankOnlyCat.ID]
	if !ok {
		t.Fatalf("只有题库、没有公开课的分类 %d 被隐藏了（US-12 要求能直接去练习）", f.bankOnlyCat.ID)
	}
	if bankOnly.Total != 1 {
		t.Fatalf("有题无课分类的题数 = %d, want 1", bankOnly.Total)
	}

	// ③ 只有草稿题的分类：不出现在目录里
	if _, ok := byID[f.draftOnlyCat.ID]; ok {
		t.Fatal("只有草稿题的分类出现在了题库目录里（草稿不属题库池）")
	}
	// ④ 三类不合格分类：一律不出现
	for _, tc := range []struct {
		name string
		cat  model.TrainingCategory
	}{
		{"无证件映射", f.unmappedCat},
		{"分类停用", f.disabledCat},
		{"映射到停用证件", f.disabledCredCat},
	} {
		if _, ok := byID[tc.cat.ID]; ok {
			t.Fatalf("%s 的分类 %d 出现在了题库目录里", tc.name, tc.cat.ID)
		}
	}

	// ⑤ 汇总边界：不因为某分类缺席就让别人少算
	if len(list.QuestionBanks) != 2 {
		t.Fatalf("应只出 2 个分类（有题有课 + 有题无课），实得 %d: %+v", len(list.QuestionBanks), list.QuestionBanks)
	}
}

// TestPublicQuestionBanksExposeAggregatesOnly 只出汇总，不出任何题目内容（票 Out of Scope）。
func TestPublicQuestionBanksExposeAggregatesOnly(t *testing.T) {
	f := setupQuestionBankFixture(t)
	w := f.get(t, "/api/public-school/question-banks")
	body := w.Body.String()
	// 夹具题干是具名的可搜字符串：出现在响应里就是泄漏
	for _, leak := range []string{"池内单选题甲", "池内判断题", "草稿题不该出现", "真题选项", `"content"`, `"answer"`, `"explanation"`, `"options"`} {
		if strings.Contains(body, leak) {
			t.Errorf("题库目录泄漏了 %q: %s", leak, body)
		}
	}
}

// TestPublicQuestionBanksEmptyResultIsEmptyList 没有合格题库时返回空数组而不是 null
// （frontend 直接遍历，null 会炸）。
func TestPublicQuestionBanksEmptyResultIsEmptyList(t *testing.T) {
	base := setupPublicSchoolFixture(t)
	// 该夹具没有题目：目录必然是空集
	w := base.get(t, "/api/public-school/question-banks")
	if w.Code != http.StatusOK {
		t.Fatalf("题库目录 = %d: %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"question_banks":[]`) {
		t.Fatalf("空结果应是 []，实得: %s", w.Body)
	}
}
