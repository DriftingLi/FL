// 本文件：积分商城商品目录的管理端读写面（#2548 / #2512 G-01 / N1）。
//
// 商品目录是可运营内容：改价、上下架、换图、改文案走 /api/admin/points/shop/items，
// 不靠发版（与 ADR-0022「管理员调价即全卷调价」的既有口径一致）。
// 「新增可兑 SKU」的登记路径是两步，缺一不可：
//  1. 在 sku_registry.go 的 shopSKUDecls 登记**权益读者**（EntitlementSKU / ReadBy；纯价格行标 Blocked）；
//  2. 经本文件的管理端 PUT 建/改商品行（enabled=true）。
//
// 只有第 2 步而没有第 1 步的行会被 ListShopItems 过滤掉（列表里看不见），所以 upsert
// 对「未登记就上架」直接拒（ErrShopSKUUnregistered），避免运营建出一行永远不出现的商品。
package points

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm/clause"

	"forklift-training/internal/model"
)

// 商品分类四值（#2548 票面 enum；与移动端筛选档 book|material|course|other 同一词表）。
const (
	shopCategoryBook     = "book"
	shopCategoryMaterial = "material"
	shopCategoryCourse   = "course"
	shopCategoryOther    = "other"
)

// isShopCategory 分类值域的单点判据（列表筛选与管理端写入同取此处）。
func isShopCategory(v string) bool {
	switch v {
	case shopCategoryBook, shopCategoryMaterial, shopCategoryCourse, shopCategoryOther:
		return true
	default:
		return false
	}
}

// 管理端商品目录的具名哨兵（ErrStatus 统一映射 400；文案给运营可执行的下一步）。
var (
	ErrShopCategoryInvalid   = errors.New("商品分类非法：仅支持 book|material|course|other")
	ErrShopSKUUnregistered   = errors.New("该 SKU 未在 sku 对账表登记权益读者：上架也不会出现在在售列表；请先在 shopSKUDecls 登记（或保持下架）")
	ErrShopPriceSourcePinned = errors.New("unlock_real_paper 是全部真题卷价的事实源，不能下架（下架会让卷价退回硬编码兜底值）")
	ErrShopItemInvalid       = errors.New("商品参数非法：title 不能为空、price 必须大于 0、stock 不得为负")
)

// AdminShopItemDTO 管理端商品行（含下架行与不可兑的纯价格行）。
type AdminShopItemDTO struct {
	SKU         string  `json:"sku"`
	Title       string  `json:"title"`
	Price       int     `json:"price"`
	Stock       *int    `json:"stock" extensions:"x-nullable"`
	Enabled     bool    `json:"enabled"`
	ImageURL    *string `json:"image_url" extensions:"x-nullable"`
	Description *string `json:"description" extensions:"x-nullable"`
	Category    string  `json:"category"`
	SortWeight  int     `json:"sort_weight"`
	// Redeemable = 该 SKU 在 sku 对账表里有真读者（与在售列表过滤、兑换放行同源）：
	// false 的行要么是纯价格事实源（unlock_real_paper），要么是漏登记的商品（上架也不在售）。
	Redeemable bool `json:"redeemable"`
}

// AdminShopItemsResult 管理端商品目录（含下架行）。
type AdminShopItemsResult struct {
	Items []AdminShopItemDTO `json:"items" nullability:"nonnil"`
}

// AdminShopItemInput 管理端建/改商品行的入参（PUT 全量替换语义）。
type AdminShopItemInput struct {
	Title       string
	Price       int
	Stock       *int
	Enabled     bool
	ImageURL    *string
	Description *string
	Category    string
	SortWeight  int
}

// AdminListShopItems 管理端目录：**含下架行**（运营要能看到并重新上架），按目录排序。
func (s *Service) AdminListShopItems(ctx context.Context) (*AdminShopItemsResult, error) {
	var rows []model.PointsShopItem
	if err := s.db.WithContext(ctx).Order("sort_weight DESC, price ASC, sku ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]AdminShopItemDTO, 0, len(rows))
	for i := range rows {
		items = append(items, adminShopItemDTO(rows[i]))
	}
	return &AdminShopItemsResult{Items: items}, nil
}

// AdminUpsertShopItem 建/改一行商品（sku 是主键，故 PUT 幂等：存在即全量替换）。
func (s *Service) AdminUpsertShopItem(ctx context.Context, sku string, in AdminShopItemInput) (*AdminShopItemDTO, error) {
	sku = strings.TrimSpace(sku)
	title := strings.TrimSpace(in.Title)
	if sku == "" || len(sku) > 64 || title == "" || in.Price <= 0 || (in.Stock != nil && *in.Stock < 0) {
		return nil, ErrShopItemInvalid
	}
	category := strings.TrimSpace(in.Category)
	if category == "" {
		category = shopCategoryOther
	}
	if !isShopCategory(category) {
		return nil, ErrShopCategoryInvalid
	}
	// 价格事实源必须保持 enabled=true：RealPaperPrice 只读 enabled 行，下架它等于让全卷价
	// 静默退回硬编码兜底（sku_registry.go 头注的同一条血账）。
	if sku == realPaperUnlockSKU && !in.Enabled {
		return nil, ErrShopPriceSourcePinned
	}
	if in.Enabled {
		if _, declared := shopSKUDecls[sku]; !declared {
			return nil, ErrShopSKUUnregistered
		}
	}
	row := model.PointsShopItem{
		SKU:         sku,
		Title:       title,
		Price:       in.Price,
		Stock:       in.Stock,
		Enabled:     in.Enabled,
		ImageURL:    normalizeShopText(in.ImageURL),
		Description: normalizeShopText(in.Description),
		Category:    category,
		SortWeight:  in.SortWeight,
	}
	// ON CONFLICT 让「新建」与「改价/上下架」共用一个幂等入口（同 sku 并发 PUT 不会主键冲突）。
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "sku"}},
		UpdateAll: true,
	}).Create(&row).Error; err != nil {
		return nil, err
	}
	out := adminShopItemDTO(row)
	return &out, nil
}

// normalizeShopText 空串与纯空白视为「没填」→ NULL（与列的可空语义一致，避免前端收到 ""）。
func normalizeShopText(v *string) *string {
	if v == nil {
		return nil
	}
	t := strings.TrimSpace(*v)
	if t == "" {
		return nil
	}
	return &t
}

// adminShopItemDTO 行 → 管理端 DTO；Redeemable 与在售列表/兑换同取对账表。
func adminShopItemDTO(row model.PointsShopItem) AdminShopItemDTO {
	_, _, err := shopRedeemEntitlementKey(row.SKU)
	return AdminShopItemDTO{
		SKU:         row.SKU,
		Title:       row.Title,
		Price:       row.Price,
		Stock:       row.Stock,
		Enabled:     row.Enabled,
		ImageURL:    row.ImageURL,
		Description: row.Description,
		Category:    row.Category,
		SortWeight:  row.SortWeight,
		Redeemable:  err == nil,
	}
}
