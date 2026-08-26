package proof

import (
	"testing"
	"task261-apimigproof/internal/model"
)

type bug08Store struct { proofs map[int64]model.MigrationProof; superseded bool }
func (s *bug08Store) CreateProof(int64,int64,int64,string,string)(model.MigrationProof,error){ panic("unused") }
func (s *bug08Store) GetProof(id int64)(model.MigrationProof,error){ p,ok:=s.proofs[id]; if !ok { return model.MigrationProof{}, model.ErrNotFound }; return p,nil }
func (s *bug08Store) ListProofs()([]model.MigrationProof,error){ panic("unused") }
func (s *bug08Store) PublishProof(int64)(model.MigrationProof,error){ panic("unused") }
func (s *bug08Store) SupersedeProof(oldID,newID int64)(model.MigrationProof,error){ s.superseded=true; p:=s.proofs[oldID]; p.Status=model.ProofSuperseded; p.SupersededBy=&newID; s.proofs[oldID]=p; return p,nil }
func (s *bug08Store) GetComparison(int64)(model.Comparison,error){ panic("unused") }

func TestBug08SupersedeRequiresMatchingContractPair(t *testing.T) {
	s := &bug08Store{proofs: map[int64]model.MigrationProof{
		1:{ID:1,FromContractID:10,ToContractID:20,Status:model.ProofPublished},
		2:{ID:2,FromContractID:30,ToContractID:40,Status:model.ProofPublished},
	}}
	_, err := NewPublisher(s).Supersede(1, 2)
	if err == nil || s.superseded { t.Fatalf("cross-pair supersede err=%v superseded=%v", err, s.superseded) }
	if s.proofs[1].Status != model.ProofPublished { t.Fatalf("old proof changed to %s", s.proofs[1].Status) }
}
