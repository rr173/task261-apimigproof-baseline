package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// 领域错误：供 service 层与 httpapi 层做可读错误映射。
var (
	ErrNotFound          = errors.New("resource not found")
	ErrConflict          = errors.New("conflict")
	ErrBadRequest        = errors.New("bad request")
	ErrSealedImmutable   = errors.New("sealed contract is immutable")
	ErrVersionRegression = errors.New("version regression is not allowed")
	ErrDuplicateField    = errors.New("duplicate field id in contract")
	ErrUnknownField      = errors.New("unknown field")
	ErrRuleCycle         = errors.New("transformation rule cycle detected")
	ErrProofSealed       = errors.New("published proof is immutable")
	ErrSampleReferenced  = errors.New("sample is referenced by a published proof")
	ErrCompareRunning    = errors.New("a comparison for the same contract pair is already running")
)

// StatusError 携带 HTTP 语义的状态错误，便于 httpapi 直接映射。
type StatusError struct {
	Status int
	Code   string
	Msg    string
}

func (e *StatusError) Error() string { return e.Msg }

// NewStatusError 构造带状态码的错误。
func NewStatusError(status int, code, msg string) *StatusError {
	return &StatusError{Status: status, Code: code, Msg: msg}
}

// WrapStatus 把领域错误包装为状态错误；未知错误一律 500。
func WrapStatus(err error, status int, code string) error {
	return &StatusError{Status: status, Code: code, Msg: err.Error()}
}

// Classify 将底层错误映射为状态错误。
func Classify(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*StatusError); ok {
		return err
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return NewStatusError(404, "not_found", err.Error())
	case errors.Is(err, ErrConflict), errors.Is(err, ErrDuplicateField),
		errors.Is(err, ErrVersionRegression), errors.Is(err, ErrRuleCycle),
		errors.Is(err, ErrCompareRunning):
		return NewStatusError(409, "conflict", err.Error())
	case errors.Is(err, ErrSealedImmutable), errors.Is(err, ErrProofSealed),
		errors.Is(err, ErrSampleReferenced):
		return NewStatusError(409, "immutable", err.Error())
	case errors.Is(err, ErrBadRequest), errors.Is(err, ErrUnknownField):
		return NewStatusError(400, "bad_request", err.Error())
	default:
		return NewStatusError(500, "internal", err.Error())
	}
}

// SampleFingerprint 计算请求样本的内容指纹（SHA-256 十六进制）。
// 指纹基于规范化后的 payload 计算，保证同一请求重复导入只保留一条。
func SampleFingerprint(payload string) (string, error) {
	if payload == "" {
		return "", fmt.Errorf("%w: empty payload", ErrBadRequest)
	}
	dec := json.NewDecoder(strings.NewReader(payload))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return "", fmt.Errorf("%w: invalid JSON payload: %v", ErrBadRequest, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return "", fmt.Errorf("%w: payload contains multiple JSON values", ErrBadRequest)
		}
		return "", fmt.Errorf("%w: invalid trailing JSON: %v", ErrBadRequest, err)
	}
	if value == nil {
		return "", fmt.Errorf("%w: payload must be a JSON object", ErrBadRequest)
	}
	if _, ok := value.(map[string]any); !ok {
		return "", fmt.Errorf("%w: payload must be a JSON object", ErrBadRequest)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("%w: canonicalize JSON payload: %v", ErrBadRequest, err)
	}
	h := sha256.Sum256(canonical)
	return hex.EncodeToString(h[:]), nil
}

// ValidateContractStatus 校验契约状态是否合法。
func ValidateContractStatus(s string) error {
	switch s {
	case ContractDraft, ContractPendingCompare, ContractCompatible,
		ContractAmbiguous, ContractSealed:
		return nil
	default:
		return fmt.Errorf("invalid contract status %q", s)
	}
}

// ValidateFieldStatus 校验字段语义状态是否合法。
func ValidateFieldStatus(s string) error {
	switch s {
	case FieldValid, FieldDeprecated, FieldTransformed, FieldConflict:
		return nil
	default:
		return fmt.Errorf("invalid field status %q", s)
	}
}

// ValidateValueType 校验字段值类型是否合法。
func ValidateValueType(t string) error {
	switch t {
	case TypeBool, TypeInt, TypeString, TypeFloat, TypeJSON:
		return nil
	default:
		return fmt.Errorf("invalid value type %q", t)
	}
}

// ValidateAction 校验转换动作是否合法。
func ValidateAction(a string) error {
	switch a {
	case ActionKeep, ActionRename, ActionCoerce, ActionDefault, ActionDrop, ActionReject:
		return nil
	default:
		return fmt.Errorf("invalid action %q", a)
	}
}

// ValidatePolicy 校验兼容窗口拒绝策略是否合法。
func ValidatePolicy(p string) error {
	switch p {
	case PolicyPreserve, PolicyTransform, PolicyReject:
		return nil
	default:
		return fmt.Errorf("invalid policy %q", p)
	}
}
