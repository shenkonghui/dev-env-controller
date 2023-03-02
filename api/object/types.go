package object

import (
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var ObjMap = make(map[string]*CustomObject)

type CustomObject struct {
	One           *unstructured.Unstructured     `json:"one"`
	List          *unstructured.UnstructuredList `json:"list"`
	Plural        string                         `json:"plural"`
	Children      map[string]*CustomObject       `json:"children"`
	StatusMapping map[string]string              `json:"status_mapping"`
}

func IsSchemeExist(ust *unstructured.Unstructured) bool {
	crd := &v1.CustomResourceDefinition{}
	runtime.DefaultUnstructuredConverter.FromUnstructured(ust.Object, crd)
	_, exist := ObjMap[crd.Spec.Names.Singular]
	return exist
}
