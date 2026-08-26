// Package service 编排各业务包，向 httpapi 层暴露完整用例。
package service

import (
	"fmt"

	"task261-apimigproof/internal/compare"
	"task261-apimigproof/internal/contract"
	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/proof"
	"task261-apimigproof/internal/semantics"
	"task261-apimigproof/internal/store"
	"task261-apimigproof/internal/transform"
)

// Service 聚合全部业务能力。
type Service struct {
	Store  *store.Store
	Reg    *contract.Registry
	Window *proof.WindowService
	Proofs *proof.Publisher
}

// New 构造服务编排层。
func New(st *store.Store) *Service {
	return &Service{
		Store:  st,
		Reg:    contract.NewRegistry(st),
		Window: proof.NewWindowService(st),
		Proofs: proof.NewPublisher(st),
	}
}

// LoadViews 读取新旧契约的字段视图与转换引擎。
// 这是比较的前置步骤：字段视图与引擎都来自持久化层。
func (s *Service) LoadViews(fromID, toID int64) (*semantics.FieldView, *semantics.FieldView, *transform.Engine, error) {
	fromFields, err := s.Store.ListFields(fromID)
	if err != nil {
		return nil, nil, nil, err
	}
	toFields, err := s.Store.ListFields(toID)
	if err != nil {
		return nil, nil, nil, err
	}
	eng, err := transform.NewEngine(toID, s.Store)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := eng.DetectCycle(); err != nil {
		return nil, nil, nil, err
	}
	return semantics.BuildFieldView(fromFields), semantics.BuildFieldView(toFields), eng, nil
}

// RunComparison 执行一次契约对比较并持久化结果。
//
// 并发约束：同一契约对同时只允许一个 running 比较（store 层强制）。
func (s *Service) RunComparison(fromID, toID int64) (model.Comparison, error) {
	if err := s.Reg.ReadyForCompare(fromID); err != nil {
		return model.Comparison{}, err
	}
	if err := s.Reg.ReadyForCompare(toID); err != nil {
		return model.Comparison{}, err
	}
	from, err := s.Store.GetContract(fromID)
	if err != nil {
		return model.Comparison{}, err
	}
	to, err := s.Store.GetContract(toID)
	if err != nil {
		return model.Comparison{}, err
	}
	if from.Version >= to.Version {
		return model.Comparison{}, fmt.Errorf("%w: comparison requires from v%d < to v%d",
			model.ErrVersionRegression, from.Version, to.Version)
	}

	policy, windowID, err := s.Window.Resolve(fromID, toID)
	if err != nil {
		return model.Comparison{}, err
	}
	samples, err := s.Store.ListSamples()
	if err != nil {
		return model.Comparison{}, err
	}
	fromView, toView, eng, err := s.LoadViews(fromID, toID)
	if err != nil {
		return model.Comparison{}, err
	}
	comp, err := s.Store.CreateComparison(fromID, toID, windowID)
	if err != nil {
		return model.Comparison{}, err
	}
	cmp := compare.NewComparator(fromView, toView, eng, policy)

	results := make([]model.SampleResult, 0, len(samples))
	issues := make([]model.Issue, 0)
	for _, sm := range samples {
		vd, err := cmp.Evaluate(sm)
		if err != nil {
			// 单个样本解析失败不影响整体：记为 rejected + coerce/parse 问题。
			vd = &compare.Verdict{
				SampleID: sm.ID, Fingerprint: sm.Fingerprint,
				Verdict: model.SampleRejected, Issue: model.IssueCoerceFailure,
				Detail: err.Error(),
			}
		}
		if vd.Verdict == model.SampleSemanticsChanged {
			if err := s.Store.SetSampleStatus(sm.ID, model.SampleSemanticsChanged); err != nil {
				return model.Comparison{}, err
			}
		} else if vd.Verdict == model.SampleRejected {
			if err := s.Store.SetSampleStatus(sm.ID, model.SampleRejected); err != nil {
				return model.Comparison{}, err
			}
		} else {
			if err := s.Store.SetSampleStatus(sm.ID, model.SampleMigratable); err != nil {
				return model.Comparison{}, err
			}
		}
		results = append(results, model.SampleResult{
			SampleID: vd.SampleID, Fingerprint: vd.Fingerprint,
			Verdict: vd.Verdict, Issue: vd.Issue, Detail: vd.Detail,
		})
		issues = append(issues, vd.Issues...)
	}

	// 汇总与契约对级结论。
	comp.TotalSamples = len(results)
	for _, r := range results {
		switch r.Verdict {
		case model.SampleMigratable:
			comp.Migratable++
		case model.SampleRejected:
			comp.Rejected++
		case model.SampleSemanticsChanged:
			comp.SemanticsChanged++
		}
	}
	if comp.TotalSamples > 0 && comp.SemanticsChanged == 0 && comp.Rejected == 0 {
		comp.Status = model.ComparisonCompatible
	} else {
		comp.Status = model.ComparisonAmbiguous
	}
	if err := s.Store.FinishComparison(comp, results, issues); err != nil {
		return model.Comparison{}, err
	}
	// 回写契约状态：仅回写未封存契约（sealed 是终态，不可覆盖）。
	if to.Status != model.ContractSealed {
		toStatus := model.ContractCompatible
		if comp.Status == model.ComparisonAmbiguous {
			toStatus = model.ContractAmbiguous
		}
		if err := s.Store.SetContractStatus(toID, toStatus); err != nil {
			return model.Comparison{}, err
		}
	}
	return s.Store.GetComparison(comp.ID)
}
