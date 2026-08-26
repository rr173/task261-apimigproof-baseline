package proof

import (
	"fmt"
	"time"

	"task261-apimigproof/internal/model"
)

// WindowService 管理兼容窗口声明。
type WindowService struct {
	store WindowStore
}

// WindowStore 定义 WindowService 依赖的存储接口。
type WindowStore interface {
	CreateWindow(w model.CompatWindow) (model.CompatWindow, error)
	GetWindow(id int64) (model.CompatWindow, error)
	ListWindows() ([]model.CompatWindow, error)
	UpdateWindowPolicy(id int64, policy string) error
	FindWindow(fromID, toID int64) (*model.CompatWindow, error)
	GetContract(id int64) (model.ContractVersion, error)
}

// NewWindowService 构造兼容窗口服务。
func NewWindowService(s WindowStore) *WindowService { return &WindowService{store: s} }

// Declare 声明兼容窗口：绑定新旧契约对并给出拒绝策略。
// 不变量：两契约必须存在；from.version 必须小于 to.version。
func (w *WindowService) Declare(fromID, toID int64, policy, note string, validUntil *string) (model.CompatWindow, error) {
	from, err := w.store.GetContract(fromID)
	if err != nil {
		return model.CompatWindow{}, err
	}
	to, err := w.store.GetContract(toID)
	if err != nil {
		return model.CompatWindow{}, err
	}
	if from.Version >= to.Version {
		return model.CompatWindow{}, fmt.Errorf("%w: window requires from v%d < to v%d",
			model.ErrVersionRegression, from.Version, to.Version)
	}
	if err := model.ValidatePolicy(policy); err != nil {
		return model.CompatWindow{}, err
	}
	if validUntil != nil {
		if _, err := time.Parse(time.RFC3339, *validUntil); err != nil {
			return model.CompatWindow{}, fmt.Errorf("%w: valid_until must be RFC3339: %v", model.ErrBadRequest, err)
		}
	}
	return w.store.CreateWindow(model.CompatWindow{
		FromContractID: fromID, ToContractID: toID,
		Policy: policy, Note: note, ValidUntil: validUntil,
	})
}

// List 列出全部窗口。
func (w *WindowService) List() ([]model.CompatWindow, error) {
	return w.store.ListWindows()
}

// UpdatePolicy 更新窗口拒绝策略。
func (w *WindowService) UpdatePolicy(id int64, policy string) (model.CompatWindow, error) {
	if err := w.store.UpdateWindowPolicy(id, policy); err != nil {
		return model.CompatWindow{}, err
	}
	return w.store.GetWindow(id)
}

// Resolve 解析契约对绑定的兼容窗口策略；无窗口时返回 transform（默认）。
func (w *WindowService) Resolve(fromID, toID int64) (string, *int64, error) {
	win, err := w.store.FindWindow(fromID, toID)
	if err != nil {
		return "", nil, err
	}
	if win == nil {
		return model.PolicyTransform, nil, nil
	}
	var wid int64 = win.ID
	return win.Policy, &wid, nil
}
