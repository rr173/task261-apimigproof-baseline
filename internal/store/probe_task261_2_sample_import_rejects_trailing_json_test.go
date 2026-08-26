package store

import (
	"path/filepath"
	"testing"
)

func TestBug02SampleImportRejectsTrailingJSON(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "samples.db"))
	if err != nil { t.Fatal(err) }
	defer st.Close()
	if _, added, err := st.ImportSample(`{"mode":"safe"} trailing`); err == nil || added { t.Fatalf("trailing JSON accepted: added=%v err=%v", added, err) }
	n, err := st.CountSamples()
	if err != nil || n != 0 { t.Fatalf("sample count=%d err=%v, want 0", n, err) }
}
