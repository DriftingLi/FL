// Package sortorder 组内排序位置的三枚无状态助手（#1445 P2 波 3b-1 的共享叶子）。
//
// 落点理由：原住 internal/service/training_catalog_service.go。课程域的 admin_course_service.go
// 与培训域的 catalog_engine.go 都要用它们，而两域互为强环（15 条边）⇒ 载体留在任一域包里都会
// 让另一个域反向依赖它；留 internal/core 导出也不行 —— 留驻 core → 域包是单向边，反过来
// 又要求 service 把它导出给域包用就成环。它们是无状态纯函数（只吃 *gorm.DB 与参数）——
// P2 五种破环手法的第三种（无状态纯函数进叶子包），同 internal/coerce / internal/timefmt 先例。
package sortorder

import (
	"errors"

	"gorm.io/gorm"

	"forklift-training/internal/coerce"
)

// ErrSwapItemNotFound 是 `swap_with` 指向的那一行不在本组序列里（ADR-0065 决策 3）。
// 它是输入不合法（400）而不是 404：404 说的是「路径里那个资源没有」，而路径资源在这里是好的。
var ErrSwapItemNotFound = errors.New("待交换的项不存在")

// ErrEntityNotSortable 对一张没开排序的目录表请求 swap（ADR-0065 决策 3：输入不合法，400）。
// 与 ErrSwapItemNotFound 同属「交换排序这一族的输入事实」，同住本叶子包：课程域与培训域都要拿
// 这一族事实建各自的 400 表，谁住谁的域包都会让另一边反向依赖（波 4d 破 course↔training 环）。
var ErrEntityNotSortable = errors.New("该实体不支持排序交换")

// NextValue 返回表内（可选按组过滤）当前最大 sort_order + 1，新项排末尾。
func NextValue(db *gorm.DB, table string, where map[string]any) int {
	q := db.Table(table).Select("COALESCE(MAX(sort_order), 0)")
	for k, v := range where {
		q = q.Where(k+" = ?", v)
	}
	var max int
	if err := q.Scan(&max).Error; err != nil {
		return 1
	}
	return max + 1
}

// Renumber 按 (sort_order, id) 升序把组内全部项重新顺序编号（1..N），返回新序 ID 列表。
// 消除同值 sort_order（默认 0）导致的顺序不可控。
func Renumber(db *gorm.DB, entity any, idCol string, where map[string]any) ([]int, error) {
	var rows []map[string]any
	q := db.Model(entity).Select(idCol + ", sort_order")
	for k, v := range where {
		q = q.Where(k+" = ?", v)
	}
	q.Order("sort_order ASC, " + idCol + " ASC")
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, toInt(r[idCol]))
	}
	return ids, nil
}

// SwapPositions 把组内两项交换位置：重编号后交换 a/b 在新序中的下标，再整体落库。
// 即使两项 sort_order 相同（默认 0）也真实生效。
func SwapPositions(db *gorm.DB, entity any, idCol string, idA, idB int, where map[string]any) error {
	ids, err := Renumber(db, entity, idCol, where)
	if err != nil {
		return err
	}
	ia, ib := -1, -1
	for i, id := range ids {
		if id == idA {
			ia = i
		}
		if id == idB {
			ib = i
		}
	}
	if ia < 0 || ib < 0 {
		// 与「前置课程不存在」同判（都指向 body 里给的一枚坏引用 ⇒ 400），不是「路径上那个对象没有」。
		return ErrSwapItemNotFound
	}
	if ia == ib {
		return nil
	}
	ids[ia], ids[ib] = ids[ib], ids[ia]
	return db.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(entity).Where(idCol+" = ?", id).Update("sort_order", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// toInt 将任意数值转为 int（gorm 把 map 扫描出来的数值列交回 interface{}，驱动不同会换类型）。
func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		i, _ := coerce.ParseInt(n)
		return i
	}
	return 0
}
