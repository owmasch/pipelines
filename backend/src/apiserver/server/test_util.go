// Copyright 2018-2022 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	api "github.com/kubeflow/pipelines/backend/api/v1beta1/go_client"
	"github.com/kubeflow/pipelines/backend/src/apiserver/client"
	"github.com/kubeflow/pipelines/backend/src/apiserver/common"
	"github.com/kubeflow/pipelines/backend/src/apiserver/model"
	"github.com/kubeflow/pipelines/backend/src/apiserver/resource"
	"github.com/kubeflow/pipelines/backend/src/common/util"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	invalidPipelineVersionId = "not_exist_pipeline_version"
)

var testWorkflowPatch = util.NewWorkflow(&v1alpha1.Workflow{
	TypeMeta:   v1.TypeMeta{APIVersion: "argoproj.io/v1alpha1", Kind: "Workflow"},
	ObjectMeta: v1.ObjectMeta{Name: "workflow-name", UID: "workflow2"},
	Spec: v1alpha1.WorkflowSpec{
		Entrypoint: "testy",
		Templates: []v1alpha1.Template{v1alpha1.Template{
			Name: "testy",
			Container: &corev1.Container{
				Image:   "docker/whalesay",
				Command: []string{"cowsay"},
				Args:    []string{"hello world"},
			},
		}},
		Arguments: v1alpha1.Arguments{Parameters: []v1alpha1.Parameter{{Name: "param1"}, {Name: "param2"}}}},
})

var testWorkflow = util.NewWorkflow(&v1alpha1.Workflow{
	TypeMeta:   v1.TypeMeta{APIVersion: "argoproj.io/v1alpha1", Kind: "Workflow"},
	ObjectMeta: v1.ObjectMeta{Name: "workflow-name", UID: "workflow1", Namespace: "ns1"},
	Spec: v1alpha1.WorkflowSpec{
		Entrypoint: "testy",
		Templates: []v1alpha1.Template{v1alpha1.Template{
			Name: "testy",
			Container: &corev1.Container{
				Image:   "docker/whalesay",
				Command: []string{"cowsay"},
				Args:    []string{"hello world"},
			},
		}},
		Arguments: v1alpha1.Arguments{Parameters: []v1alpha1.Parameter{{Name: "param1"}}}},
	Status: v1alpha1.WorkflowStatus{Phase: v1alpha1.WorkflowRunning},
})

var validReference = []*api.ResourceReference{
	{
		Key: &api.ResourceKey{
			Type: api.ResourceType_EXPERIMENT, Id: resource.DefaultFakeUUID},
		Relationship: api.Relationship_OWNER,
	},
}

var validReferencesOfExperimentAndPipelineVersion = []*api.ResourceReference{
	{
		Key: &api.ResourceKey{
			Type: api.ResourceType_EXPERIMENT,
			Id:   resource.DefaultFakeUUID,
		},
		Relationship: api.Relationship_OWNER,
	},
	{
		Key: &api.ResourceKey{
			Type: api.ResourceType_PIPELINE_VERSION,
			Id:   resource.DefaultFakeUUID,
		},
		Relationship: api.Relationship_CREATOR,
	},
}

var referencesOfExperimentAndInvalidPipelineVersion = []*api.ResourceReference{
	{
		Key: &api.ResourceKey{
			Type: api.ResourceType_EXPERIMENT,
			Id:   resource.DefaultFakeUUID,
		},
		Relationship: api.Relationship_OWNER,
	},
	{
		Key:          &api.ResourceKey{Type: api.ResourceType_PIPELINE_VERSION, Id: invalidPipelineVersionId},
		Relationship: api.Relationship_CREATOR,
	},
}

var referencesOfInvalidPipelineVersion = []*api.ResourceReference{
	{
		Key:          &api.ResourceKey{Type: api.ResourceType_PIPELINE_VERSION, Id: invalidPipelineVersionId},
		Relationship: api.Relationship_CREATOR,
	},
}

// This automatically runs before all the tests.
func initEnvVars() {
	viper.Set(common.PodNamespace, "ns1")
}

