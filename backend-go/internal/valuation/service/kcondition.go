// Package service 实现核心业务逻辑
// 本文件：车况系数 Kc（两级加权模型）
// 第一级（类别级）：Kc = Σ(wcᵢ × Kcᵢ)
// 第二级（条目级）：Kcᵢ = Σ(wiⱼ × Kciⱼ)
// Kciⱼ 由条目状态映射得到：normal=1.0 / slight_wear=0.85 / need_repair=0.6 / need_replace=0.3
package service

import (
	"context"
	"fmt"
	"sort"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// PartConfigLoader 部件配置加载器（按叉车类型缓存）
type PartConfigLoader struct {
	queries *repository.Queries
	cache   map[model.ForkliftType][]model.PartConfigInfo
}

// NewPartConfigLoader 构造部件配置加载器
func NewPartConfigLoader(q *repository.Queries) *PartConfigLoader {
	return &PartConfigLoader{
		queries: q,
		cache:   make(map[model.ForkliftType][]model.PartConfigInfo),
	}
}

// LoadAll 预加载电动与内燃两种类型的部件配置
func (p *PartConfigLoader) LoadAll(ctx context.Context) error {
	for _, ft := range []model.ForkliftType{model.ForkliftTypeElectric, model.ForkliftTypeCombustion} {
		rows, err := p.queries.ListPartConfigs(ctx, string(ft))
		if err != nil {
			return fmt.Errorf("加载部件配置失败(%s): %w", ft, err)
		}
		items := make([]model.PartConfigInfo, 0, len(rows))
		for _, r := range rows {
			items = append(items, model.PartConfigInfo{
				CategoryCode:   r.CategoryCode,
				CategoryName:   r.CategoryName,
				CategoryWeight: r.CategoryWeight,
				ItemCode:       r.ItemCode,
				ItemName:       r.ItemName,
				ItemWeight:     r.ItemWeight,
			})
		}
		p.cache[ft] = items
	}
	return nil
}

// GetParts 获取指定叉车类型的所有部件配置
func (p *PartConfigLoader) GetParts(ft model.ForkliftType) []model.PartConfigInfo {
	return p.cache[ft]
}

// KcResult 车况系数计算结果
type KcResult struct {
	KCondition float64            // 车况系数 Kc
	Categories []CategoryScore    // 每个类别的明细
}

// CategoryScore 单个类别的车况得分
type CategoryScore struct {
	CategoryCode   string           // 类别编码
	CategoryName   string           // 类别名称
	CategoryWeight float64          // 类别权重 wc
	KCategory      float64          // 该类别的车况系数 Kcᵢ
	Items          []ItemScore      // 该类别下条目级明细
}

// ItemScore 单个条目的车况得分
type ItemScore struct {
	ItemCode   string          // 条目编码
	ItemName   string          // 条目名称
	ItemWeight float64         // 条目权重 wi
	Status     model.ItemStatus // 用户提交的状态
	Score      float64         // 状态评分 Kciⱼ
}

// CalcKCondition 计算车况系数 Kc
// forkliftType: 叉车类型，决定加载哪套部件配置
// items: 用户提交的部件状态列表
// 流程：
//  1. 加载该类型所有部件配置（按 category 分组）
//  2. 对每个类别，按条目权重 × 状态评分 加权求和得 Kcᵢ
//  3. 类别系数按 wcᵢ 加权求和得 Kc
//  4. Kc 受 can_drive 与 hydraulic_ok 影响（这两个全局开关对所有条目生效）
func CalcKCondition(
	forkliftType model.ForkliftType,
	items []model.ItemInput,
	loader *PartConfigLoader,
	canDrive, hydraulicOK bool,
) (KcResult, error) {
	// 0. 基础校验
	if len(items) == 0 {
		return KcResult{}, model.ErrItemsEmpty
	}

	// 1. 加载配置并按类别分组
	configs := loader.GetParts(forkliftType)
	if len(configs) == 0 {
		return KcResult{}, model.ErrPartConfigMissing
	}

	// 用户提交的 item_code → status 映射
	statusMap := make(map[string]model.ItemStatus, len(items))
	for _, it := range items {
		statusMap[it.ItemCode] = it.Status
	}

	// 按 category_code 分组
	type categoryBucket struct {
		info   model.PartConfigInfo // 类别信息（取首条记录的类别权重、名称）
		parts  []model.PartConfigInfo
	}
	groups := make(map[string]*categoryBucket)
	categoryOrder := []string{} // 保持类别出现顺序
	for _, cfg := range configs {
		if _, ok := groups[cfg.CategoryCode]; !ok {
			groups[cfg.CategoryCode] = &categoryBucket{info: cfg}
			categoryOrder = append(categoryOrder, cfg.CategoryCode)
		}
		groups[cfg.CategoryCode].parts = append(groups[cfg.CategoryCode].parts, cfg)
	}

	// 2. 逐类别计算 Kcᵢ
	result := KcResult{Categories: make([]CategoryScore, 0, len(categoryOrder))}
	totalKc := 0.0
	for _, code := range categoryOrder {
		g := groups[code]
		kcI := 0.0
		itemsScore := make([]ItemScore, 0, len(g.parts))
		for _, part := range g.parts {
			// 用户未提交该条目时，视为"正常"（不影响整体）
			status, ok := statusMap[part.ItemCode]
			if !ok {
				status = model.ItemStatusNormal
			}
			score := status.Score()
			kcI += part.ItemWeight * score
			itemsScore = append(itemsScore, ItemScore{
				ItemCode:   part.ItemCode,
				ItemName:   part.ItemName,
				ItemWeight: part.ItemWeight,
				Status:     status,
				Score:      score,
			})
		}
		// 类别内部所有条目权重和应等于 1.0，但因四舍五入可能略有偏差
		// 这里不做归一化处理，保持与方案一致：wcᵢ × Kcᵢ
		result.Categories = append(result.Categories, CategoryScore{
			CategoryCode:   g.info.CategoryCode,
			CategoryName:   g.info.CategoryName,
			CategoryWeight: g.info.CategoryWeight,
			KCategory:      kcI,
			Items:          itemsScore,
		})
		totalKc += g.info.CategoryWeight * kcI
	}

	// 3. 全局开关修正：能否行驶 / 液压功能
	// 方案中未明确给出两个开关的折损规则，按行业经验：
	//   不能行驶：Kc 再乘以 0.7（车辆无法使用，残值大打折扣）
	//   液压故障：Kc 再乘以 0.8（核心功能失效）
	if !canDrive {
		totalKc *= 0.7
	}
	if !hydraulicOK {
		totalKc *= 0.8
	}

	// 4. 兜底边界
	if totalKc < 0 {
		totalKc = 0
	}
	if totalKc > 1 {
		totalKc = 1
	}
	result.KCondition = totalKc

	// 类别按编码排序（保证输出稳定）
	sort.SliceStable(result.Categories, func(i, j int) bool {
		return result.Categories[i].CategoryCode < result.Categories[j].CategoryCode
	})

	return result, nil
}
