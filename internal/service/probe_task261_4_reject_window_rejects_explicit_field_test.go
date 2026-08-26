package service

import (
	"path/filepath"
	"testing"
	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/store"
)

func TestBug04RejectWindowRejectsExplicitField(t *testing.T) {
	st, err := store.OpenStore(filepath.Join(t.TempDir(), "compare.db"))
	if err != nil { t.Fatal(err) }
	defer st.Close()
	svc := New(st)
	from, err := svc.Reg.CreateContract("orders", 1); if err != nil { t.Fatal(err) }
	to, err := svc.Reg.CreateContract("orders", 2); if err != nil { t.Fatal(err) }
	for _, c := range []model.ContractVersion{from, to} {
		if _, err := svc.Reg.AddField(model.FieldSemantics{ContractID:c.ID, FieldID:"legacy", Status:model.FieldValid, ValueType:model.TypeString}); err != nil { t.Fatal(err) }
	}
	if err := st.SetContractStatus(from.ID, model.ContractSealed); err != nil { t.Fatal(err) }
	if err := st.SetContractStatus(to.ID, model.ContractPendingCompare); err != nil { t.Fatal(err) }
	if _, err := svc.Window.Declare(from.ID, to.ID, model.PolicyReject, "", nil); err != nil { t.Fatal(err) }
	if _, _, err := st.ImportSample(`{"legacy":"old"}`); err != nil { t.Fatal(err) }
	comp, err := svc.RunComparison(from.ID, to.ID)
	if err != nil { t.Fatal(err) }
	if comp.Rejected != 1 || comp.SemanticsChanged != 0 || comp.Results[0].Verdict != model.SampleRejected { t.Fatalf("comparison=%#v, want one rejected sample", comp) }
}
