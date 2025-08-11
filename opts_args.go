// SPDX-FileCopyrightText: Copyright DB InfraGO AG and contributors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"encoding/json"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	fncontext "github.com/crossplane/function-sdk-go/context"
	fnapi "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dbinfrago/crossplane-function-test-framework/internal/util/maps"
	"github.com/dbinfrago/crossplane-function-test-framework/internal/util/yaml"
)

// WithContextValue sets the expected context field to value.
func WithContextValue(key string, value any) TestFunctionOpt {
	return func(tc *FunctionTest) {
		val := mustStructValue(value)
		tc.req.Context.Fields[key] = val
	}
}

// WithContextValueYAML reads a value from a single YAML document and sets it
// as value of the given context field.
func WithContextValueYAML(key string, rawYAML []byte) TestFunctionOpt {
	var val any
	if err := yaml.Unmarshal(rawYAML, &val); err != nil {
		panic(err.Error())
	}
	return WithContextValue(key, val)
}

// WithContextValueYAML reads a value from a JSON document and sets it
// as value of the given context field.
func WithContextValueJSON(key string, rawJSON []byte) TestFunctionOpt {
	var val any
	if err := json.Unmarshal(rawJSON, &val); err != nil {
		panic(err.Error())
	}
	return WithContextValue(key, val)
}

// WithInput sets the input that is passed to the function run.
func WithInput(input runtime.Object) TestFunctionOpt {
	str, err := resource.AsStruct(input)
	if err != nil {
		panic(err)
	}
	return func(tc *FunctionTest) {
		tc.req.Input = str
	}
}

// WithInputYAML is the same as [WithInput] but accepts raw YAML.
func WithInputYAML(inputYAML []byte) TestFunctionOpt {
	u := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(inputYAML, u); err != nil {
		panic(err)
	}
	return WithInput(u)
}

// WithInputJSON is the same as [WithInput] but accepts raw JSON.
func WithInputJSON(inputJSON []byte) TestFunctionOpt {
	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(inputJSON, u); err != nil {
		panic(err)
	}
	return WithInput(u)
}

// WithObservedResourceObject adds o to the observed state passed to the
// function.
func WithObservedResourceObject(name string, o runtime.Object) TestFunctionOpt {
	return func(tc *FunctionTest) {
		str := mustObjectAsStruct(o)
		tc.req.Observed.Resources[name] = &fnapi.Resource{
			Resource: str,
		}
	}
}

// WithObservedResourceYAML reads an object from a single YAML document and adds
// it to the observed state passed to the function.
func WithObservedResourceYAML(name string, rawYAML []byte) TestFunctionOpt {
	u := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(rawYAML, u); err != nil {
		panic(err.Error())
	}
	return WithObservedResourceObject(name, u)
}

// WithObservedResourceJSON reads an object from a single JSON document and adds
// it to the observed state passed to the function.
func WithObservedResourceJSON(name string, rawJSON []byte) TestFunctionOpt {
	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(rawJSON, u); err != nil {
		panic(err.Error())
	}
	return WithObservedResourceObject(name, u)
}

// AnnotationKeyResourceName is the key of the annotation that defines the
// resource name.
const AnnotationKeyResourceName = "fn.test/resource-name"

type AnnotatedObject interface {
	GetAnnotations() map[string]string
}

// GetTestResourceName returns the resource key of the given object based on its
// annotation. It falls back to the "crossplane.io/composition-resource-name" if
// the explicit test annotation does not exists.
func GetTestResourceName(o AnnotatedObject) string {
	ann := o.GetAnnotations()
	if testAnn, exists := ann[AnnotationKeyResourceName]; exists {
		return testAnn
	}
	return ann["crossplane.io/composition-resource-name"]
}

// WithObservedResourcesYAML reads all objects from a multi-document YAML and
// passes them with the observed state to the function.
//
// It uses the annotation [AnnotationKeyResourceName] to determine
// the name of the resource.
func WithObservedResourcesYAML(rawYAML []byte) TestFunctionOpt {
	return func(tc *FunctionTest) {
		uList, err := yaml.UnmarshalObjects[*unstructured.Unstructured](rawYAML)
		if err != nil {
			panic(err.Error())
		}
		for _, u := range uList {
			key := GetTestResourceName(u)
			if key == "" {
				panic(fmt.Sprintf("resource has no name annotation: %s/%s", u.GroupVersionKind().String(), u.GetName()))
			}
			meta.RemoveAnnotations(u, AnnotationKeyResourceName)

			str := mustObjectAsStruct(u)
			tc.req.Observed.Resources[key] = &fnapi.Resource{
				Resource: str,
				// ConnectionDetails: str.GetFields()["connectionDetails"],
			}
		}
	}
}

// WithObservedResourcesYAMLOverride loads the given objects from YAML and
// merges them with existing observed objects.
// It only modifies resources that are already observed.
func WithObservedResourcesYAMLOverride(rawYAML []byte) TestFunctionOpt {
	return func(tc *FunctionTest) {
		if tc.req.GetObserved() == nil || len(tc.req.GetObserved().GetResources()) == 0 {
			return
		}
		uList, err := yaml.UnmarshalObjects[*unstructured.Unstructured](rawYAML)
		if err != nil {
			panic(err.Error())
		}
		for _, u := range uList {
			key := GetTestResourceName(u)
			if key == "" {
				panic(fmt.Sprintf("resource has no name annotation: %s/%s", u.GroupVersionKind().String(), u.GetName()))
			}
			meta.RemoveAnnotations(u, AnnotationKeyResourceName)

			observedRes, hasObservedResource := tc.req.GetObserved().GetResources()[key]
			if !hasObservedResource {
				panic(fmt.Sprintf("tried to override observed resource %s, which was not observed yet", key))
			}
			resRaw, err := protojson.Marshal(observedRes.GetResource())
			if err != nil {
				panic(errors.Wrap(err, key).Error())
			}
			original := map[string]interface{}{}
			if err := json.Unmarshal(resRaw, &original); err != nil {
				panic(errors.Wrap(err, key).Error())
			}
			u.Object = maps.Merge(original, u.Object)

			str := mustObjectAsStruct(u)
			tc.req.Observed.Resources[key] = &fnapi.Resource{
				Resource: str,
			}
		}
	}
}