func initWithExperiment(t *testing.T) (*resource.FakeClientManager, *resource.ResourceManager, *model.Experiment) {
	initEnvVars()
	clientManager := resource.NewFakeClientManagerOrFatal(util.NewFakeTimeForEpoch())
	resourceManager := resource.NewResourceManager(clientManager)
	apiExperiment := &api.Experiment{Name: "exp1"}
	if common.IsMultiUserMode() {
		apiExperiment = &api.Experiment{
			Name: "exp1",
			ResourceReferences: []*api.ResourceReference{
				{
					Key:          &api.ResourceKey{Type: api.ResourceType_NAMESPACE, Id: "ns1"},
					Relationship: api.Relationship_OWNER,
				},
			},
		}
	}
	experiment, err := resourceManager.CreateExperiment(apiExperiment)
	assert.Nil(t, err)
	return clientManager, resourceManager, experiment
}

func initWithExperiment_SubjectAccessReview_Unauthorized(t *testing.T) (*resource.FakeClientManager, *resource.ResourceManager, *model.Experiment) {
	initEnvVars()
	clientManager := resource.NewFakeClientManagerOrFatal(util.NewFakeTimeForEpoch())
	clientManager.SubjectAccessReviewClientFake = client.NewFakeSubjectAccessReviewClientUnauthorized()
	resourceManager := resource.NewResourceManager(clientManager)
	apiExperiment := &api.Experiment{Name: "exp1"}
	if common.IsMultiUserMode() {
		apiExperiment = &api.Experiment{
			Name: "exp1",
			ResourceReferences: []*api.ResourceReference{
				{
					Key:          &api.ResourceKey{Type: api.ResourceType_NAMESPACE, Id: "ns1"},
					Relationship: api.Relationship_OWNER,
				},
			},
		}
	}
	experiment, err := resourceManager.CreateExperiment(apiExperiment)
	assert.Nil(t, err)
	return clientManager, resourceManager, experiment
}

func initWithExperimentAndPipelineVersion(t *testing.T) (*resource.FakeClientManager, *resource.ResourceManager, *model.Experiment) {
	initEnvVars()
	clientManager := resource.NewFakeClientManagerOrFatal(util.NewFakeTimeForEpoch())
	resourceManager := resource.NewResourceManager(clientManager)

	// Create an experiment.
	apiExperiment := &api.Experiment{Name: "exp1"}
	experiment, err := resourceManager.CreateExperiment(apiExperiment)
	assert.Nil(t, err)

	// Create a pipeline and then a pipeline version.
	_, err = resourceManager.CreatePipeline("pipeline", "", "", []byte(testWorkflow.ToStringForStore()))
	assert.Nil(t, err)
	clientManager.UpdateUUID(util.NewFakeUUIDGeneratorOrFatal(resource.NonDefaultFakeUUID, nil))
	_, err = resourceManager.CreatePipelineVersion(&api.PipelineVersion{
		Name: "pipeline_version",
		ResourceReferences: []*api.ResourceReference{
			&api.ResourceReference{
				Key: &api.ResourceKey{
					Id:   resource.DefaultFakeUUID,
					Type: api.ResourceType_PIPELINE,
				},
				Relationship: api.Relationship_OWNER,
			},
		},
	},
		[]byte("apiVersion: argoproj.io/v1alpha1\nkind: Workflow"), true)

	return clientManager, resourceManager, experiment
}

