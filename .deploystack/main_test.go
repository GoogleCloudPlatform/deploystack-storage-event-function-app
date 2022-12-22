// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/deploystack"
	"github.com/GoogleCloudPlatform/deploystack/dstester"
)

var (
	ops              = dstester.NewOperationsSet()
	project, _       = deploystack.ProjectID()
	projectNumber, _ = deploystack.ProjectNumber(project)
	basename         = "scaler"
	location         = "US"
	debug            = false
	region           = "us-central1"

	tf = dstester.Terraform{
		Dir: "../terraform",
		Vars: map[string]string{
			"project_id":     project,
			"project_number": projectNumber,
			"bucket":         fmt.Sprintf("%s-bucket", project),
			"region":         region,
			"location":       location,
			"basename":       basename,
		},
	}

	resources = dstester.Resources{
		Project: project,
		Items: []dstester.Resource{
			{
				Product:  "functions",
				Name:     basename,
				Expected: fmt.Sprintf("projects/%s/locations/%s/functions/%s", project, region, basename),
			},
			{
				Product: "run services",
				Name:    fmt.Sprintf("%s-app", basename),
				Arguments: map[string]string{
					"region": region,
				},
			},
			{
				Product:  "alpha storage buckets",
				Name:     fmt.Sprintf("gs://%s-bucket", project),
				Expected: fmt.Sprintf("%s-bucket", project),
			},
			{
				Product:  "alpha storage buckets",
				Name:     fmt.Sprintf("gs://%s-function-deployer", project),
				Expected: fmt.Sprintf("%s-function-deployer", project),
			},
			{
				Product:  "beta artifacts repositories",
				Name:     fmt.Sprintf("%s-app", basename),
				Expected: fmt.Sprintf("projects/%s/locations/%s/repositories/%s-app", project, region, basename),
				Arguments: map[string]string{
					"location": region,
				},
			},
		},
	}
)

func init() {
	if os.Getenv("debug") != "" {
		debug = true
	}

	ops.Add("postApply", dstester.Operation{Output: "endpoint", Type: "httpPoll"})
	ops.Add("postDestroy", dstester.Operation{Type: "sleep", Interval: 60})
}

func TestListCommands(t *testing.T) {
	resources.Init()
	dstester.DebugCommands(t, tf, resources)
}

func TestStack(t *testing.T) {
	dstester.TestStack(t, tf, resources, ops, debug)
}

func TestClean(t *testing.T) {
	if os.Getenv("clean") == "" {
		t.Skip("Clean must be very intentionally called")
	}

	resources.Init()
	dstester.Clean(t, tf, resources)
}
