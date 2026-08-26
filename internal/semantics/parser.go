// Package semantics 负责解析字段语义与请求样本的可见效果。
//
// 核心概念：
//   - presence（在场性）：字段在请求体中是否被显式设置。区分「未设置」与
//     「显式零值」是语义保持性判定的基础。
//   - 可见效果（EffectiveValue）：(值, 在场性) 元组。跨版本比较比较的是
//     可见效果，而非原始 JSON。
package semantics

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"task261-apimigproof/internal/model"
)

// Payload 解析后的请求样本。
type Payload struct {
	Values map[string]Value
}

// Value 描述一个字段的原始值与其在场性。
type Value struct {
	Present bool   // 字段是否在请求体中显式出现
	Raw     string // 值的 JSON 文本（Present=true 时有效）
	Null    bool   // 值是否为 JSON null
}

// ParsePayload 把请求体 JSON 解析为字段值映射。
// 非对象 JSON（数组、标量）视为非法样本。
func ParsePayload(payload string) (*Payload, error) {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(strings.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON payload: %v", model.ErrBadRequest, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("%w: payload contains multiple JSON values", model.ErrBadRequest)
		}
		return nil, fmt.Errorf("%w: invalid trailing JSON: %v", model.ErrBadRequest, err)
	}
	if raw == nil {
		return nil, fmt.Errorf("%w: payload must be a JSON object", model.ErrBadRequest)
	}
	p := &Payload{Values: make(map[string]Value, len(raw))}
	for k, v := range raw {
		text := string(v)
		null := strings.TrimSpace(text) == "null"
		p.Values[k] = Value{Present: true, Raw: text, Null: null}
	}
	return p, nil
}

// Get 返回字段值；字段缺失时 Present=false。
func (p *Payload) Get(field string) (Value, bool) {
	v, ok := p.Values[field]
	if !ok {
		return Value{}, false
	}
	return v, true
}

// EffectiveValue 计算字段的可见效果文本：在场性 + 规范化值。
// 该文本用于跨版本比较：只有 (在场性, 值) 完全一致才算语义等价。
func EffectiveValue(v Value, hasDefault bool, defaultValue *string) string {
	if !v.Present {
		if hasDefault && defaultValue != nil {
			// 未设置但旧契约声明默认值：可见效果即默认值（带标记 D）。
			return "D:" + *defaultValue
		}
		return "ABSENT"
	}
	if v.Null {
		return "NULL"
	}
	return "S:" + NormalizeForCompare(v.Raw)
}

// NormalizeForCompare 规范化值文本（布尔与数值统一形态），保证
// "true"/"true "、"1"/"1.0" 视为等价。导出供比较器复用。
func NormalizeForCompare(raw string) string {
	s := strings.TrimSpace(raw)
	// 布尔规范化。
	if b, err := strconv.ParseBool(s); err == nil {
		return strconv.FormatBool(b)
	}
	// 数值规范化：去掉无意义前导/小数尾零。
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return s
}

// DefaultValueFor 返回字段声明的默认值文本；无默认值时返回 nil。
func DefaultValueFor(f model.FieldSemantics) *string {
	if f.HasDefault && f.DefaultValue != nil {
		return f.DefaultValue
	}
	return nil
}

// CoerceValue 按转换规则把源值强制转换为目标类型。
// 返回转换后的 JSON 文本与是否成功。失败原因用于错误边界报告。
func CoerceValue(raw, fromType, toType string) (string, error) {
	raw = strings.TrimSpace(raw)
	switch toType {
	case model.TypeString:
		// 字符串目标：int/bool/float 直接字面化，string 原样。
		switch fromType {
		case model.TypeString:
			return raw, nil
		case model.TypeBool, model.TypeInt, model.TypeFloat:
			return raw, nil
		case model.TypeJSON:
			// JSON 对象/数组 → 压缩成一行 JSON 字符串。
			var v any
			if err := json.Unmarshal([]byte(raw), &v); err != nil {
				return "", fmt.Errorf("invalid json value %q", raw)
			}
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(b), nil
		}
	case model.TypeInt:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return "", fmt.Errorf("cannot coerce %q to int", raw)
		}
		return strconv.FormatInt(int64(f), 10), nil
	case model.TypeFloat:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return "", fmt.Errorf("cannot coerce %q to float", raw)
		}
		return strconv.FormatFloat(f, 'f', -1, 64), nil
	case model.TypeBool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return "", fmt.Errorf("cannot coerce %q to bool", raw)
		}
		return strconv.FormatBool(b), nil
	case model.TypeJSON:
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return "", fmt.Errorf("invalid json value %q", raw)
		}
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return "", fmt.Errorf("unsupported coerce %s -> %s", fromType, toType)
}