func initWithExperimentsAndTwoPipelineVersions(t *testing.T) *resource.FakeClientManager {
	initEnvVars()
	clientManager := resource.NewFakeClientManagerOrFatal(util.NewFakeTimeForEpoch())
	resourceManager := resource.NewResourceManager(clientManager)

	// Create an experiment.
	apiExperiment := &api.Experiment{Name: "exp1"}
	_, err := resourceManager.CreateExperiment(apiExperiment)
	assert.Nil(t, err)

	// Create a pipeline and then a pipeline version.
	_, err = resourceManager.CreatePipeline("pipeline", "", "", []byte("apiVersion: argoproj.io/v1alpha1\nkind: Workflow"))
	assert.Nil(t, err)
	clientManager.UpdateUUID(util.NewFakeUUIDGeneratorOrFatal("123e4567-e89b-12d3-a456-426655441001", nil))
	resourceManager = resource.NewResourceManager(clientManager)
	_, err = resourceManager.CreatePipelineVersion(&api.PipelineVersion{
		Name: "pipeline_version",
		ResourceReferences: []*api.ResourceReference{
			&api.ResourceReference{
				Key: &api.ResourceKey{
					Id:   resource.DefaultFakeUUID,
					Type: api.ResourceType_PIPELINE,
				},
				Relationship: api.Relationship_OWNER,
			},
		},
	},
		[]byte("apiVersion: argoproj.io/v1alpha1\nkind: Workflow"), true)
	assert.Nil(t, err)
	clientManager.UpdateUUID(util.NewFakeUUIDGeneratorOrFatal(resource.NonDefaultFakeUUID, nil))
	resourceManager = resource.NewResourceManager(clientManager)
	// Create another pipeline and then pipeline version.
	_, err = resourceManager.CreatePipeline("anpther-pipeline", "", "", []byte("apiVersion: argoproj.io/v1alpha1\nkind: Workflow"))
	assert.Nil(t, err)

	clientManager.UpdateUUID(util.NewFakeUUIDGeneratorOrFatal("123e4567-e89b-12d3-a456-426655441002", nil))
	resourceManager = resource.NewResourceManager(clientManager)
	_, err = resourceManager.CreatePipelineVersion(&api.PipelineVersion{
		Name: "another_pipeline_version",
		ResourceReferences: []*api.ResourceReference{
			&api.ResourceReference{
				Key: &api.ResourceKey{
					Id:   resource.NonDefaultFakeUUID,
					Type: api.ResourceType_PIPELINE,
				},
				Relationship: api.Relationship_OWNER,
			},
		},
	},
		[]byte("apiVersion: argoproj.io/v1alpha1\nkind: Workflow"), true)
	assert.Nil(t, err)
	return clientManager
}

func initWithOneTimeRun(t *testing.T) (*resource.FakeClientManager, *resource.ResourceManager, *model.RunDetail) {
	clientManager, manager, exp := initWithExperiment(t)

	ctx := context.Background()
	if common.IsMultiUserMode() {
		md := metadata.New(map[string]string{common.GoogleIAPUserIdentityHeader: common.GoogleIAPUserIdentityPrefix + "user@google.com"})
		ctx = metadata.NewIncomingContext(context.Background(), md)
	}
	apiRun := &api.Run{
		Name: "run1",
		PipelineSpec: &api.PipelineSpec{
			WorkflowManifest: testWorkflow.ToStringForStore(),
			Parameters: []*api.Parameter{
				{Name: "param1", Value: "world"},
			},
		},
		ResourceReferences: []*api.ResourceReference{
			{
				Key:          &api.ResourceKey{Type: api.ResourceType_EXPERIMENT, Id: exp.UUID},
				Relationship: api.Relationship_OWNER,
			},
		},
	}
	runDetail, err := manager.CreateRun(ctx, apiRun)
	assert.Nil(t, err)
	return clientManager, manager, runDetail
}

// Util function to create an initial state with pipeline uploaded
func initWithPipeline(t *testing.T) (*resource.FakeClientManager, *resource.ResourceManager, *model.Pipeline) {
	initEnvVars()
	store := resource.NewFakeClientManagerOrFatal(util.NewFakeTimeForEpoch())
	manager := resource.NewResourceManager(store)
	p, err := manager.CreatePipeline("p1", "", "", []byte(testWorkflow.ToStringForStore()))
	assert.Nil(t, err)
	return store, manager, p
}

func AssertUserError(t *testing.T, err error, expectedCode codes.Code) {
	userError, ok := err.(*util.UserError)
	assert.True(t, ok)
	assert.Equal(t, expectedCode, userError.ExternalStatusCode())
}

func getPermissionDeniedError(userIdentity string, resourceAttributes *authorizationv1.ResourceAttributes) error {
	return util.NewPermissionDeniedError(
		errors.New("Unauthorized access"),
		"User '%s' is not authorized with reason: %s (request: %+v)",
		userIdentity,
		"this is not allowed",
		resourceAttributes,
	)
}

func wrapFailedAuthzApiResourcesError(err error) error {
	return util.Wrap(err, "Failed to authorize with API")
}

func wrapFailedAuthzRequestError(err error) error {
	return util.Wrap(err, "Failed to authorize the request")
}
