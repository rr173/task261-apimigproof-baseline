package model

import (
	"errors"
	"testing"
)

func TestSampleFingerprintRejectsTrailingContent(t *testing.T) {
	// 完整 JSON 之后附带额外内容必须在导入阶段即被拒绝。
	for _, payload := range []string{
		`{"x":1}{"y":2}`,   // 第二个 JSON 对象
		`{"x":1}  "extra"`, // 完整对象后跟标量
		`{"x":1}garbage`,   // 完整对象后跟非法内容
	} {
		if _, err := SampleFingerprint(payload); err == nil {
			t.Errorf("SampleFingerprint(%q) unexpectedly succeeded", payload)
		} else if !errors.Is(err, ErrBadRequest) {
			t.Errorf("SampleFingerprint(%q) error = %v, want bad request", payload, err)
		}
	}
}

func TestSampleFingerprintAcceptsCleanObject(t *testing.T) {
	// 干净的单对象 payload 仍应正常计算指纹。
	fp, err := SampleFingerprint(`{"mode":"safe"}`)
	if err != nil {
		t.Fatalf("SampleFingerprint: %v", err)
	}
	if fp == "" {
		t.Fatal("fingerprint must not be empty")
	}

	// 同一对象（无论空白差异）指纹一致。
	fp2, err := SampleFingerprint(`{  "mode" : "safe"  }`)
	if err != nil {
		t.Fatalf("SampleFingerprint (whitespace variant): %v", err)
	}
	if fp != fp2 {
		t.Fatalf("fingerprint not stable: %q vs %q", fp, fp2)
	}
}

func TestSampleFingerprintRejectsNonObject(t *testing.T) {
	for _, payload := range []string{"[]", "null", "42", `{"x":}`} {
		if _, err := SampleFingerprint(payload); err == nil {
			t.Errorf("SampleFingerprint(%q) unexpectedly succeeded", payload)
		} else if !errors.Is(err, ErrBadRequest) {
			t.Errorf("SampleFingerprint(%q) error = %v, want bad request", payload, err)
		}
	}
}
