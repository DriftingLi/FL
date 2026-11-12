// 课程积分价写入面（#2447）：三种输入形态（≥1 落值 / 显式 null 清空 / 未携带保留）与
// 非法值（0、负数、非整数）的拒绝。全部从 JSON 解码进 CourseInput 再走服务——与端点同一条路，
// 顺带守住「显式 null 与缺键解码后分得开」这个前提（缺了它「清空」无从表达）。
package course

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func inputFromJSON(t *testing.T, body string) *CourseInput {
	t.Helper()
	var in CourseInput
	if err := json.Unmarshal([]byte(body), &in); err != nil {
		t.Fatalf("请求体解码失败: %v", err)
	}
	return &in
}

func TestCourseInputPointsPriceDecode(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		price   int
		has     bool
		cleared bool
	}{
		{name: "未携带", body: `{"name":"x"}`, has: false, cleared: false},
		{name: "显式 null", body: `{"points_price":null}`, has: false, cleared: true},
		{name: "携带值", body: `{"points_price":100}`, has: true, price: 100, cleared: false},
		{name: "携带 0", body: `{"points_price":0}`, has: true, price: 0, cleared: false}, // 解码放行，拒绝在写面校验
	}
	for _, tc := range cases {
		in := inputFromJSON(t, tc.body)
		if tc.has != (in.PointsPrice != nil) {
			t.Fatalf("%s: PointsPrice 有值形态不对: %+v", tc.name, in.PointsPrice)
		}
		if tc.has && *in.PointsPrice != tc.price {
			t.Fatalf("%s: PointsPrice 应为 %d, got %d", tc.name, tc.price, *in.PointsPrice)
		}
		if in.pointsPriceCleared != tc.cleared {
			t.Fatalf("%s: 显式清空标志应为 %v", tc.name, tc.cleared)
		}
	}
	// 非整数在解码层就拒（400「请求数据无效」面），不进服务
	for _, body := range []string{`{"points_price":1.5}`, `{"points_price":"100"}`, `{"points_price":true}`} {
		var in CourseInput
		if err := json.Unmarshal([]byte(body), &in); err == nil {
			t.Fatalf("%s 应解码失败", body)
		}
	}
}

func TestAdminCourse_PointsPriceWrite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAdminService(db, nil, zap.NewNop())
	spec := model.Specialty{Code: "maintenance", Name: "维修", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&spec)
	lv := model.CourseLevel{Code: "beginner", Name: "入门", Status: 1, CreatedAt: testutil.Now()}
	db.Create(&lv)
	base := fmt.Sprintf(`"specialty_id":%d,"level_id":%d`, spec.SpecialtyID, lv.LevelID)

	// 携带值：≥1 校验后落库（响应与库里都要看得到）
	created, err := svc.CreateCourse(inputFromJSON(t, fmt.Sprintf(`{"name":"付费课",%s,"points_price":100}`, base)))
	if err != nil {
		t.Fatalf("创建付费课失败: %v", err)
	}
	if created.PointsPrice == nil || *created.PointsPrice != 100 {
		t.Fatalf("创建响应应带积分价 100: %+v", created.PointsPrice)
	}
	var row model.Course
	if err := db.First(&row, created.CourseID).Error; err != nil {
		t.Fatalf("查库失败: %v", err)
	}
	if row.PointsPrice == nil || *row.PointsPrice != 100 {
		t.Fatalf("积分价未落库: %+v", row.PointsPrice)
	}

	// 0 与负数拒绝，且不落任何行
	for _, bad := range []string{"0", "-1"} {
		_, err := svc.CreateCourse(inputFromJSON(t, fmt.Sprintf(`{"name":"坏价课",%s,"points_price":%s}`, base, bad)))
		if !errors.Is(err, ErrCoursePointsPriceInvalid) {
			t.Fatalf("points_price=%s 应拒（%v）", bad, err)
		}
	}

	// 新建：未携带与显式 null 都是免费（NULL）
	free := map[string]bool{}
	for _, body := range []string{
		fmt.Sprintf(`{"name":"免费课A",%s}`, base),
		fmt.Sprintf(`{"name":"免费课B",%s,"points_price":null}`, base),
	} {
		out, err := svc.CreateCourse(inputFromJSON(t, body))
		if err != nil {
			t.Fatalf("创建免费课失败: %v", err)
		}
		if out.PointsPrice != nil {
			t.Fatalf("免费课不应带积分价: %+v", out.PointsPrice)
		}
		free[out.Name] = true
	}
	if len(free) != 2 {
		t.Fatalf("两门免费课都应建成: %v", free)
	}

	// 编辑：未携带 = 保留现状
	updated, err := svc.UpdateCourse(created.CourseID, inputFromJSON(t, `{"name":"付费课改名"}`))
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.PointsPrice == nil || *updated.PointsPrice != 100 {
		t.Fatalf("未携带 points_price 不应动价: %+v", updated.PointsPrice)
	}

	// 编辑：0 拒绝且原价不动
	if _, err := svc.UpdateCourse(created.CourseID, inputFromJSON(t, `{"points_price":0}`)); !errors.Is(err, ErrCoursePointsPriceInvalid) {
		t.Fatalf("编辑传 0 应拒: %v", err)
	}
	if err := db.First(&row, created.CourseID).Error; err != nil {
		t.Fatalf("查库失败: %v", err)
	}
	if row.PointsPrice == nil || *row.PointsPrice != 100 {
		t.Fatalf("被拒的编辑不应动价: %+v", row.PointsPrice)
	}

	// 编辑：显式 null = 清空为免费（NULL）
	cleared, err := svc.UpdateCourse(created.CourseID, inputFromJSON(t, `{"points_price":null}`))
	if err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	if cleared.PointsPrice != nil {
		t.Fatalf("清空后不应带价: %+v", cleared.PointsPrice)
	}
	if err := db.First(&row, created.CourseID).Error; err != nil {
		t.Fatalf("查库失败: %v", err)
	}
	if row.PointsPrice != nil {
		t.Fatalf("库里应是 NULL: %+v", row.PointsPrice)
	}
}
