package manifest

import (
	"context"
	"strings"
	"testing"
)

func TestListLocationsAndPartialFailure(t *testing.T) {
	s, e := Decode(context.Background(), []byte("apiVersion: v1\nkind: List\nitems:\n- apiVersion: v1\n  kind: Service\n  metadata:\n    name: x\n---\nkind: Pod\nkind: Secret\n"), "x")
	if e != nil || len(s.Resources) != 1 || len(s.Issues) != 1 {
		t.Fatal(s, e)
	}
	if s.Resources[0].Locations["/kind"] != 5 {
		t.Fatal(s.Resources[0].Locations)
	}
}
func TestAliasesAndCancellation(t *testing.T) {
	s, e := Decode(context.Background(), []byte("a: &x [1, 2]\nb: *x\n"), "x")
	if e != nil || len(s.Issues) != 1 {
		t.Fatal(s, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Decode(ctx, []byte("kind: Pod"), "x"); e == nil {
		t.Fatal("cancel ignored")
	}
	if _, e = Decode(context.Background(), []byte(strings.Repeat("a", MaxFile+1)), "x"); e == nil {
		t.Fatal("size limit ignored")
	}
}
