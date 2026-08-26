package transform

import (
	"fmt"

	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/semantics"
)

// Event 记录转换过程中的一次原子动作，供比较阶段取证。
type Event struct {
	Field   string // 旧字段 ID
	ToField string // 新字段 ID
	Action  string // 执行的动作
	Outcome string // 结果：applied / defaulted / dropped / rejected / coerced
	Raw     string // 输入值文本（如存在）
	Value   string // 输出值文本（如存在）
}

// Apply 对样本的一个字段应用转换计划，产出事件与转换后的值。
//
// 返回值：
//   - outRaw：转换后的值文本（空串表示该字段在新契约中无值）；
//   - event：转换取证事件；
//   - rejected：是否命中显式拒绝；
//   - err：coerce 等失败时返回错误（调用方转为 rejected 判定）。
func Apply(plan *Plan, v semantics.Value, rule *model.TransformationRule) (string, *Event, bool, error) {
	ev := &Event{Field: plan.Field, ToField: plan.ToField, Action: plan.Action}
	switch plan.Action {
	case model.ActionKeep, model.ActionRename:
		if !v.Present {
			ev.Outcome = "defaulted"
			return "", ev, false, nil
		}
		ev.Outcome = "applied"
		ev.Raw = v.Raw
		ev.Value = v.Raw
		return v.Raw, ev, false, nil

	case model.ActionCoerce:
		if !v.Present {
			ev.Outcome = "defaulted"
			return "", ev, false, nil
		}
		if rule == nil || rule.CoerceFrom == "" || rule.CoerceTo == "" {
			return "", ev, false, fmt.Errorf("%w: coerce rule for %q lacks type spec",
				model.ErrBadRequest, plan.Field)
		}
		out, err := semantics.CoerceValue(v.Raw, rule.CoerceFrom, rule.CoerceTo)
		if err != nil {
			ev.Outcome = "coerce_failed"
			return "", ev, false, err
		}
		ev.Outcome = "coerced"
		ev.Raw = v.Raw
		ev.Value = out
		return out, ev, false, nil

	case model.ActionDefault:
		if v.Present {
			ev.Outcome = "applied"
			ev.Raw = v.Raw
			ev.Value = v.Raw
			return v.Raw, ev, false, nil
		}
		if rule != nil && rule.DefaultJSON != nil {
			ev.Outcome = "defaulted"
			ev.Value = *rule.DefaultJSON
			return *rule.DefaultJSON, ev, false, nil
		}
		ev.Outcome = "defaulted"
		return "", ev, false, nil

	case model.ActionDrop:
		ev.Outcome = "dropped"
		if v.Present {
			ev.Raw = v.Raw
		}
		return "", ev, false, nil

	case model.ActionReject:
		if v.Present {
			ev.Outcome = "rejected"
			ev.Raw = v.Raw
			return "", ev, true, nil
		}
		ev.Outcome = "skipped"
		return "", ev, false, nil

	default:
		return "", ev, false, fmt.Errorf("%w: unknown action %q", model.ErrBadRequest, plan.Action)
	}
}
