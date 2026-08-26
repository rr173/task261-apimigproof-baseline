package service

import (
	"path/filepath"
	"testing"
	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/store"
)

func TestBug09EquivalentNumericDefaultsStayCompatible(t *testing.T) {
	st, err := store.OpenStore(filepath.Join(t.TempDir(), "defaults.db")); if err != nil { t.Fatal(err) }
	defer st.Close()
	svc := New(st)
	from, _ := svc.Reg.CreateContract("orders", 1); to, _ := svc.Reg.CreateContract("orders", 2)
	oldDefault, newDefault := "3.0", "3"
	for _, f := range []model.FieldSemantics{{ContractID:from.ID,FieldID:"retry",Status:model.FieldValid,ValueType:model.TypeFloat,HasDefault:true,DefaultValue:&oldDefault},{ContractID:to.ID,FieldID:"retry",Status:model.FieldValid,ValueType:model.TypeFloat,HasDefault:true,DefaultValue:&newDefault}} { if _, err := svc.Reg.AddField(f); err != nil { t.Fatal(err) } }
	if err := st.SetContractStatus(from.ID, model.ContractSealed); err != nil { t.Fatal(err) }
	if err := st.SetContractStatus(to.ID, model.ContractPendingCompare); err != nil { t.Fatal(err) }
	if _, _, err := st.ImportSample(`{}`); err != nil { t.Fatal(err) }
	comp, err := svc.RunComparison(from.ID, to.ID); if err != nil { t.Fatal(err) }
	if comp.Status != model.ComparisonCompatible || comp.Migratable != 1 { t.Fatalf("comparison=%#v, want compatible/1", comp) }
}
