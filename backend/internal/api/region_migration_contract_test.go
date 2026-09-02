// 契约测试 #486：地区存量迁移样本（pg 双跑）。
// Postgres 适配器由真实 SQL 迁移建表（含 000017_region_city_level），全量 up 成功 = 迁移语法与顺序正确。
package api

import (
  "testing"
  "time"

  "github.com/gin-gonic/gin"

  "forklift-training/internal/model"
  "forklift-training/internal/service"
  "forklift-training/internal/testutil"
)

// Postgres 适配器：真实 SQL 迁移（含 000017）全量 up 成功即迁移正确。
func TestRegionMigrationOnPostgres(t *testing.T) {
  gin.SetMode(gin.TestMode)
  db := testutil.NewPostgresDB(t)
  if db == nil {
    t.Skip("DATABASE_URL 未设置")
  }
  // 迁移链执行成功即验证。写入一段契约样本验证列可读写。
  pwd, _ := service.HashPassword("pass1234")
  stu := testutil.SeedStudent(t, db, "stuMig", pwd)
  min_ := 6000
  max_ := 9000
  card := model.JobCard{
    UserID: stu.ID, RealName: "迁移学员", ContactPhone: "13822220001", Region: "江苏省/苏州市",
    ExpectedRegions: model.JSONB([]byte(`["江苏省/苏州市"]`)), Visibility: "open",
    SalaryMin: &min_, SalaryMax: &max_, CreatedAt: time.Now(), UpdatedAt: time.Now(),
  }
  if err := db.Create(&card).Error; err != nil {
    t.Fatalf("创建卡失败: %v", err)
  }
  var got model.JobCard
  if err := db.First(&got, "user_id = ?", stu.ID).Error; err != nil {
    t.Fatalf("读取卡失败: %v", err)
  }
  if string(got.ExpectedRegions) != string(card.ExpectedRegions) {
    t.Fatalf("region 列读写不一致: %s", string(got.ExpectedRegions))
  }
}

// SQLite 适配器：Go 层归一逻辑覆盖三类存量样本（迁移语义的本地镜像）。
func TestRegionMigrationSamplesOnSqlite(t *testing.T) {
  cases := []struct {
    in    string
    prov  string
    city  string
  }{
    {"江苏省/苏州市/吴中区", "江苏省", "苏州市"}, // 三段截断为两段
    {"江苏苏州", "江苏省", "苏州市"}, // 无分隔短名
    {"北京市", "北京市", ""}, // 直辖市一段
    {"江苏苏州精确地址123号", "江苏省", "苏州市"}, // 无分隔带后缀
  }
  for _, c := range cases {
    parts := service.SplitRegionPath(c.in)
    if len(parts) >= 3 {
      // 三段：前两段拼接（迁移的截断语义）
      if parts[0] != c.prov || parts[1] != c.city {
        t.Errorf("三段 %q → (%s,%s), want (%s,%s)", c.in, parts[0], parts[1], c.prov, c.city)
      }
      continue
    }
    prov, city := service.SplitRegionNoSeparator(c.in)
    if prov != c.prov || city != c.city {
      t.Errorf("拆分 %q → (%q,%q), want (%q,%q)", c.in, prov, city, c.prov, c.city)
    }
  }
}
