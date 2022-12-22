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
	"testing"

	"github.com/GoogleCloudPlatform/deploystack"
	"github.com/GoogleCloudPlatform/deploystack/dstester"
)

var (
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

	resources = dstester.GCPResources{
		Project: project,
		Items: []dstester.GCPResource{
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
				Product:  "beta artifacts repositories",
				Name:     fmt.Sprintf("%s-app", basename),
				Expected: fmt.Sprintf("projects/%s/locations/%s/repositories/%s-app", project, region, basename),
				Arguments: map[string]string{
					"location": region,
				},
			},
		},
	}

	checks = []dstester.Check{}
)

func TestCreateDestroy(t *testing.T) {
	resources.Init()
	tf.InitApplyForTest(t, debug)
	dstester.TextExistence(t, resources.Items)

	dstester.TestChecks(t, checks, tf)

	tf.DestroyForTest(t, debug)
	dstester.TextNonExistence(t, resources.Items)
}

func TestCreation(t *testing.T) {
	resources.Init()
	tf.InitApplyForTest(t, debug)
	dstester.TextExistence(t, resources.Items)
}

// func TestPolls(t *testing.T) {
// 	dstester.TestChecks(t, checks, tf)
// }

// func TestCreateAndPoll(t *testing.T) {
// 	TestCreation(t)
// 	TestPolls(t)
// }

func TestDestruction(t *testing.T) {
	tf.DestroyForTest(t, debug)
	dstester.TextNonExistence(t, resources.Items)
}
