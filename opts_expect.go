// SPDX-FileCopyrightText: Copyright DB InfraGO AG and contributors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"encoding/json"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	fnapi "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/dbinfrago/crossplane-function-test-framework/internal/util/maps"
	"github.com/dbinfrago/crossplane-function-test-framework/internal/util/yaml"
)

// ResourceModifier modifies a [fnapi.Resource].
type ResourceModifier func(res *fnapi.Resource)

// WithReady sets the ready state of an [fnapi.Resource].
func WithReady(ready fnapi.Ready) ResourceModifier {
	return func(res *fnapi.Resource) { res.Ready = ready }
}

// WithConnectionDetails sets the connection details of an [fnapi.Resource].
func WithConnectionDetails(cd map[string][]byte) ResourceModifier {
	return func(res *fnapi.Resource) { res.ConnectionDetails = cd }
}

func WithoutAPIVersionAndKind() ResourceModifier {
	return func(res *fnapi.Resource) {
		delete(res.GetResource().GetFields(), "apiVersion")
		delete(res.GetResource().GetFields(), "kind")
	}
}

// WithManifestOverride is a modifier that merges the existing resource
// manifest with the given overrideYAML.
func WithManifestOverride(overrideYAML []byte) ResourceModifier {
	return func(res *fnapi.Resource) {
		override := map[string]interface{}{}
		if err := yaml.Unmarshal(overrideYAML, &override); err != nil {
			panic(err.Error())
		}
		overwriteResourceManifest(res, override)
	}
}

// WithManifestOverride is a modifier that merges the existing resource
// manifest with the given override object.
func WithManifestOverrideObject(override runtime.Object) ResourceModifier {
	return func(res *fnapi.Resource) {
		overrideU, err := runtime.DefaultUnstructuredConverter.ToUnstructured(override)
		if err != nil {
			panic(err.Error())
		}
		overwriteResourceManifest(res, overrideU)
	}
}

func overwriteResourceManifest(res *fnapi.Resource, override map[string]interface{}) {
	resRaw, err := protojson.Marshal(res.GetResource())
	if err != nil {
		panic(err.Error())
	}
	original := map[string]interface{}{}
	if err := json.Unmarshal(resRaw, &original); err != nil {
		panic(err.Error())
	}
	merged := maps.Merge(original, override)
	mergedRaw, err := json.Marshal(merged)
	if err != nil {
		panic(err.Error())
	}
	if err := protojson.Unmarshal(mergedRaw, res.GetResource()); err != nil {
		panic(err.Error())
	}
}

// DeleteNestedFieldPath from a resource.
func DeleteNestedFieldPath(fields ...string) ResourceModifier {
	return func(res *fnapi.Resource) {
		resRaw, err := protojson.Marshal(res.GetResource())
		if err != nil {
			panic(err.Error())
		}
		data := map[string]interface{}{}
		if err := json.Unmarshal(resRaw, &data); err != nil {
			panic(err.Error())
		}
		unstructured.RemoveNestedField(data, fields...)
		updateRaw, err := json.Marshal(data)
		if err != nil {
			panic(err.Error())
		}
		if err := protojson.Unmarshal(updateRaw, res.GetResource()); err != nil {
			panic(err.Error())
		}
	}
}

// ExpectDesiredCompositeObject expects the given [runtime.Object] as desired
// composite as result of the function.
func ExpectDesiredCompositeObject(o runtime.Object, mods ...ResourceModifier) TestFunctionOpt {
	return func(tc *FunctionTest) {
		res := &fnapi.Resource{
			Resource: mustObjectAsStruct(o),
		}
		for _, m := range mods {
			m(res)
		}
		tc.res.Desired.Composite = res
	}
}

// ExpectDesiredCompositeYAML is the same as [ExpectDesiredCompositeObject] but
// reads the object from a single YAML document.
func ExpectDesiredCompositeYAML(rawYAML []byte, mods ...ResourceModifier) TestFunctionOpt {
	return ExpectDesiredCompositeObject(mustUnstructuredFromYAML(rawYAML), mods...)
}

// ExpectDesiredCompositeJSON is the same as [ExpectDesiredCompositeObject] but
// reads the object from a JSON document.
func ExpectDesiredCompositeJSON(rawJSON []byte, mods ...ResourceModifier) TestFunctionOpt {
	return ExpectDesiredCompositeObject(mustUnstructuredFromJSON(rawJSON), mods...)
}

// ExpectDesiredResourceObject adds an object to the expected outcome of a
// function.
func ExpectDesiredResourceObject(name string, o runtime.Object, mods ...ResourceModifier) TestFunctionOpt {
	return func(tc *FunctionTest) {
		res := &fnapi.Resource{
			Resource: mustObjectAsStruct(o),
		}
		for _, m := range mods {
			m(res)
		}
		tc.res.Desired.Resources[name] = res
	}
}

