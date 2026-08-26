package store

import (
	"path/filepath"
	"testing"
)

func TestBug01CanonicalSampleFingerprint(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "samples.db"))
	if err != nil { t.Fatal(err) }
	defer st.Close()
	one, added, err := st.ImportSample(`{"enabled":false,"limit":3}`)
	if err != nil || !added { t.Fatalf("first import: added=%v err=%v", added, err) }
	two, added, err := st.ImportSample(" { \"limit\": 3, \"enabled\": false } ")
	if err != nil || added || one.ID != two.ID { t.Fatalf("canonical duplicate: first=%#v second=%#v added=%v err=%v", one, two, added, err) }
	n, err := st.CountSamples()
	if err != nil || n != 1 { t.Fatalf("sample count=%d err=%v, want 1", n, err) }
}
