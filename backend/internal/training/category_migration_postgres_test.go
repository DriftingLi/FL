package training

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
	"forklift-training/migrations"
)

func TestTrainingCategoryMigrationOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	migrationSQL := func(migration, direction string) string {
		t.Helper()
		path := filepath.Join("testdata", migration+"."+direction+".sql")
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(sql)
	}
	apply := func(migration, direction string) {
		t.Helper()
		if err := db.Exec(migrationSQL(migration, direction)).Error; err != nil {
			t.Fatalf("category migration %s: %v", direction, err)
		}
	}
	readID := func(sql string, args ...any) int {
		t.Helper()
		var id int
		if err := db.Raw(sql, args...).Row().Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}

	const categoryMigration = "000069_training_category"
	const publicationMigration = "000070_training_course_publication"
	if err := db.Exec(migrations.DropPublicStateViews).Error; err != nil {
		t.Fatalf("migration 000077 public state down: %v", err)
	}
	if err := db.Exec(migrations.DropPublicEligibilityViews).Error; err != nil {
		t.Fatalf("migration 000076 public eligibility down: %v", err)
	}
	apply("000073_wrong_question_categories", "down")
	apply(categoryMigration, "down")
	student := testutil.SeedStudent(t, db, "category-migration", "x")
	activeID := readID(`INSERT INTO credential (id, code, name, category, sort_order, status)
		OVERRIDING SYSTEM VALUE VALUES (10001, 'category_live', 'Migration active', 'special_operation', 42, 1) RETURNING id`)
	inactiveID := readID(`INSERT INTO credential (id, code, name, category, sort_order, status)
		OVERRIDING SYSTEM VALUE VALUES (10002, 'category_inactive', 'Migration inactive', 'special_operation', 43, 0) RETURNING id`)
	courseID := readID("INSERT INTO course (name, credential_id) VALUES ('Category course', ?) RETURNING course_id", activeID)
	visibleID := readID(`INSERT INTO course (name, credential_id, specialty_id, level_id)
		VALUES ('Already visible', ?, (SELECT min(specialty_id) FROM specialty), (SELECT min(level_id) FROM course_level)) RETURNING course_id`, activeID)
	questionID := readID("INSERT INTO question (type, content, answer, credential_id) VALUES ('single_choice', 'Moved question', 'A', ?) RETURNING id", activeID)
	paperID := readID("INSERT INTO real_exam_paper (title, source_ref, credential_id) VALUES ('Legacy paper', 'category-paper', ?) RETURNING paper_id", activeID)
	contributionID := readID("INSERT INTO user_contribution (user_id, credential_id, title, intro) VALUES (?, ?, 'Legacy material', 'old') RETURNING id", student.ID, activeID)
	progressID := readID("INSERT INTO practice_progress (student_id, practice_mode, credential_id, question_ids) VALUES (?, 'sequential', ?, '[]') RETURNING id", student.ID, inactiveID)
	attemptID := readID("INSERT INTO question_practice_record (student_id, question_id, credential_id, user_answer) VALUES (?, ?, ?, 'B') RETURNING id", student.ID, questionID, inactiveID)
	examID := readID("INSERT INTO mock_exam (student_id, credential_id) VALUES (?, ?) RETURNING id", student.ID, inactiveID)
	unclassifiedID := readID("INSERT INTO mock_exam (student_id) VALUES (?) RETURNING id", student.ID)
	wrongID := readID("INSERT INTO wrong_question (student_id, question_id) VALUES (?, ?) RETURNING id", student.ID, questionID)
	if err := db.Exec(`INSERT INTO mock_exam (student_id, credential_id, status, result)
		VALUES (?, ?, 'submitted', jsonb_build_object('details', jsonb_build_array(
			jsonb_build_object('question_id', CAST(? AS integer), 'is_correct', false))))`,
		student.ID, activeID, questionID).Error; err != nil {
		t.Fatal(err)
	}

	apply(categoryMigration, "up")
	readCategory := func(credentialID int) model.TrainingCategory {
		t.Helper()
		var category model.TrainingCategory
		if err := db.Where("legacy_credential_id = ?", credentialID).Take(&category).Error; err != nil {
			t.Fatal(err)
		}
		return category
	}
	active := readCategory(activeID)
	inactive := readCategory(inactiveID)
	if active.ID == activeID || inactive.ID == inactiveID || active.ID == inactive.ID ||
		active.Name != "Migration active" || active.SortOrder != 42 || active.Status != 1 ||
		inactive.Name != "Migration inactive" || inactive.SortOrder != 43 || inactive.Status != 0 {
		t.Fatalf("bad historical mapping: active=%+v inactive=%+v", active, inactive)
	}

	apply(categoryMigration, "up")
	for _, check := range []struct {
		query string
		id    int
		want  *int
	}{
		{"SELECT category_id FROM course WHERE course_id = ?", courseID, &active.ID},
		{"SELECT category_id FROM question WHERE id = ?", questionID, &active.ID},
		{"SELECT category_id FROM real_exam_paper WHERE paper_id = ?", paperID, &active.ID},
		{"SELECT category_id FROM user_contribution WHERE id = ?", contributionID, &active.ID},
		{"SELECT category_id FROM practice_progress WHERE id = ?", progressID, &inactive.ID},
		{"SELECT category_id FROM question_practice_record WHERE id = ?", attemptID, &inactive.ID},
		{"SELECT category_id FROM mock_exam WHERE id = ?", examID, &inactive.ID},
		{"SELECT category_id FROM mock_exam WHERE id = ?", unclassifiedID, nil},
	} {
		var got *int
		if err := db.Raw(check.query, check.id).Row().Scan(&got); err != nil {
			t.Fatal(err)
		}
		if categoryValue(got) != categoryValue(check.want) {
			t.Errorf("%s (%d): category = %v, want %v", check.query, check.id, categoryValue(got), categoryValue(check.want))
		}
	}
	if got := readCategory(activeID).ID; got != active.ID {
		t.Errorf("replay created a new mapping: got %d, want %d", got, active.ID)
	}
	if got := readID("SELECT status FROM course WHERE course_id = ?", courseID); got != 1 {
		t.Errorf("additive migration changed the course publication status: got %d", got)
	}
	missingID := readID(`INSERT INTO course (name, specialty_id, level_id)
		VALUES ('Needs manual review', (SELECT min(specialty_id) FROM specialty), (SELECT min(level_id) FROM course_level)) RETURNING course_id`)
	if err := db.Exec(migrationSQL(publicationMigration, "up")).Error; err == nil ||
		!strings.Contains(err.Error(), "Previously visible courses require a classification") {
		t.Fatalf("missing category should block cutover: %v", err)
	}
	if got := readID("SELECT status FROM course WHERE course_id = ?", courseID); got != 1 {
		t.Fatalf("blocked cutover changed publication status: got %d", got)
	}
	if err := db.Exec("UPDATE course SET category_id = ? WHERE course_id = ?", active.ID, missingID).Error; err != nil {
		t.Fatal(err)
	}
	apply(publicationMigration, "up")
	if got := readID("SELECT status FROM course WHERE course_id = ?", courseID); got != 0 {
		t.Errorf("previously hidden course is still marked published: got %d", got)
	}
	if got := readID("SELECT status FROM course WHERE course_id = ?", visibleID); got != 1 {
		t.Errorf("previously visible course was unpublished: got %d", got)
	}
	if got := readID("SELECT status FROM course WHERE course_id = ?", missingID); got != 1 {
		t.Errorf("manually classified course was unpublished: got %d", got)
	}

	writeProgress := "INSERT INTO practice_progress (student_id, practice_mode, category_id, question_ids) VALUES (?, 'tag:1', ?, '[]') RETURNING id"
	legacyProgress := readID(writeProgress, student.ID, nil)
	firstProgress := readID(writeProgress, student.ID, active.ID)
	secondProgress := readID(writeProgress, student.ID, inactive.ID)
	if legacyProgress == firstProgress || firstProgress == secondProgress || legacyProgress == secondProgress {
		t.Fatal("legacy and classified progress must retain separate rows")
	}
	if err := db.Exec("INSERT INTO practice_progress (student_id, practice_mode, category_id, question_ids) VALUES (?, 'tag:1', ?, '[]')", student.ID, active.ID).Error; err == nil {
		t.Fatal("duplicate progress for the same student, mode and category was accepted")
	}
	apply("000073_wrong_question_categories", "up")
	var historicalEdges []model.WrongQuestionCategory
	if err := db.Where("wrong_question_id = ?", wrongID).Find(&historicalEdges).Error; err != nil {
		t.Fatal(err)
	}
	mapped := make(map[int]bool)
	for _, edge := range historicalEdges {
		if edge.CategoryID != nil {
			mapped[*edge.CategoryID] = true
		}
	}
	if len(historicalEdges) != 2 || !mapped[active.ID] || !mapped[inactive.ID] {
		t.Fatalf("correct-answer provenance from practice and exam records not preserved: %+v", historicalEdges)
	}
}

func categoryValue(id *int) any {
	if id == nil {
		return nil
	}
	return *id
}
