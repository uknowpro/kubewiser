package analyze

import (
	"encoding/json"
	"fmt"
	"github.com/uknowpro/kubewiser/internal/manifest"
	ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structural "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	defaults "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type DiffOptions struct {
	CRDDefaults   bool
	CRDConversion bool
}

func defaultCRD(obj map[string]any, raw map[string]any) error {
	b, _ := json.Marshal(raw)
	var v extv1.JSONSchemaProps
	if e := json.Unmarshal(b, &v); e != nil {
		return fmt.Errorf("invalid CRD default schema")
	}
	var native ext.JSONSchemaProps
	if e := extv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(&v, &native, nil); e != nil {
		return e
	}
	s, e := structural.NewStructural(&native)
	if e != nil || len(structural.ValidateStructural(field.NewPath("schema"), s)) > 0 {
		return fmt.Errorf("CRD defaults require a structural schema")
	}
	defaults.Default(obj, s)
	// The manifest loader uses JSON float64 numbers; normalize inserted integer defaults too.
	normalized := JSONClone(obj)
	for key, value := range normalized {
		obj[key] = value
	}
	return nil
}
func cloneForDiff(r manifest.Resource, ns string) manifest.Resource {
	r.Object = JSONClone(r.Object)
	meta := manifest.Map(r.Object["metadata"])
	if manifest.String(meta, "namespace") == "" && ns != "" {
		meta["namespace"] = ns
		r.Object["metadata"] = meta
	}
	return r
}
