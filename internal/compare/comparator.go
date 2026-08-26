// Package compare 实现语义保持性比较——本服务的核心算法。
//
// 对每个请求样本，比较器构造「旧可见效果 → 新可见效果」的转换并逐字段比对。
// 判定类别：
//
//	equivalence        可见效果完全一致（可迁移）
//	loss_of_distinction旧客户端显式表达的信息在新契约中无处安放
//	default_conflict   未设置字段被新默认值改变了可见效果
//	rule_rejected      命中显式拒绝规则
//	coerce_failure     强转失败
//	unknown_field      样本携带契约外的未知字段
//
// 契约对级结论：全部样本 equivalence → compatible；出现任何非等价样本 → ambiguous。
package compare

import (
	"fmt"
	"sort"

	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/semantics"
	"task261-apimigproof/internal/transform"
)

// Verdict 是单样本判定结果。
type Verdict struct {
	SampleID    int64
	Fingerprint string
	Verdict     string // model.SampleMigratable / SampleRejected / SampleSemanticsChanged
	Issue       string // IssueKind 或空
	Detail      string // 人类可读原因
	Issues      []model.Issue
}

// Comparator 封装一次比较所需的全部输入。
type Comparator struct {
	FromView *semantics.FieldView
	ToView   *semantics.FieldView
	Engine   *transform.Engine
	Policy   string // 兼容窗口策略（默认 transform）
}

// NewComparator 构造比较器。
func NewComparator(from, to *semantics.FieldView, eng *transform.Engine, policy string) *Comparator {
	if policy == "" {
		policy = model.PolicyTransform
	}
	return &Comparator{FromView: from, ToView: to, Engine: eng, Policy: policy}
}

// Evaluate 对单个样本执行语义保持性判定。
func (c *Comparator) Evaluate(sm model.RequestSample) (*Verdict, error) {
	payload, err := semantics.ParsePayload(sm.PayloadJSON)
	if err != nil {
		return nil, err
	}
	// 未知字段检测：样本携带的字段若旧契约与转换规则都不认识，属未知字段。
	var unknown []string
	for fieldID := range payload.Values {
		if _, ok := c.FromView.Get(fieldID); !ok {
			if c.Engine.RuleFor(fieldID) == nil {
				unknown = append(unknown, fieldID)
			}
		}
	}
	sort.Strings(unknown)

	v := &Verdict{
		SampleID: sm.ID, Fingerprint: sm.Fingerprint,
		Verdict: model.SampleMigratable, Issue: model.IssueEquivalence,
	}
	for _, u := range unknown {
		v.Issues = append(v.Issues, model.Issue{
			SampleID: sm.ID, Kind: model.IssueUnknownField, Field: u,
			Message: fmt.Sprintf("sample carries field %q unknown to both contracts and rules", u),
		})
	}

	// 逐字段比较（按旧契约字段定义顺序保证确定性）。
	fields := sortedFields(c.FromView)
	for _, f := range fields {
		if f.Status == model.FieldConflict {
			continue // 冲突字段应已被契约注册阻塞，防御性跳过
		}
		if err := c.evaluateField(payload, f, v); err != nil {
			return nil, err
		}
	}

	// 汇总：有语义改变 → semantics_changed；有拒绝 → rejected；否则 migratable。
	if len(v.Issues) > 0 {
		for _, is := range v.Issues {
			if is.Kind == model.IssueLossOfDistinction || is.Kind == model.IssueDefaultConflict ||
				is.Kind == model.IssueUnknownField || is.Kind == model.IssueCoerceFailure {
				v.Verdict = model.SampleSemanticsChanged
				v.Issue = is.Kind
				v.Detail = firstIssueMessage(v.Issues)
				return v, nil
			}
		}
		for _, is := range v.Issues {
			if is.Kind == model.IssueRuleRejected {
				v.Verdict = model.SampleRejected
				v.Issue = model.IssueRuleRejected
				v.Detail = firstIssueMessage(v.Issues)
				return v, nil
			}
		}
	}
	v.Detail = "all fields preserve observable semantics across versions"
	return v, nil
}

