package service

import (
	"errors"
	"path/filepath"
	"testing"
	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/store"
)

func TestBug06FailedComparisonDoesNotStickRunning(t *testing.T) {
	st, err := store.OpenStore(filepath.Join(t.TempDir(), "compare.db")); if err != nil { t.Fatal(err) }
	defer st.Close()
	svc := New(st)
	from, _ := svc.Reg.CreateContract("orders", 1); to, _ := svc.Reg.CreateContract("orders", 2)
	for _, c := range []model.ContractVersion{from, to} { if _, err := svc.Reg.AddField(model.FieldSemantics{ContractID:c.ID, FieldID:"x", Status:model.FieldValid, ValueType:model.TypeString}); err != nil { t.Fatal(err) } }
	if err := st.SetContractStatus(from.ID, model.ContractSealed); err != nil { t.Fatal(err) }
	if err := st.SetContractStatus(to.ID, model.ContractPendingCompare); err != nil { t.Fatal(err) }
	a, err := st.CreateRule(model.TransformationRule{ContractID:to.ID, FromField:"x", ToField:"y", Action:model.ActionRename}); if err != nil { t.Fatal(err) }
	if _, err := st.CreateRule(model.TransformationRule{ContractID:to.ID, FromField:"y", ToField:"x", Action:model.ActionRename}); err != nil { t.Fatal(err) }
	if _, err := svc.RunComparison(from.ID, to.ID); err == nil { t.Fatal("cyclic comparison unexpectedly succeeded") }
	comparisons, err := st.ListComparisons(); if err != nil { t.Fatal(err) }
	if len(comparisons) != 0 { t.Fatalf("failed comparison left rows: %#v", comparisons) }
	if err := st.DeleteRule(a.ID); err != nil { t.Fatal(err) }
	if _, err := svc.RunComparison(from.ID, to.ID); err != nil && errors.Is(err, model.ErrCompareRunning) { t.Fatalf("retry blocked by stale running task: %v", err) }
}
