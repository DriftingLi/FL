// 本文件把 topicListResp / titles 的**一份**留在 package api。
//
// #1445 批 2 把 forum 域的 10 个契约测试沉到 internal/forum（那边自带一份同名形状），但
// forum_list_contract_test.go / forum_designation_contract_test.go / forum_featured_contract_test.go
// 走 NewRouter(newContractDeps(...)) 全量装配链（ADR-0070 决策 8 要保护的装配面证据），必须留在
// internal/api，它们仍引用本形状。定义逐字来自原 forum_category_contract_test.go（勿改）。
package api

// topicListResp 列表响应（只取本测试关心的字段）。
type topicListResp struct {
	Code int `json:"code"`
	Data struct {
		Total  int64 `json:"total"`
		Topics []struct {
			ID       int64  `json:"id"`
			Title    string `json:"title"`
			Category string `json:"category"`
			// RewardIssued 供 #827 的列表回填断言（该字段是列表契约的一部分）。
			RewardIssued bool `json:"reward_issued"`
		} `json:"topics"`
	} `json:"data"`
}

// titles 返回列表里的标题集合，便于做包含/排除断言。
func (r topicListResp) titles() map[string]bool {
	out := make(map[string]bool, len(r.Data.Topics))
	for _, tp := range r.Data.Topics {
		out[tp.Title] = true
	}
	return out
}
