package semantics

import (
	"fmt"

	"task261-apimigproof/internal/model"
)

// FieldView 是契约字段语义的内存视图，供比较阶段快速查找。
type FieldView struct {
	ByID map[string]model.FieldSemantics
}

// BuildFieldView 从字段列表构造索引视图。
func BuildFieldView(fields []model.FieldSemantics) *FieldView {
	fv := &FieldView{ByID: make(map[string]model.FieldSemantics, len(fields))}
	for _, f := range fields {
		fv.ByID[f.FieldID] = f
	}
	return fv
}

// Get 按字段 ID 查找；不存在返回 ok=false。
func (fv *FieldView) Get(fieldID string) (model.FieldSemantics, bool) {
	f, ok := fv.ByID[fieldID]
	return f, ok
}

// HasDistinctZeroSemantics 判断字段是否具备「未设置 ≠ 显式零值」的区分语义。
// 布尔字段是典型例子：absent（未设置，走默认）与 false（显式关闭）可区分。
// 判定规则：类型为 bool，或字段声明了非空默认值。
func HasDistinctZeroSemantics(f model.FieldSemantics) bool {
	if f.ValueType == model.TypeBool {
		return true
	}
	if f.HasDefault && f.DefaultValue != nil && *f.DefaultValue != "" && *f.DefaultValue != "null" {
		return true
	}
	return false
}

// ValidateFieldDecl 校验一条字段语义声明是否可以进入比较：
//   - status 必须合法；
//   - conflict 状态的字段不允许出现在待比较契约中（阻塞）。
func ValidateFieldDecl(f model.FieldSemantics) error {
	if err := model.ValidateFieldStatus(f.Status); err != nil {
		return err
	}
	if f.Status == model.FieldConflict {
		return fmt.Errorf("%w: field %q is in conflict status, resolve before comparing",
			model.ErrBadRequest, f.FieldID)
	}
	return nil
}

// LookupOrCoerce 尝试把字段值定位到目标契约的对应字段：
//  1. 精确同名；
//  2. 无同名时返回 nil（字段被移除）。
func LookupOrCoerce(fv *FieldView, fieldID string) (*model.FieldSemantics, bool) {
	f, ok := fv.Get(fieldID)
	if !ok {
		return nil, false
	}
	return &f, true
}