// evaluateField 对单个旧契约字段执行转换与语义比对。
func (c *Comparator) evaluateField(payload *semantics.Payload, f model.FieldSemantics, v *Verdict) error {
	val, present := payload.Get(f.FieldID)
	if c.Policy == model.PolicyReject && present {
		v.Issues = append(v.Issues, model.Issue{
			SampleID: v.SampleID, Kind: model.IssueRuleRejected, Field: f.FieldID,
			Message: fmt.Sprintf("field %q is rejected by compatibility window policy", f.FieldID),
		})
		return nil
	}
	plan := c.Engine.PlanFor(f.FieldID, present)
	rule := c.Engine.RuleFor(f.FieldID)

	// 1. 拒绝路径。
	if plan.Rejected {
		v.Issues = append(v.Issues, model.Issue{
			SampleID: v.SampleID, Kind: model.IssueRuleRejected, Field: f.FieldID,
			Message: fmt.Sprintf("field %q is rejected by policy %s (value present)", f.FieldID, c.Policy),
		})
		return nil
	}

	// 2. 应用转换。
	outRaw, _, _, err := transform.Apply(plan, val, rule)
	if err != nil {
		v.Issues = append(v.Issues, model.Issue{
			SampleID: v.SampleID, Kind: model.IssueCoerceFailure, Field: f.FieldID,
			Message: err.Error(),
		})
		return nil
	}

	// 3. 旧可见效果。
	oldEff := semantics.EffectiveValue(val, f.HasDefault, f.DefaultValue)

	// 4. 新可见效果：定位目标字段。
	targetField, targetOK := c.lookupTarget(plan, f)
	switch {
	case plan.Action == model.ActionDrop || plan.Action == model.ActionReject:
		// 字段被丢弃：若旧样本中该字段显式存在（有信息量）→ 丢失区分度；
		// 若旧样本未设置且旧契约有默认 → 默认值可能丢失，同样记录。
		if val.Present {
			v.Issues = append(v.Issues, model.Issue{
				SampleID: v.SampleID, Kind: model.IssueLossOfDistinction, Field: f.FieldID,
				Message: fmt.Sprintf("field %q (value=%s) is dropped in the new contract; explicit value cannot be represented",
					f.FieldID, oldEff),
			})
		} else if f.HasDefault && f.DefaultValue != nil {
			v.Issues = append(v.Issues, model.Issue{
				SampleID: v.SampleID, Kind: model.IssueLossOfDistinction, Field: f.FieldID,
				Message: fmt.Sprintf("field %q default %s is dropped in the new contract",
					f.FieldID, *f.DefaultValue),
			})
		}
		return nil

	case !targetOK:
		// 无目标字段且未被规则接管：显式值无处安放 → 丢失区分度。
		if val.Present {
			v.Issues = append(v.Issues, model.Issue{
				SampleID: v.SampleID, Kind: model.IssueLossOfDistinction, Field: f.FieldID,
				Message: fmt.Sprintf("field %q (value=%s) has no target field or rule in the new contract",
					f.FieldID, oldEff),
			})
		}
		return nil
	}

	// 5. 目标字段的可见效果。
	newEff := newEffectiveValue(plan, outRaw, val, targetField)

	// 6. 语义等价判定。
	if oldEff != newEff {
		kind := classifyMismatch(f, targetField, val, plan, oldEff, newEff)
		msg := fmt.Sprintf("field %q observable semantics change: %s -> %s", f.FieldID, oldEff, newEff)
		if kind == model.IssueDefaultConflict {
			msg += fmt.Sprintf(" (new default %q differs from old default %q)",
				defaultOrNil(targetField), defaultOrNil(f))
		}
		v.Issues = append(v.Issues, model.Issue{
			SampleID: v.SampleID, Kind: kind, Field: f.FieldID, Message: msg,
		})
	}
	return nil
}

