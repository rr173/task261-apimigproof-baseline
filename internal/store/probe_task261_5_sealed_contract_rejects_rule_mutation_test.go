package store

import (
	"errors"
	"path/filepath"
	"testing"
	"task261-apimigproof/internal/model"
)

func TestBug05SealedContractRejectsRuleMutation(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "rules.db")); if err != nil { t.Fatal(err) }
	defer st.Close()
	c, err := st.CreateContract("orders", 1); if err != nil { t.Fatal(err) }
	if err := st.SealContract(c.ID); err != nil { t.Fatal(err) }
	if _, err := st.CreateRule(model.TransformationRule{ContractID:c.ID, FromField:"legacy", Action:model.ActionDrop}); !errors.Is(err, model.ErrSealedImmutable) { t.Fatalf("sealed rule mutation err=%v, want sealed immutable", err) }
	n, err := st.CountRules(c.ID); if err != nil || n != 0 { t.Fatalf("rule count=%d err=%v, want 0", n, err) }
}
