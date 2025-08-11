<!--
 ~ SPDX-FileCopyrightText: Copyright DB InfraGO AG and contributors
 ~ SPDX-License-Identifier: Apache-2.0
 -->

# Crossplane Function Test Framework

A Go library that allows easy implementation of unit tests for Crossplane functions written in Go.

## Usage

Example Go test that runs a function in a blackbox test and compares the received with the expected output:

```go
package account

import (
	_ "embed"
	"testing"

	"github.com/crossplane/function-sdk-go/logging"
	fntesting "github.com/dbinfrago/crossplane-function-test-framework"

	"github.com/my-group/my-project/function"
)

var (
	//go:embed testdata/arg_composite.yaml
	observedComposite []byte
	//go:embed testdata/arg_observed_composed.yaml
	observedComposed []byte

	//go:embed testdata/expect_composed.yaml
	expectComposed []byte
	//go:embed testdata/expect_composite.yaml
	expectComposite []byte

)

const (
	subroutineName = "test-routine"
)

func TestSuccess(t *testing.T) {
	log := logging.NewNopLogger()
	fn := function.NewFunction(log)

	fntesting.TestFunction(
		t, fn,
		fntesting.WithObservedCompositeYAML(observedComposite),
		fntesting.WithObservedResourcesYAML(observedComposed),
		fntesting.ExpectDesiredCompositeYAML(expectComposite),
		fntesting.ExpectDesiredResourcesYAML(expectComposed),
	)
}
```

# Contributing

See our [Contributing Guidelines](./CONTRIBUTING.md).

# Licensing

Each file contains a license reference to one of the [included licenses](./LICENSES).