// lookupTarget 解析转换计划的目标字段。
// rename/coerce 规则即使目标字段未在新契约中声明，也视为被规则接管
// （值安放于规则指定的字段名下，无契约默认值）。
func (c *Comparator) lookupTarget(plan *transform.Plan, f model.FieldSemantics) (model.FieldSemantics, bool) {
	to := plan.ToField
	if to == "" {
		to = f.FieldID
	}
	if tf, ok := c.ToView.Get(to); ok {
		return tf, true
	}
	if plan.Action == model.ActionRename || plan.Action == model.ActionCoerce {
		return model.FieldSemantics{FieldID: to, ValueType: f.ValueType}, true
	}
	return model.FieldSemantics{}, false
}

// newEffectiveValue 计算转换后目标字段的可见效果。
func newEffectiveValue(plan *transform.Plan, outRaw string, val semantics.Value, target model.FieldSemantics) string {
	if plan.Action == model.ActionDefault && plan.Defaulted {
		// 补默认值：优先规则声明的默认，其次目标契约默认。
		if outRaw != "" {
			return "D:" + semantics.NormalizeForCompare(outRaw)
		}
		if target.HasDefault && target.DefaultValue != nil {
			return "D:" + semantics.NormalizeForCompare(*target.DefaultValue)
		}
		return "ABSENT"
	}
	if !val.Present && plan.Action != model.ActionDefault {
		// 未设置且走 keep/rename/coerce：新侧效果 = 目标契约默认（若有）。
		if target.HasDefault && target.DefaultValue != nil {
			return "D:" + *target.DefaultValue
		}
		return "ABSENT"
	}
	// 显式 null 的在场性与值需保持：旧侧 EffectiveValue 对显式 null
	// 产出 "NULL"，新侧同样以 "NULL" 表达，从而与未设置（ABSENT）及
	// 显式字符串值（"S:..."）区分。否则显式 null 经 keep/rename/coerce
	// 后会被误报为字符串值 "null"，在新旧契约语义相同时误判为语义改变。
	if val.Null {
		return "NULL"
	}
	if outRaw == "" {
		return "ABSENT"
	}
	return "S:" + semantics.NormalizeForCompare(outRaw)
}

// classifyMismatch 对不匹配归类：优先默认值冲突，其次丢失区分度。
func classifyMismatch(oldF, newF model.FieldSemantics, val semantics.Value,
	plan *transform.Plan, oldEff, newEff string) string {
	// 未设置场景：旧默认 vs 新默认不一致 → default_conflict。
	if !val.Present {
		if oldF.HasDefault && newF.HasDefault &&
			oldF.DefaultValue != nil && newF.DefaultValue != nil {
			return model.IssueDefaultConflict
		}
		// 旧「未设置」→ 新「显式默认」（如 bool 默认 false）：区分语义被改变。
		if semantics.HasDistinctZeroSemantics(newF) {
			return model.IssueDefaultConflict
		}
		return model.IssueDefaultConflict
	}
	// 显式设置场景：值被转换/丢失 → 丢失区分度或默认冲突。
	if plan.Defaulted {
		return model.IssueDefaultConflict
	}
	return model.IssueLossOfDistinction
}

// sortedFields 按字段 ID 排序，保证比较顺序确定。
func sortedFields(fv *semantics.FieldView) []model.FieldSemantics {
	out := make([]model.FieldSemantics, 0, len(fv.ByID))
	for _, f := range fv.ByID {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FieldID < out[j].FieldID })
	return out
}

// defaultOrNil 返回字段默认值文本；无默认返回 "(none)"。
func defaultOrNil(f model.FieldSemantics) string {
	if f.HasDefault && f.DefaultValue != nil {
		return *f.DefaultValue
	}
	return "(none)"
}

// firstIssueMessage 返回第一条问题的消息。
func firstIssueMessage(issues []model.Issue) string {
	if len(issues) == 0 {
		return ""
	}
	return issues[0].Message
}
