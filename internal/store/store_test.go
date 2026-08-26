package store

import (
	"errors"
	"path/filepath"
	"testing"

	"task261-apimigproof/internal/model"
)

func TestContractVersionRegressionAndRestartPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	c, err := st.CreateContract("orders", 1)
	if err != nil {
		t.Fatalf("CreateContract: %v", err)
	}
	if _, err := st.CreateContract("orders", 1); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("duplicate version err = %v, want conflict", err)
	}
	if _, err := st.CreateContract("orders", 0); !errors.Is(err, model.ErrVersionRegression) {
		t.Fatalf("regressed version err = %v, want version regression", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	st, err = OpenStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st.Close()
	got, err := st.GetContract(c.ID)
	if err != nil {
		t.Fatalf("GetContract after reopen: %v", err)
	}
	if got.Name != "orders" || got.Version != 1 || got.Status != model.ContractDraft {
		t.Fatalf("recovered contract = %#v", got)
	}
}

func TestImportSampleFingerprintIsIdempotent(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "samples.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer st.Close()
	one, added, err := st.ImportSample(`{"mode":"safe"}`)
	if err != nil || !added {
		t.Fatalf("first import: added=%v err=%v", added, err)
	}
	two, added, err := st.ImportSample(`{"mode":"safe"}`)
	if err != nil || added || one.ID != two.ID {
		t.Fatalf("second import: first=%#v second=%#v added=%v err=%v", one, two, added, err)
	}
}
