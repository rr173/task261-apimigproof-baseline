// Package transform 实现转换规则引擎：把旧契约字段映射到新契约字段。
//
// 职责：
//  1. 规则索引：按 from_field 分组并按 precedence 排序；
//  2. 循环检测：rename/coerce 链不得成环（图 DFS）；
//  3. 转换路径构造：对单个字段确定「去向」——保留、改名、强转、补默认、
//     丢弃或拒绝；
//  4. 值转换：执行 coerce 等原子动作。
package transform

import (
	"fmt"
	"sort"

	"task261-apimigproof/internal/model"
)

// Engine 是转换规则引擎。
type Engine struct {
	contractID int64
	byFrom     map[string][]model.TransformationRule
}

// Store 定义 Engine 依赖的规则存储接口。
type Store interface {
	ListRules(contractID int64) ([]model.TransformationRule, error)
}

// NewEngine 加载契约的全部规则并建索引。
func NewEngine(contractID int64, s Store) (*Engine, error) {
	rules, err := s.ListRules(contractID)
	if err != nil {
		return nil, err
	}
	e := &Engine{
		contractID: contractID,
		byFrom:     make(map[string][]model.TransformationRule),
	}
	for _, r := range rules {
		e.byFrom[r.FromField] = append(e.byFrom[r.FromField], r)
	}
	for k := range e.byFrom {
		sort.SliceStable(e.byFrom[k], func(i, j int) bool {
			return e.byFrom[k][i].Precedence < e.byFrom[k][j].Precedence
		})
	}
	return e, nil
}

// RuleFor 返回字段对应的生效规则；无规则返回 nil。
// 同一 from 字段可注册多条规则（如先 drop 后 rename 覆盖），
// precedence 数值越大优先级越高（后声明的规则用于覆盖默认行为）。
func (e *Engine) RuleFor(field string) *model.TransformationRule {
	rules := e.byFrom[field]
	if len(rules) == 0 {
		return nil
	}
	return &rules[len(rules)-1]
}

// HasRules 报告契约是否注册了任何转换规则。
func (e *Engine) HasRules() bool { return len(e.byFrom) > 0 }

// FieldCount 报告参与转换的字段数。
func (e *Engine) FieldCount() int { return len(e.byFrom) }

// DetectCycle 检测 rename/coerce 链是否成环。
// 图节点 = 字段；边 = rename 或 coerce 规则（from → to）。存在环即返回错误。
func (e *Engine) DetectCycle() error {
	const (
		white = 0 // 未访问
		gray  = 1 // 访问中（在递归栈内）
		black = 2 // 完成
	)
	color := make(map[string]int, e.FieldCount())
	var dfs func(field string, path []string) error
	dfs = func(field string, path []string) error {
		color[field] = gray
		path = append(path, field)
		rule := e.RuleFor(field)
		if rule != nil && (rule.Action == model.ActionRename || rule.Action == model.ActionCoerce) &&
			rule.ToField != "" && rule.ToField != field {
			next := rule.ToField
			switch color[next] {
			case gray:
				return fmt.Errorf("%w: rename chain %v -> %s", model.ErrRuleCycle, path, next)
			case white:
				if err := dfs(next, path); err != nil {
					return err
				}
			}
		}
		color[field] = black
		return nil
	}
	for field := range e.byFrom {
		if color[field] == white {
			if err := dfs(field, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// Plan 描述一个字段的转换路径决策。
type Plan struct {
	Field     string // 旧字段 ID
	ToField   string // 新字段 ID（空 = 无去向）
	Action    string // 最终动作
	Defaulted bool   // 是否因缺失而补了默认值
	Rejected  bool   // 是否命中 reject
}

// PlanFor 为单个字段构造转换路径。
// 该方法是比较阶段的核心决策点：决定字段值在新版本中的去向。
func (e *Engine) PlanFor(field string, present bool) *Plan {
	rule := e.RuleFor(field)
	if rule == nil {
		// 无规则：字段原样保留（keep 语义）。
		return &Plan{Field: field, ToField: field, Action: model.ActionKeep}
	}
	switch rule.Action {
	case model.ActionReject:
		// 命中 reject 规则：仅当字段在样本中显式出现才算拒绝。
		if present {
			return &Plan{Field: field, ToField: "", Action: model.ActionReject, Rejected: true}
		}
		return &Plan{Field: field, ToField: rule.ToField, Action: model.ActionReject}
	case model.ActionKeep:
		return &Plan{Field: field, ToField: rule.ToField, Action: model.ActionKeep}
	case model.ActionRename:
		return &Plan{Field: field, ToField: rule.ToField, Action: model.ActionRename}
	case model.ActionCoerce:
		return &Plan{Field: field, ToField: rule.ToField, Action: model.ActionCoerce}
	case model.ActionDefault:
		return &Plan{Field: field, ToField: rule.ToField, Action: model.ActionDefault,
			Defaulted: !present}
	case model.ActionDrop:
		return &Plan{Field: field, ToField: "", Action: model.ActionDrop}
	default:
		return &Plan{Field: field, ToField: rule.ToField, Action: model.ActionKeep}
	}
}
