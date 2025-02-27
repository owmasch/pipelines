# Copyright 2022 The Kubeflow Authors. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from typing import Dict, List

from google_cloud_pipeline_components.types.artifact_types import BQMLModel
from kfp.dsl import ConcatPlaceholder
from kfp.dsl import container_component
from kfp.dsl import ContainerSpec
from kfp.dsl import Input
from kfp.dsl import OutputPath


@container_component
def bigquery_export_model_job(
    project: str,
    model: Input[BQMLModel],
    model_destination_path: str,
    # TODO(b/243411151): misalignment of arguments in documentation vs function
    # signature.
    exported_model_path: OutputPath(str),
    gcp_resources: OutputPath(str),
    location: str = 'us-central1',
    job_configuration_extract: Dict[str, str] = {},
    labels: Dict[str, str] = {},
):
  """Launch a BigQuery export model job and waits for it to finish.

    Args:
        project (str): Required. Project to run BigQuery model export job.
        location (Optional[str]): Location of the job to export the BigQuery
          model. If not set, default to `US` multi-region.  For more details,
          see
          https://cloud.google.com/bigquery/docs/locations#specifying_your_location
        model (google.BQMLModel): Required. BigQuery ML model to export.
          model_destination_path(str): Required. The gcs bucket to export the
          model to.
        job_configuration_extract (Optional[dict]): A json formatted string
          describing the rest of the job configuration.  For more details, see
          https://cloud.google.com/bigquery/docs/reference/rest/v2/Job#JobConfigurationQuery
        labels (Optional[dict]): The labels associated with this job. You can
          use these to organize and group your jobs. Label keys and values can
          be no longer than 63 characters, can only containlowercase letters,
          numeric characters, underscores and dashes. International characters
          are allowed. Label values are optional. Label keys must start with a
          letter and each label in the list must have a different key.
            Example: { "name": "wrench", "mass": "1.3kg", "count": "3" }.

    Returns:
        exported_model_path (str):
            The gcs bucket path where you export the model to.
        gcp_resources (str):
            Serialized gcp_resources proto tracking the BigQuery job.
            For more details, see
            https://github.com/kubeflow/pipelines/blob/master/components/google-cloud/google_cloud_pipeline_components/proto/README.md.
  """
  return ContainerSpec(
      image='gcr.io/ml-pipeline/google-cloud-pipeline-components:latest',
      command=[
          'python3', '-u', '-m',
          'google_cloud_pipeline_components.container.v1.bigquery.export_model.launcher'
      ],
      args=[
          '--type',
          'BigqueryExportModelJob',
          '--project',
          project,
          '--location',
          location,
          '--model_name',
          ConcatPlaceholder([
              "{{$.inputs.artifacts['model'].metadata['projectId']}}", '.',
              "{{$.inputs.artifacts['model'].metadata['datasetId']}}", '.',
              "{{$.inputs.artifacts['model'].metadata['modelId']}}"
          ]),
          '--model_destination_path',
          model_destination_path,
          '--payload',
          ConcatPlaceholder([
              '{', '"configuration": {', '"query": ', job_configuration_extract,
              ', "labels": ', labels, '}', '}'
          ]),
          '--exported_model_path',
          exported_model_path,
          '--gcp_resources',
          gcp_resources,
          '--executor_input',
          '{{$}}',
      ])
