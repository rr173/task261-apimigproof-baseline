package compare

import (
	"testing"

	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/semantics"
	"task261-apimigproof/internal/transform"
)

type compareRuleStore struct{ rules []model.TransformationRule }

func (s compareRuleStore) ListRules(int64) ([]model.TransformationRule, error) { return s.rules, nil }

func TestEvaluateRejectsExplicitDeprecatedField(t *testing.T) {
	from := semantics.BuildFieldView([]model.FieldSemantics{{FieldID: "legacy", ValueType: model.TypeBool, Status: model.FieldDeprecated}})
	to := semantics.BuildFieldView(nil)
	eng, err := transform.NewEngine(2, compareRuleStore{rules: []model.TransformationRule{
		{FromField: "legacy", Action: model.ActionReject},
	}})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	verdict, err := NewComparator(from, to, eng, model.PolicyReject).Evaluate(model.RequestSample{
		ID: 1, Fingerprint: "fp", PayloadJSON: `{"legacy":true}`,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if verdict.Verdict != model.SampleRejected || verdict.Issue != model.IssueRuleRejected {
		t.Fatalf("verdict = %#v, want rejected/rule_rejected", verdict)
	}
}

func TestEvaluateReportsUnknownFieldDeterministically(t *testing.T) {
	from := semantics.BuildFieldView(nil)
	to := semantics.BuildFieldView(nil)
	eng, err := transform.NewEngine(2, compareRuleStore{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	verdict, err := NewComparator(from, to, eng, "").Evaluate(model.RequestSample{
		ID: 2, Fingerprint: "fp", PayloadJSON: `{"unknown":1}`,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if verdict.Verdict != model.SampleSemanticsChanged || verdict.Issue != model.IssueUnknownField {
		t.Fatalf("verdict = %#v, want semantics_changed/unknown_field", verdict)
	}
	if len(verdict.Issues) != 1 || verdict.Issues[0].Field != "unknown" {
		t.Fatalf("issues = %#v", verdict.Issues)
	}
}

// TestEvaluateExplicitNullEquivalentPreserved 验证旧客户端显式发送的 null 字段，
// 在新旧契约语义相同（字段保持，无规则接管）时被判为可迁移：显式 null 的在场性
// 与值（"NULL"）在新侧保持，且与字段未设置（ABSENT）严格区分。
func TestEvaluateExplicitNullEquivalentPreserved(t *testing.T) {
	from := semantics.BuildFieldView([]model.FieldSemantics{
		{FieldID: "note", ValueType: model.TypeString, Status: model.FieldValid},
	})
	to := semantics.BuildFieldView([]model.FieldSemantics{
		{FieldID: "note", ValueType: model.TypeString, Status: model.FieldValid},
	})
	eng, err := transform.NewEngine(2, compareRuleStore{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	cases := map[string]string{
		`{"note":null}`:  "explicit null",
		`{"note":""}`:    "explicit empty string",
		`{"note":"x"}`:   "explicit value",
		`{}`:             "field unset -> absent",
	}
	for pl, desc := range cases {
		v, err := NewComparator(from, to, eng, "").Evaluate(model.RequestSample{
			ID: 1, Fingerprint: "fp", PayloadJSON: pl,
		})
		if err != nil {
			t.Fatalf("Evaluate(%s): %v", desc, err)
		}
		if v.Verdict != model.SampleMigratable {
			t.Errorf("%s verdict=%s issue=%s detail=%s, want migratable",
				desc, v.Verdict, v.Issue, v.Detail)
		}
		if len(v.Issues) != 0 {
			t.Errorf("%s expected no issues, got %#v", desc, v.Issues)
		}
	}
}
