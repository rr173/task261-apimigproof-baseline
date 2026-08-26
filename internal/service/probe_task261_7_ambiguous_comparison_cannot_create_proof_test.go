package service

import (
	"errors"
	"path/filepath"
	"testing"
	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/store"
)

func TestBug07AmbiguousComparisonCannotCreateProof(t *testing.T) {
	st, err := store.OpenStore(filepath.Join(t.TempDir(), "proof.db")); if err != nil { t.Fatal(err) }
	defer st.Close()
	svc := New(st)
	from, _ := svc.Reg.CreateContract("orders", 1); to, _ := svc.Reg.CreateContract("orders", 2)
	if _, err := svc.Reg.AddField(model.FieldSemantics{ContractID:from.ID, FieldID:"legacy", Status:model.FieldValid, ValueType:model.TypeBool}); err != nil { t.Fatal(err) }
	if err := st.SetContractStatus(from.ID, model.ContractSealed); err != nil { t.Fatal(err) }
	if err := st.SetContractStatus(to.ID, model.ContractPendingCompare); err != nil { t.Fatal(err) }
	if _, _, err := st.ImportSample(`{"legacy":true}`); err != nil { t.Fatal(err) }
	comp, err := svc.RunComparison(from.ID, to.ID); if err != nil { t.Fatal(err) }
	if comp.Status != model.ComparisonAmbiguous { t.Fatalf("status=%s, want ambiguous", comp.Status) }
	if _, err := svc.Proofs.Create(comp.ID); err == nil || !errors.Is(err, model.ErrBadRequest) { t.Fatalf("ambiguous proof creation err=%v, want bad request", err) }
	proofs, err := st.ListProofs(); if err != nil || len(proofs) != 0 { t.Fatalf("proofs=%#v err=%v, want none", proofs, err) }
}