// ExpectDesiredResourceYAML is the same as [ExpectDesiredResourceObject] but
// reads the object from a single YAML document.
func ExpectDesiredResourceYAML(name string, rawYAML []byte, mods ...ResourceModifier) TestFunctionOpt {
	return ExpectDesiredResourceObject(name, mustUnstructuredFromYAML(rawYAML), mods...)
}

// ExpectDesiredResourceJSON is the same as [ExpectDesiredResourceObject] but
// reads the object from a JSON document.
func ExpectDesiredResourceJSON(name string, rawJSON []byte, mods ...ResourceModifier) TestFunctionOpt {
	return ExpectDesiredResourceObject(name, mustUnstructuredFromJSON(rawJSON), mods...)
}

// IgnoreDesiredResources removes the resources from the expected desired
// resource response. If no resource with a given name does exist it is a noop.
func IgnoreDesiredResources(names ...string) TestFunctionOpt {
	return func(tc *FunctionTest) {
		for _, n := range names {
			delete(tc.res.GetDesired().GetResources(), n)
		}
	}
}

// IgnoreDesiredResourcesInResponse removes resources from the functions generated
// response before comparing the response with the expected desired resources.
// Use it to ignore desired resources whose existence and content you don't
// want to verify, e.g. statically generated ones.
func IgnoreDesiredResourcesInResponse(resourceNames ...string) TestFunctionOpt {
	return func(tc *FunctionTest) {
		tc.ignoredDesired = append(tc.ignoredDesired, resourceNames...)
	}
}

// ExpectedDesiredResourcesYAML reads all objects from a multi-document YAML and
// expected them as desired resources from the function.
//
// It uses the annotation [AnnotationKeyResourceName] to determine
// the name of the resource.
func ExpectDesiredResourcesYAML(rawYAML []byte, mods ...ResourceModifier) TestFunctionOpt {
	return func(tc *FunctionTest) {
		uList, err := yaml.UnmarshalObjects[*unstructured.Unstructured](rawYAML)
		if err != nil {
			panic(err.Error())
		}
		for _, u := range uList {
			key := GetTestResourceName(u)
			if key == "" {
				panic("resource has no name annotation")
			}
			meta.RemoveAnnotations(u, AnnotationKeyResourceName)

			str := mustObjectAsStruct(u)
			res := &fnapi.Resource{
				Resource: str,
				// TODO: Set connection details and ready state
			}
			for _, mod := range mods {
				mod(res)
			}
			tc.res.Desired.Resources[key] = res
		}
	}
}

// ExpectDesiredResourcesYAMLOverride loads the given objects from YAML and
// merges them with existing desired objects.
// It only modifies resources that are already desired.
func ExpectDesiredResourcesYAMLOverride(rawYAML []byte) TestFunctionOpt {
	return func(tc *FunctionTest) {
		if tc.res.GetDesired() == nil || len(tc.res.GetDesired().GetResources()) == 0 {
			return
		}
		uList, err := yaml.UnmarshalObjects[*unstructured.Unstructured](rawYAML)
		if err != nil {
			panic(err.Error())
		}
		for _, u := range uList {
			key := GetTestResourceName(u)
			if key == "" {
				panic("override resource has no name annotation")
			}
			meta.RemoveAnnotations(u, AnnotationKeyResourceName)

			desiredRes, hasDesiredResource := tc.res.GetDesired().GetResources()[key]
			if !hasDesiredResource {
				continue
			}
			resRaw, err := protojson.Marshal(desiredRes.GetResource())
			if err != nil {
				panic(errors.Wrap(err, key).Error())
			}
			original := map[string]interface{}{}
			if err := json.Unmarshal(resRaw, &original); err != nil {
				panic(errors.Wrap(err, key).Error())
			}
			u.Object = maps.Merge(original, u.Object)
			str := mustObjectAsStruct(u)
			res := &fnapi.Resource{
				Resource:          str,
				ConnectionDetails: desiredRes.GetConnectionDetails(),
				Ready:             desiredRes.GetReady(),
			}
			tc.res.Desired.Resources[key] = res
		}
	}
}

// ExpectResults expects a list of [fnapi.Result] from a function.
func ExpectResults(results []*fnapi.Result) TestFunctionOpt {
	return func(tc *FunctionTest) { tc.res.Results = results }
}

// ExpectError expects an error from a TestFunctionOpt.
func ExpectError(err error) TestFunctionOpt {
	return func(tc *FunctionTest) { tc.err = err }
}