// WithObservedConnectionSecrets expect and reads all ConnectionSecrets  from a multi-document YAML and
// passes their data to the respective observed resources state to the function.
//
// It uses the annotation [AnnotationKeyResourceName] to determine
// the name of the resource.
func WithObservedConnectionSecrets(rawYAML []byte) TestFunctionOpt {
	return func(tc *FunctionTest) {
		uList, err := yaml.UnmarshalObjects[*corev1.Secret](rawYAML)
		if err != nil {
			panic(err.Error())
		}
		for _, u := range uList {
			if u.Type != "connection.crossplane.io/v1alpha1" {
				panic("Secret is not of type connection.crossplane.io/v1alpha1")
			}
			key, exists := u.GetAnnotations()[AnnotationKeyResourceName]
			if !exists || key == "" {
				panic("Secret has no name annotation")
			}
			meta.RemoveAnnotations(u, AnnotationKeyResourceName)

			if tc.req.GetObserved().GetResources()[key] == nil {
				panic("parent resource of the ConnectionSecret is not (yet) observed")
			}
			if u.Data == nil && u.StringData != nil {
				u.Data = make(map[string][]byte)
				for key, value := range u.StringData {
					u.Data[key] = []byte(value)
				}
			}
			tc.req.GetObserved().GetResources()[key].ConnectionDetails = u.Data
		}
	}
}

// WithObservedCompositeObject sets the observed composite to the given object.
func WithObservedCompositeObject(o runtime.Object, mods ...ResourceModifier) TestFunctionOpt {
	return func(tc *FunctionTest) {
		str := mustObjectAsStruct(o)
		res := &fnapi.Resource{
			Resource: str,
		}
		for _, mod := range mods {
			mod(res)
		}
		tc.req.Observed.Composite = res
	}
}

// WithObservedCompositeYAML reads an object from a single YAML document and
// passes it as observed composite to the function.
func WithObservedCompositeYAML(rawYAML []byte, mods ...ResourceModifier) TestFunctionOpt {
	u := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(rawYAML, u); err != nil {
		panic(err.Error())
	}
	return WithObservedCompositeObject(u, mods...)
}

// WithObservedCompositeJSON reads an object from a JSON document and
// passes it as observed composite to the function.
func WithObservedCompositeJSON(rawJSON []byte) TestFunctionOpt {
	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(rawJSON, u); err != nil {
		panic(err.Error())
	}
	return WithObservedCompositeObject(u)
}

var (
	environmentGvk = schema.GroupVersionKind{
		Group:   "internal.crossplane.io",
		Version: "v1alpha1",
		Kind:    "Environment",
	}
)

// WithEnvironmentFromConfigsYAML is a custom test opt that creates an
// environment from a series of EnvironmentConfigs that are read from a
// multi-document YAML file and adds it as environment to the request
// context of a function.
//
// Experimental: Environments are a Crossplane alpha feature and are prone to
// change in the future. This applies to this functions as well.
func WithEnvironmentFromConfigsYAML(rawYAML []byte) TestFunctionOpt {
	configs, err := yaml.UnmarshalObjects[*unstructured.Unstructured](rawYAML)
	if err != nil {
		panic(err.Error())
	}

	env := unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	for _, c := range configs {
		data, exists := c.Object["data"]
		if !exists {
			continue
		}
		dataMap, ok := data.(map[string]interface{})
		if !ok {
			continue
		}
		env.Object = maps.Merge(env.Object, dataMap)
	}
	// Environment Needs a kind because
	env.SetGroupVersionKind(environmentGvk)
	return WithContextValue(fncontext.KeyEnvironment, env.UnstructuredContent())
}

// WithEnvironmentFromConfigsYAMLMultiple is a custom test opt that creates an
// environment from a series of EnvironmentConfigs that are read from a
// multiple single-document YAML files and adds it as environment to the request
// context of a function.
//
// Experimental: Environments are a Crossplane alpha feature and are prone to
// change in the future. This applies to this functions as well.
func WithEnvironmentFromConfigsYAMLMultiple(rawMulitYAML ...[]byte) TestFunctionOpt {
	env := unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	for i, raw := range rawMulitYAML {
		objects, err := yaml.UnmarshalObjects[*unstructured.Unstructured](raw)
		if err != nil {
			panic(errors.Wrapf(err, "cannot unmarshal file at index %d", i).Error())
		}

		for _, o := range objects {
			data, exists := o.Object["data"]
			if !exists {
				continue
			}
			dataMap, ok := data.(map[string]interface{})
			if !ok {
				continue
			}
			env.Object = maps.Merge(env.Object, dataMap)
		}
	}
	// Environment Needs a kind because
	env.SetGroupVersionKind(environmentGvk)
	return WithContextValue(fncontext.KeyEnvironment, env.UnstructuredContent())
}
