// SPDX-FileCopyrightText: Copyright DB InfraGO AG and contributors
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"testing"

	fnapi "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	testRequestMetaTag = "go-test"
)

type TestFunctionOpt func(tc *FunctionTest)

func generateTc(fn fnapi.FunctionRunnerServiceServer) *FunctionTest {
	tc := &FunctionTest{
		fn: fn,
		req: &fnapi.RunFunctionRequest{
			Observed: &fnapi.State{
				Resources: map[string]*fnapi.Resource{},
			},
			Desired: &fnapi.State{
				Resources: map[string]*fnapi.Resource{},
			},
			ExtraResources: map[string]*fnapi.Resources{},
			Context: &structpb.Struct{
				Fields: map[string]*structpb.Value{},
			},
			Meta: &fnapi.RequestMeta{
				Tag: testRequestMetaTag,
			},
		},
		reqCtx: context.Background(),
		res: &fnapi.RunFunctionResponse{
			Desired: &fnapi.State{
				Resources: map[string]*fnapi.Resource{},
			},
			Context: &structpb.Struct{
				Fields: map[string]*structpb.Value{},
			},
			Meta: &fnapi.ResponseMeta{
				Tag: testRequestMetaTag,
			},
		},
	}
	return tc
}

func TestFunctionGetResult(t *testing.T, fn fnapi.FunctionRunnerServiceServer, opts ...TestFunctionOpt) *fnapi.RunFunctionResponse {
	tc := generateTc(fn)

	// Apply user options
	for _, o := range opts {
		o(tc)
	}

	res, err := tc.generateResponse()
	if err != nil {
		t.Fatal(errors.Wrapf(err, "cannot generate response"))
	}

	return res
}

func TestFunction(t *testing.T, fn fnapi.FunctionRunnerServiceServer, opts ...TestFunctionOpt) {
	tc := generateTc(fn)

	// Apply user options
	for _, o := range opts {
		o(tc)
	}

	res, err := tc.generateResponse()
	tc.compareResponseToExpectedResources(t, res, err)
}

type FunctionTest struct {
	fn fnapi.FunctionRunnerServiceServer

	req    *fnapi.RunFunctionRequest
	reqCtx context.Context
	res    *fnapi.RunFunctionResponse
	err    error

	ignoredDesired []string
}

func (tc *FunctionTest) generateResponse() (*fnapi.RunFunctionResponse, error) {
	res, err := tc.fn.RunFunction(tc.reqCtx, tc.req)

	if res == nil {
		res = &fnapi.RunFunctionResponse{}
	}
	if res.GetDesired() == nil {
		res.Desired = &fnapi.State{}
	}

	for _, n := range tc.ignoredDesired {
		delete(res.GetDesired().GetResources(), n)
	}

	return res, err
}

func (tc *FunctionTest) compareResponseToExpectedResources(t *testing.T, res *fnapi.RunFunctionResponse, err error) {
	if diff := cmp.Diff(convertResourceToUnstructured(tc.res.GetDesired().GetComposite()), convertResourceToUnstructured(res.GetDesired().GetComposite())); diff != "" {
		t.Errorf("res.Desired.Composite: -want +got\n%s\n", diff)
	}
	if diff := cmp.Diff(convertResourcesMapToUnstructured(tc.res.GetDesired().GetResources()), convertResourcesMapToUnstructured(res.GetDesired().GetResources())); diff != "" {
		t.Errorf("res.Desired.Resources: -want +got\n%s\n", diff)
	}
	if diff := cmp.Diff(convertResultsToMap(tc.res.GetResults()), convertResultsToMap(res.GetResults())); diff != "" {
		t.Errorf("Results: -want +got\n%s\n", diff)
		for i, r := range res.GetResults() {
			t.Errorf("Result %d: %s: %s", i, r.GetSeverity().String(), r.GetMessage())
		}
	}
	if diff := cmp.Diff(tc.err, err); diff != "" {
		t.Errorf("Error: -want +got\n%s\n", diff)
	}
}
