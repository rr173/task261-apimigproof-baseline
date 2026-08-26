package semantics

import (
	"testing"

	"task261-apimigproof/internal/model"
)

func TestParsePayloadPreservesPresenceAndNull(t *testing.T) {
	p, err := ParsePayload(`{"enabled":false,"limit":0,"note":null}`)
	if err != nil {
		t.Fatalf("ParsePayload: %v", err)
	}
	for _, field := range []string{"enabled", "limit", "note"} {
		value, ok := p.Get(field)
		if !ok || !value.Present {
			t.Fatalf("field %q should be present: %#v, ok=%v", field, value, ok)
		}
	}
	if value, _ := p.Get("enabled"); value.Null {
		t.Fatal("false must not be treated as null")
	}
	if value, _ := p.Get("note"); !value.Null {
		t.Fatal("null must retain its null marker")
	}
	if value, ok := p.Get("missing"); ok || value.Present {
		t.Fatalf("missing field should be absent: %#v, ok=%v", value, ok)
	}
	if got := EffectiveValue(Value{}, true, stringPtr("3")); got != "D:3" {
		t.Fatalf("default effective value = %q, want D:3", got)
	}
	if got := EffectiveValue(Value{Present: true, Raw: "false"}, false, nil); got != "S:false" {
		t.Fatalf("explicit zero effective value = %q, want S:false", got)
	}
}

func TestParsePayloadRejectsNonObject(t *testing.T) {
	for _, payload := range []string{"[]", "null", "42", `{"x":`} {
		if _, err := ParsePayload(payload); err == nil {
			t.Errorf("ParsePayload(%q) unexpectedly succeeded", payload)
		} else if !modelErrorIsBadRequest(err) {
			t.Errorf("ParsePayload(%q) error = %v, want bad request", payload, err)
		}
	}
}

func TestCoerceValueRejectsInvalidTypedInput(t *testing.T) {
	if got, err := CoerceValue("3", model.TypeInt, model.TypeString); err != nil || got != "3" {
		t.Fatalf("int to string = %q, %v", got, err)
	}
	if _, err := CoerceValue("not-an-int", model.TypeString, model.TypeInt); err == nil {
		t.Fatal("invalid integer coercion unexpectedly succeeded")
	}
}

func stringPtr(s string) *string { return &s }

func modelErrorIsBadRequest(err error) bool {
	return err != nil && (containsError(err, model.ErrBadRequest))
}

func containsError(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
