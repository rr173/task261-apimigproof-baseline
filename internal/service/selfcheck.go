package service

import (
	"fmt"

	"task261-apimigproof/internal/model"
)

// SelfCheck 执行内置端到端验收场景（与 REQ-20260825-123 的端到端场景对齐）：
//
//  1. 建契约 orders.v1：auto_renew(bool, 无默认) / api_key(string) / retry(int, 默认 3)；
//  2. 建契约 orders.v2：移除 auto_renew、retry 默认改为 5；
//  3. 导入样本：显式 false 的 auto_renew 与未设置的 retry；
//  4. 比较：v1→v2 判为存在歧义（丢失区分度 + 冲突默认值）；
//  5. 补充兼容规则：auto_renew→rename legacy_auto_renew、retry→default 3；
//  6. 再比较：全部样本可迁移 → compatible；发布迁移证明 → published。
//
// 返回 v1 与 v2 的契约 ID（供调用方做重启恢复验证）。
func (s *Service) SelfCheck() (int64, int64, error) {
	// ---- 1. 契约 v1 ----
	v1, err := s.Reg.CreateContract("orders", 1)
	if err != nil {
		return 0, 0, fmt.Errorf("create v1: %w", err)
	}
	fAutoRenew := s.mustField(v1.ID, "auto_renew", model.TypeBool, model.FieldValid, false, nil)
	if _, err := s.Reg.AddField(fAutoRenew); err != nil {
		return 0, 0, fmt.Errorf("add auto_renew: %w", err)
	}
	if _, err := s.Reg.AddField(s.mustField(v1.ID, "api_key", model.TypeString, model.FieldValid, false, nil)); err != nil {
		return 0, 0, fmt.Errorf("add api_key: %w", err)
	}
	d3 := `3`
	if _, err := s.Reg.AddField(s.mustField(v1.ID, "retry", model.TypeInt, model.FieldValid, true, &d3)); err != nil {
		return 0, 0, fmt.Errorf("add retry: %w", err)
	}
	v1Sealed, err := s.Reg.Seal(v1.ID)
	if err != nil {
		return 0, 0, fmt.Errorf("seal v1: %w", err)
	}
	if v1Sealed.Status != model.ContractSealed {
		return 0, 0, fmt.Errorf("v1 not sealed")
	}

	// 字段冲突防御：同一契约内同名字段声明矛盾语义 → conflict，阻塞封存。
	cm, err := s.Reg.CreateContract("orders-probe", 1)
	if err != nil {
		return 0, 0, fmt.Errorf("create probe: %w", err)
	}
	if _, err := s.Reg.AddField(s.mustField(cm.ID, "mode", model.TypeString, model.FieldValid, false, nil)); err != nil {
		return 0, 0, fmt.Errorf("probe decl1: %w", err)
	}
	confField, err := s.Reg.AddField(s.mustField(cm.ID, "mode", model.TypeInt, model.FieldValid, false, nil))
	if err != nil {
		return 0, 0, fmt.Errorf("probe decl2: %w", err)
	}
	if confField.Status != model.FieldConflict {
		return 0, 0, fmt.Errorf("expected conflict status, got %s", confField.Status)
	}
	if _, err := s.Reg.Seal(cm.ID); err == nil {
		return 0, 0, fmt.Errorf("expected seal to fail on conflict field")
	}

	// ---- 2. 契约 v2 ----
	v2, err := s.Reg.CreateContract("orders", 2)
	if err != nil {
		return 0, 0, fmt.Errorf("create v2: %w", err)
	}
	if _, err := s.Reg.AddField(s.mustField(v2.ID, "api_key", model.TypeString, model.FieldValid, false, nil)); err != nil {
		return 0, 0, fmt.Errorf("add v2 api_key: %w", err)
	}
	d5 := `5`
	if _, err := s.Reg.AddField(s.mustField(v2.ID, "retry", model.TypeInt, model.FieldValid, true, &d5)); err != nil {
		return 0, 0, fmt.Errorf("add v2 retry: %w", err)
	}
	// auto_renew 在 v2 中明确废弃（removed_in=2）——即语义上已移除。
	if _, err := s.Reg.AddField(s.mustField(v2.ID, "auto_renew", model.TypeBool, model.FieldDeprecated, false, nil)); err != nil {
		return 0, 0, fmt.Errorf("add v2 deprecated auto_renew: %w", err)
	}
	if err := s.Store.SetContractStatus(v2.ID, model.ContractPendingCompare); err != nil {
		return 0, 0, fmt.Errorf("mark v2 pending compare: %w", err)
	}

	// ---- 3. 转换规则（目标契约 v2）----
	// 阶段一：auto_renew 显式 drop（模拟未做兼容处理的默认路径）。
	if _, err := s.Store.CreateRule(model.TransformationRule{
		ContractID: v2.ID, FromField: "auto_renew", Action: model.ActionDrop, Precedence: 0,
	}); err != nil {
		return 0, 0, fmt.Errorf("drop rule: %w", err)
	}

	// ---- 4. 样本导入（幂等）----
	payloads := []string{
		`{"auto_renew": false, "api_key": "k-001", "retry": 3}`,
		`{"api_key": "k-002"}`, // retry 未设置：v1 默认 3，v2 默认 5 → 冲突默认值
	}
	for i, pl := range payloads {
		_, added, err := s.Store.ImportSample(pl)
		if err != nil {
			return 0, 0, fmt.Errorf("import sample %d: %w", i, err)
		}
		if !added {
			return 0, 0, fmt.Errorf("import sample %d: expected added", i)
		}
	}
	// 幂等重导。
	if _, added, err := s.Store.ImportSample(payloads[0]); err != nil {
		return 0, 0, err
	} else if added {
		return 0, 0, fmt.Errorf("expected idempotent ignore on duplicate import")
	}

	// ---- 5. 第一次比较：应判 ambiguous ----
	comp1, err := s.RunComparison(v1.ID, v2.ID)
	if err != nil {
		return 0, 0, fmt.Errorf("run comparison 1: %w", err)
	}
	if comp1.Status != model.ComparisonAmbiguous {
		return 0, 0, fmt.Errorf("comparison 1 status = %s, want ambiguous", comp1.Status)
	}
	if comp1.TotalSamples != 2 || comp1.Migratable != 0 || comp1.SemanticsChanged != 2 {
		return 0, 0, fmt.Errorf("comparison 1 stats = t%d/m%d/s%d, want 2/0/2",
			comp1.TotalSamples, comp1.Migratable, comp1.SemanticsChanged)
	}
	issueKinds := map[string]bool{}
	for _, is := range comp1.Issues {
		issueKinds[is.Kind] = true
	}
	if !issueKinds[model.IssueLossOfDistinction] || !issueKinds[model.IssueDefaultConflict] {
		return 0, 0, fmt.Errorf("comparison 1 issues = %v, want loss_of_distinction + default_conflict", issueKinds)
	}

	// ---- 6. 补充兼容规则：保留 auto_renew、固定 retry 旧默认值 ----
	if _, err := s.Store.CreateRule(model.TransformationRule{
		ContractID: v2.ID, FromField: "auto_renew", ToField: "legacy_auto_renew",
		Action: model.ActionRename, Precedence: 1,
	}); err != nil {
		return 0, 0, fmt.Errorf("rename rule: %w", err)
	}
	if _, err := s.Store.CreateRule(model.TransformationRule{
		ContractID: v2.ID, FromField: "retry", ToField: "retry",
		Action: model.ActionDefault, Precedence: 0, DefaultJSON: &d3,
	}); err != nil {
		return 0, 0, fmt.Errorf("default rule: %w", err)
	}

	// ---- 7. 第二次比较：全部可迁移 → compatible ----
	comp2, err := s.RunComparison(v1.ID, v2.ID)
	if err != nil {
		return 0, 0, fmt.Errorf("run comparison 2: %w", err)
	}
	if comp2.Status != model.ComparisonCompatible {
		return 0, 0, fmt.Errorf("comparison 2 status = %s, want compatible", comp2.Status)
	}
	if comp2.Migratable != 2 {
		return 0, 0, fmt.Errorf("comparison 2 migratable = %d, want 2", comp2.Migratable)
	}
	if _, err := s.Reg.Seal(v2.ID); err != nil {
		return 0, 0, fmt.Errorf("seal v2: %w", err)
	}

	// ---- 8. 发布迁移证明 ----
	draft, err := s.Proofs.Create(comp2.ID)
	if err != nil {
		return 0, 0, fmt.Errorf("create proof: %w", err)
	}
	published, err := s.Proofs.Publish(draft.ID)
	if err != nil {
		return 0, 0, fmt.Errorf("publish proof: %w", err)
	}
	if published.Status != model.ProofPublished || published.EvidenceFingerprint == "" {
		return 0, 0, fmt.Errorf("proof status=%s fingerprint empty", published.Status)
	}
	// 封存证明不可修改：再次发布应失败。
	if _, err := s.Proofs.Publish(published.ID); err == nil {
		return 0, 0, fmt.Errorf("expected republish of sealed proof to fail")
	}
	return v1.ID, v2.ID, nil
}

// mustField 构造字段语义声明（测试/自检用辅助）。
func (s *Service) mustField(contractID int64, fieldID, valueType, status string,
	hasDefault bool, defaultVal *string) model.FieldSemantics {
	f := model.FieldSemantics{
		ContractID: contractID, FieldID: fieldID, Status: status,
		ValueType: valueType, HasDefault: hasDefault, DefaultValue: defaultVal,
	}
	if hasDefault && defaultVal != nil {
		f.DefaultValue = defaultVal
	}
	return f
}
