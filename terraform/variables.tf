/**
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

variable "project_id" {
  type = string
}

variable "project_number" {
  type = string
}

variable "region" {
  type = string
}

variable "bucket" {
  type = string
}

variable "basename" {
  type = string
}

variable "location" {
  type = string
}

variable "gcp_service_list" {
  description = "The list of apis necessary for the project"
  type        = list(string)
  default = [
    "cloudbuild.googleapis.com",
    "storage.googleapis.com",
    "cloudfunctions.googleapis.com",
    "run.googleapis.com",
    "artifactregistry.googleapis.com",
  ]
}

variable "build_roles_list" {
  description = "The list of roles that build needs for"
  type        = list(string)
  default = [
    "roles/run.developer",
    "roles/iam.serviceAccountUser",
    "roles/iam.serviceAccountUser",
    "roles/run.admin",
    "roles/cloudfunctions.admin",
    "roles/artifactregistry.admin",
  ]
}
