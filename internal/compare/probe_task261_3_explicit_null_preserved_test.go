package compare

import (
	"testing"
	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/semantics"
	"task261-apimigproof/internal/transform"
)

type bug03Rules struct{}
func (bug03Rules) ListRules(int64) ([]model.TransformationRule, error) { return nil, nil }

func TestBug03ExplicitNullPreserved(t *testing.T) {
	fields := []model.FieldSemantics{{FieldID: "comment", Status: model.FieldValid, ValueType: model.TypeString}}
	eng, err := transform.NewEngine(2, bug03Rules{})
	if err != nil { t.Fatal(err) }
	v, err := NewComparator(semantics.BuildFieldView(fields), semantics.BuildFieldView(fields), eng, model.PolicyTransform).Evaluate(model.RequestSample{ID: 1, Fingerprint: "fp", PayloadJSON: `{"comment":null}`})
	if err != nil { t.Fatal(err) }
	if v.Verdict != model.SampleMigratable || len(v.Issues) != 0 { t.Fatalf("null verdict=%#v, want migratable without issues", v) }
}
