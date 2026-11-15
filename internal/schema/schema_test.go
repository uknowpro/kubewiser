package schema

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/manifest"
	"testing"
)

func resource(t *testing.T, text string) manifest.Resource {
	t.Helper()
	s, e := manifest.Decode(context.Background(), []byte(text), "test")
	if e != nil || len(s.Resources) != 1 {
		t.Fatal(s, e)
	}
	return s.Resources[0]
}
func TestAllOfficialBundles(t *testing.T) {
	for n := 16; n <= 37; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			v, e := New(n)
			if e != nil {
				t.Fatal(e)
			}
			b, _ := bundles.ReadFile(fmt.Sprintf("bundles/v1.%d.0.json.gz", n))
			if fmt.Sprintf("%x", sha256.Sum256(b)) != v.Source.BundleSHA {
				t.Fatal("bundle hash mismatch")
			}
			r := resource(t, "apiVersion: v1\nkind: Service\nmetadata: {name: test}\nspec:\n  ports: [{port: 80, targetPort: http}]\n")
			errs, known, e := v.Validate(r)
			if e != nil || !known || len(errs) > 0 {
				t.Fatal(errs, known, e)
			}
		})
	}
}
func TestUnknownAndTypeFields(t *testing.T) {
	v, e := New(32)
	if e != nil {
		t.Fatal(e)
	}
	for _, text := range []string{"apiVersion: v1\nkind: Pod\nspec: {containers: wrong}\n", "apiVersion: v1\nkind: Pod\nspec: {containers: [], typo: true}\n"} {
		errs, known, e := v.Validate(resource(t, text))
		if e != nil || !known || len(errs) == 0 {
			t.Fatal(errs, known, e)
		}
	}
}
func TestCRDSchemaAndRemoteReference(t *testing.T) {
	v, _ := New(32)
	crd := resource(t, `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
spec:
  group: example.org
  names: {kind: Widget, plural: widgets}
  versions:
  - name: v1
    served: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            required: [count]
            properties:
              count: {type: integer, minimum: 1}
`)
	if e := v.AddCRDs([]manifest.Resource{crd}); e != nil {
		t.Fatal(e)
	}
	errs, known, e := v.Validate(resource(t, "apiVersion: example.org/v1\nkind: Widget\nspec: {count: nope}\n"))
	if e != nil || !known || len(errs) == 0 {
		t.Fatal(errs, known, e)
	}
	if e = normalize(map[string]any{"$ref": "https://example.org/schema"}, false); e == nil {
		t.Fatal("remote ref accepted")
	}
	_, known, _ = v.Validate(resource(t, "apiVersion: other.org/v1\nkind: Unknown\n"))
	if known {
		t.Fatal("unknown schema claimed covered")
	}
}
