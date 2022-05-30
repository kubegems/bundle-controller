package utils

import (
	"bytes"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"
)

func SplitYAML(data []byte) ([]*unstructured.Unstructured, error) {
	d := kubeyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var objs []*unstructured.Unstructured
	for {
		u := &unstructured.Unstructured{}
		if err := d.Decode(u); err != nil {
			if err == io.EOF {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %v", err)
		}
		if u.Object == nil || len(u.Object) == 0 {
			continue // skip empty object
		}
		objs = append(objs, u)
	}
	return objs, nil
}

// SplitYAMLFilterd reurns objects has type of `t`
func SplitYAMLFilterd[T runtime.Object](raw io.Reader) ([]T, error) {
	const readcache = 4096
	d := kubeyaml.NewYAMLOrJSONDecoder(raw, readcache)
	decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer()

	var objs []T
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if err == io.EOF {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %v", err)
		}
		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}

		obj, _, err := decoder.Decode(ext.Raw, nil, nil)
		if err != nil {
			// decode type error using unstructured
			obj = &unstructured.Unstructured{}
			if e := yaml.Unmarshal(ext.Raw, obj); e != nil {
				return nil, e
			}
		}
		if istyped, ok := obj.(T); ok {
			objs = append(objs, istyped)
		}
	}
	return objs, nil
}
