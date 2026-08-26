// Package contract 管理契约版本与字段语义的注册。
//
// 契约注册的职责：
//  1. 版本号单调递增（拒绝版本倒退）；
//  2. 字段 ID 在同一契约内唯一；
//  3. 字段语义冲突（conflict）检测——同名字段重复声明矛盾语义即置冲突；
//  4. 封存（seal）——封存后任何字段与规则变更都被拒绝，保证比较基线不可变。
package contract

import (
	"fmt"

	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/semantics"
)

// Registry 是契约注册的服务对象。
type Registry struct {
	store Store
}

// Store 定义 Registry 依赖的存储接口（由 store.Store 实现）。
type Store interface {
	CreateContract(name string, version int) (model.ContractVersion, error)
	GetContract(id int64) (model.ContractVersion, error)
	UpsertField(f model.FieldSemantics) (model.FieldSemantics, error)
	GetField(contractID int64, fieldID string) (model.FieldSemantics, error)
	ListFields(contractID int64) ([]model.FieldSemantics, error)
	DeleteField(contractID int64, fieldID string) error
	SetContractStatus(id int64, status string) error
	SealContract(id int64) error
	CountFieldsByStatus(contractID int64, status string) (int, error)
}

// NewRegistry 构造注册服务。
func NewRegistry(s Store) *Registry { return &Registry{store: s} }

// CreateContract 创建契约版本。
func (r *Registry) CreateContract(name string, version int) (model.ContractVersion, error) {
	return r.store.CreateContract(name, version)
}

// GetContract 读取契约版本。
func (r *Registry) GetContract(id int64) (model.ContractVersion, error) {
	return r.store.GetContract(id)
}

// AddField 添加字段语义声明。
//
// 不变量：同一契约内字段 ID 唯一；若重复声明且语义矛盾（类型不同或
// 默认值不同）则置为 conflict 状态；conflict 字段阻塞比较。
func (r *Registry) AddField(f model.FieldSemantics) (model.FieldSemantics, error) {
	if f.Status == "" {
		f.Status = model.FieldValid
	}
	if err := semantics.ValidateFieldDecl(f); err != nil {
		return model.FieldSemantics{}, err
	}
	// 契约必须存在且未封存。
	contract, err := r.store.GetContract(f.ContractID)
	if err != nil {
		return model.FieldSemantics{}, err
	}
	if contract.Status == model.ContractSealed {
		return model.FieldSemantics{}, fmt.Errorf("%w: contract %d", model.ErrSealedImmutable, f.ContractID)
	}
	// 重复声明检查：语义矛盾 → conflict。
	existing, err := r.store.GetField(f.ContractID, f.FieldID)
	if err == nil {
		if !fieldSemanticsEqual(existing, f) {
			f.Status = model.FieldConflict
			f.Description = fmt.Sprintf("conflict with prior declaration (type=%s default=%v)",
				existing.ValueType, existing.DefaultValue)
		}
	}
	return r.store.UpsertField(f)
}

// UpdateField 更新字段语义（标记弃用等）。
func (r *Registry) UpdateField(f model.FieldSemantics) (model.FieldSemantics, error) {
	contract, err := r.store.GetContract(f.ContractID)
	if err != nil {
		return model.FieldSemantics{}, err
	}
	if contract.Status == model.ContractSealed {
		return model.FieldSemantics{}, fmt.Errorf("%w: contract %d", model.ErrSealedImmutable, f.ContractID)
	}
	if _, err := r.store.GetField(f.ContractID, f.FieldID); err != nil {
		return model.FieldSemantics{}, err
	}
	if err := semantics.ValidateFieldDecl(f); err != nil {
		return model.FieldSemantics{}, err
	}
	return r.store.UpsertField(f)
}

// RemoveField 移除字段（仅 draft 契约）。
func (r *Registry) RemoveField(contractID int64, fieldID string) error {
	contract, err := r.store.GetContract(contractID)
	if err != nil {
		return err
	}
	if contract.Status != model.ContractDraft {
		return fmt.Errorf("%w: only draft contracts are editable", model.ErrSealedImmutable)
	}
	return r.store.DeleteField(contractID, fieldID)
}

// Seal 封存契约。封存前要求字段语义齐备且无 conflict。
func (r *Registry) Seal(contractID int64) (model.ContractVersion, error) {
	contract, err := r.store.GetContract(contractID)
	if err != nil {
		return model.ContractVersion{}, err
	}
	if contract.Status == model.ContractSealed {
		return contract, nil
	}
	conflicts, err := r.store.CountFieldsByStatus(contractID, model.FieldConflict)
	if err != nil {
		return model.ContractVersion{}, err
	}
	if conflicts > 0 {
		return model.ContractVersion{}, fmt.Errorf("%w: %d conflict field(s) must be resolved before seal",
			model.ErrBadRequest, conflicts)
	}
	if err := r.store.SealContract(contractID); err != nil {
		return model.ContractVersion{}, err
	}
	return r.store.GetContract(contractID)
}

// ReadyForCompare 检查契约是否具备比较条件（非草稿、无冲突字段）。
func (r *Registry) ReadyForCompare(contractID int64) error {
	contract, err := r.store.GetContract(contractID)
	if err != nil {
		return err
	}
	if contract.Status == model.ContractDraft {
		return fmt.Errorf("%w: contract %d is still a draft", model.ErrBadRequest, contractID)
	}
	conflicts, err := r.store.CountFieldsByStatus(contractID, model.FieldConflict)
	if err != nil {
		return err
	}
	if conflicts > 0 {
		return fmt.Errorf("%w: contract %d has %d conflict field(s)",
			model.ErrBadRequest, contractID, conflicts)
	}
	return nil
}

// fieldSemanticsEqual 判断两条字段语义声明是否等价（类型与默认值一致）。
func fieldSemanticsEqual(a, b model.FieldSemantics) bool {
	if a.ValueType != b.ValueType {
		return false
	}
	if a.HasDefault != b.HasDefault {
		return false
	}
	if a.HasDefault {
		if a.DefaultValue == nil || b.DefaultValue == nil {
			return a.DefaultValue == b.DefaultValue
		}
		return *a.DefaultValue == *b.DefaultValue
	}
	return true
}
