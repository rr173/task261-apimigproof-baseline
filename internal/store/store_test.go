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

// 内容相同但仅空白/对象键顺序不同的请求应折叠为同一条样本，
// 而真正不同的请求仍可分别记录。
func TestImportSampleFoldsWhitespaceAndKeyOrder(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "samples.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer st.Close()

	base := `{"mode":"safe","retry":3}`
	// 同一对象，仅空白与键顺序不同。
	whitespace := `{"mode": "safe",  "retry": 3}`
	reordered := `{"retry":3,"mode":"safe"}`

	one, added, err := st.ImportSample(base)
	if err != nil || !added {
		t.Fatalf("base import: added=%v err=%v", added, err)
	}
	for _, variant := range []string{whitespace, reordered} {
		got, added, err := st.ImportSample(variant)
		if err != nil || added || got.ID != one.ID {
			t.Fatalf("variant %q: got=%#v added=%v err=%v, want same sample as base", variant, got, added, err)
		}
	}

	// 真正不同的请求仍应分别记录。
	other, added, err := st.ImportSample(`{"mode":"safe","retry":4}`)
	if err != nil || !added {
		t.Fatalf("distinct import: added=%v err=%v", added, err)
	}
	if other.ID == one.ID {
		t.Fatalf("distinct request folded onto base sample: other.ID=%d one.ID=%d", other.ID, one.ID)
	}
}
