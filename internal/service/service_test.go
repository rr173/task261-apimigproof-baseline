package service

import (
	"os"
	"path/filepath"
	"testing"

	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/store"
)

// TestSemanticsPreservation 验证核心判定算法：
// 缺省字段被新默认值改变可见效果 → 冲突默认值；显式值被丢弃 → 丢失区分度。
func TestSemanticsPreservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	st, err := store.OpenStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		st.Close()
		_ = os.Remove(path)
	}()
	svc := New(st)

	// 旧契约 v1：retry 默认 3；auto_renew 无默认。
	v1, err := svc.Reg.CreateContract("orders", 1)
	if err != nil {
		t.Fatalf("create v1: %v", err)
	}
	d3 := `3`
	if _, err := svc.Reg.AddField(svc.mustField(v1.ID, "retry", model.TypeInt, model.FieldValid, true, &d3)); err != nil {
		t.Fatalf("add retry: %v", err)
	}
	if _, err := svc.Reg.AddField(svc.mustField(v1.ID, "auto_renew", model.TypeBool, model.FieldValid, false, nil)); err != nil {
		t.Fatalf("add auto_renew: %v", err)
	}
	if _, err := svc.Reg.Seal(v1.ID); err != nil {
		t.Fatalf("seal v1: %v", err)
	}

	// 新契约 v2：retry 默认 5；auto_renew 移除。
	v2, err := svc.Reg.CreateContract("orders", 2)
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}
	d5 := `5`
	if _, err := svc.Reg.AddField(svc.mustField(v2.ID, "retry", model.TypeInt, model.FieldValid, true, &d5)); err != nil {
		t.Fatalf("add v2 retry: %v", err)
	}
	if _, err := svc.Reg.AddField(svc.mustField(v2.ID, "auto_renew", model.TypeBool, model.FieldDeprecated, false, nil)); err != nil {
		t.Fatalf("add v2 auto_renew: %v", err)
	}
	if err := st.SetContractStatus(v2.ID, model.ContractPendingCompare); err != nil {
		t.Fatalf("mark v2 pending compare: %v", err)
	}
	// auto_renew 显式 drop（模拟未兼容处理）。
	if _, err := st.CreateRule(model.TransformationRule{
		ContractID: v2.ID, FromField: "auto_renew", Action: model.ActionDrop, Precedence: 0,
	}); err != nil {
		t.Fatalf("drop rule: %v", err)
	}

	// 样本：显式关闭 auto_renew + 未设置 retry。
	if _, added, err := st.ImportSample(`{"auto_renew": false, "retry": 3}`); err != nil || !added {
		t.Fatalf("import s1: added=%v err=%v", added, err)
	}
	if _, added, err := st.ImportSample(`{"auto_renew": true}`); err != nil || !added {
		t.Fatalf("import s2: added=%v err=%v", added, err)
	}

	comp, err := svc.RunComparison(v1.ID, v2.ID)
	if err != nil {
		t.Fatalf("run comparison: %v", err)
	}
	if comp.Status != model.ComparisonAmbiguous {
		t.Fatalf("status = %s, want ambiguous", comp.Status)
	}
	if comp.SemanticsChanged != 2 {
		t.Fatalf("semantics_changed = %d, want 2", comp.SemanticsChanged)
	}
	kinds := map[string]bool{}
	for _, is := range comp.Issues {
		kinds[is.Kind] = true
	}
	if !kinds[model.IssueDefaultConflict] {
		t.Errorf("want default_conflict issue, got %v", kinds)
	}
	if !kinds[model.IssueLossOfDistinction] {
		t.Errorf("want loss_of_distinction issue, got %v", kinds)
	}

	// 补充兼容规则后应全部可迁移。
	if _, err := st.CreateRule(model.TransformationRule{
		ContractID: v2.ID, FromField: "retry", ToField: "retry",
		Action: model.ActionDefault, Precedence: 0, DefaultJSON: &d3,
	}); err != nil {
		t.Fatalf("default rule: %v", err)
	}
	if _, err := st.CreateRule(model.TransformationRule{
		ContractID: v2.ID, FromField: "auto_renew", ToField: "legacy_auto_renew",
		Action: model.ActionRename, Precedence: 1,
	}); err != nil {
		t.Fatalf("rename rule: %v", err)
	}
	comp2, err := svc.RunComparison(v1.ID, v2.ID)
	if err != nil {
		t.Fatalf("run comparison 2: %v", err)
	}
	if comp2.Status != model.ComparisonCompatible || comp2.Migratable != 2 {
		t.Fatalf("comparison 2: status=%s migratable=%d, want compatible/2", comp2.Status, comp2.Migratable)
	}
	if _, err := svc.Reg.Seal(v2.ID); err != nil {
		t.Fatalf("seal v2: %v", err)
	}
}

// TestIdempotentImport 验证样本指纹幂等：重复导入只保留一条。
func TestIdempotentImport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	st, err := store.OpenStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		st.Close()
		_ = os.Remove(path)
	}()
	payload := `{"a": 1, "b": "x"}`
	first, added, err := st.ImportSample(payload)
	if err != nil || !added {
		t.Fatalf("first import: added=%v err=%v", added, err)
	}
	second, added2, err := st.ImportSample(payload)
	if err != nil || added2 {
		t.Fatalf("second import: added=%v err=%v", added2, err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent import mismatch: %d vs %d", first.ID, second.ID)
	}
	n, err := st.CountSamples()
	if err != nil || n != 1 {
		t.Fatalf("sample count = %d err=%v, want 1", n, err)
	}
}

// TestRuleCycleRejected 验证转换循环被拒绝（rename 链成环）。
func TestRuleCycleRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	st, err := store.OpenStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		st.Close()
		_ = os.Remove(path)
	}()
	v, err := st.CreateContract("cycle", 1)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// a → b → a 环。
	if _, err := st.CreateRule(model.TransformationRule{
		ContractID: v.ID, FromField: "a", ToField: "b", Action: model.ActionRename, Precedence: 0,
	}); err != nil {
		t.Fatalf("rule a->b: %v", err)
	}
	if _, err := st.CreateRule(model.TransformationRule{
		ContractID: v.ID, FromField: "b", ToField: "a", Action: model.ActionRename, Precedence: 0,
	}); err != nil {
		t.Fatalf("rule b->a: %v", err)
	}
	svc := New(st)
	if _, _, _, err := svc.LoadViews(v.ID, v.ID); err == nil {
		t.Fatalf("expected rule cycle error, got nil")
	}
}

// TestSealedContractImmutable 验证封存契约不可再修改。
func TestSealedContractImmutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	st, err := store.OpenStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		st.Close()
		_ = os.Remove(path)
	}()
	svc := New(st)
	v, err := svc.Reg.CreateContract("imm", 1)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Reg.AddField(svc.mustField(v.ID, "x", model.TypeString, model.FieldValid, false, nil)); err != nil {
		t.Fatalf("add field: %v", err)
	}
	if _, err := svc.Reg.Seal(v.ID); err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := svc.Reg.AddField(svc.mustField(v.ID, "y", model.TypeString, model.FieldValid, false, nil)); err == nil {
		t.Fatalf("expected sealed contract to reject field add")
	}
}
