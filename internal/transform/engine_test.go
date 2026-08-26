package transform

import (
	"testing"

	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/semantics"
)

type ruleListStore struct{ rules []model.TransformationRule }

func (s ruleListStore) ListRules(int64) ([]model.TransformationRule, error) { return s.rules, nil }

func TestEngineUsesHighestPrecedenceRuleAndPlansReject(t *testing.T) {
	e, err := NewEngine(2, ruleListStore{rules: []model.TransformationRule{
		{FromField: "legacy", ToField: "new", Action: model.ActionRename, Precedence: 1},
		{FromField: "legacy", Action: model.ActionReject, Precedence: 2},
	}})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if got := e.RuleFor("legacy"); got == nil || got.Action != model.ActionReject {
		t.Fatalf("effective rule = %#v, want reject", got)
	}
	if plan := e.PlanFor("legacy", true); !plan.Rejected || plan.Action != model.ActionReject {
		t.Fatalf("present reject plan = %#v", plan)
	}
	if plan := e.PlanFor("legacy", false); plan.Rejected {
		t.Fatalf("missing reject field must not be rejected: %#v", plan)
	}
	if plan := e.PlanFor("unchanged", true); plan.Action != model.ActionKeep || plan.ToField != "unchanged" {
		t.Fatalf("unruled field plan = %#v", plan)
	}
	if _, _, rejected, err := Apply(e.PlanFor("legacy", true), semantics.Value{Present: true, Raw: "1"}, e.RuleFor("legacy")); err != nil || !rejected {
		t.Fatalf("Apply reject = rejected:%v err:%v", rejected, err)
	}
}

func TestEngineDetectsRenameCycle(t *testing.T) {
	e, err := NewEngine(1, ruleListStore{rules: []model.TransformationRule{
		{FromField: "a", ToField: "b", Action: model.ActionRename},
		{FromField: "b", ToField: "a", Action: model.ActionRename},
	}})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := e.DetectCycle(); err == nil {
		t.Fatal("DetectCycle unexpectedly accepted a->b->a")
	}
}
