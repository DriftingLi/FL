// 本文件：讲师**有效能力集**的出口（#2599 / 规格 E3、F1）。
//
// 为什么独立成文件而不并进 /auth/me：与 admin 的同名端点同一条理由 —— /auth/me 的响应形状
// 被契约测试锁住且服务四种角色，而「讲师端能力集」只对讲师有意义；塞进去会让四端共用的形状
// 多出一个恒空的字段。资料面与权限装载有各自的状态机（「装载失败」要能与「资料没补齐」区分）。
package tutor

import (
	"sort"

	"forklift-training/internal/authz"
)

// TutorCapabilities 返回该讲师的有效能力集（#2599 / 规格 E3）：转调 tutorcap 解析器，
// 语义（缓存 TTL、未授权与故障的分档）见该包注释。供装配根注入中间件用。
func (s *Service) TutorCapabilities(tutorID int) (map[authz.Capability]struct{}, bool, error) {
	return s.caps.TutorCapabilities(tutorID)
}

// TutorCapabilitySet 当前讲师的有效能力集（端点出口形态，能力键按字典序稳定输出）。
//
// 排序不是装饰：前端把这份清单参与路由守卫与侧栏过滤，稳定序让「同一权限的两个账号」
// 拿到逐字相同的响应，也让契约测试能按字面量断言。
func (s *Service) TutorCapabilitySet(tutorID int) (*TutorCapabilitiesDTO, error) {
	caps, granted, err := s.caps.TutorCapabilities(tutorID)
	if err != nil {
		return nil, err
	}
	list := make([]string, 0, len(caps))
	for c := range caps {
		list = append(list, string(c))
	}
	sort.Strings(list)
	return &TutorCapabilitiesDTO{Capabilities: list, Granted: granted}, nil
}
