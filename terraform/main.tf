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

locals {
  sabuild   = "${var.project_number}@cloudbuild.gserviceaccount.com"
  sacompute = "${var.project_number}-compute@developer.gserviceaccount.com"
}

# Handle services
resource "google_project_service" "all" {
  for_each           = toset(var.gcp_service_list)
  project            = var.project_number
  service            = each.key
  disable_on_destroy = false
}

# Handle Permissions
resource "google_project_iam_member" "allbuild" {
  for_each   = toset(var.build_roles_list)
  project    = var.project_number
  role       = each.key
  member     = "serviceAccount:${local.sabuild}"
  depends_on = [google_project_service.all]
}


# Handle storage bucket
resource "google_storage_bucket" "target_bucket" {
  name     = var.bucket
  project  = var.project_number
  location = var.location
}

resource "google_storage_bucket" "function_bucket" {
  name     = "${var.project_id}-function-deployer"
  project  = var.project_number
  location = var.location
}


# Handle artifact registry
resource "google_artifact_registry_repository" "app" {
  provider      = google-beta
  format        = "DOCKER"
  location      = var.region
  project       = var.project_id
  repository_id = "${var.basename}-app"
  depends_on    = [google_project_service.all]
}




resource "null_resource" "cloudbuild_function" {
  provisioner "local-exec" {
    command = <<-EOT
    cp ../code/function/function.go .
    cp ../code/function/go.mod .
    zip ../index.zip function.go
    zip ../index.zip go.mod
    rm go.mod
    rm function.go
    EOT
  }

  depends_on = [
    google_project_service.all
  ]
}

resource "null_resource" "cloudbuild_app" {
  provisioner "local-exec" {
    working_dir = "${path.module}/../code/app"
    command     = "gcloud builds submit . --substitutions=_REGION=${var.region},_BASENAME=${var.basename}  --project ${var.project_id}"
  }

  depends_on = [
    google_artifact_registry_repository.app,
    google_project_service.all
  ]
}

resource "google_cloud_run_service" "app" {
  name     = "${var.basename}-app"
  location = var.region
  project  = var.project_id

  template {
    spec {
      containers {
        image = "${var.region}-docker.pkg.dev/${var.project_id}/${var.basename}-app/prod"
        env {
          name  = "BUCKET"
          value = var.bucket
        }
      }
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/maxScale" = "1000"
        "run.googleapis.com/client-name"   = "terraform"
      }
    }
  }
  autogenerate_revision_name = true
  depends_on = [
    null_resource.cloudbuild_app,
  ]
}

data "google_iam_policy" "noauth" {
  binding {
    role = "roles/run.invoker"
    members = [
      "allUsers",
    ]
  }
}

resource "google_cloud_run_service_iam_policy" "noauth_app" {
  location    = google_cloud_run_service.app.location
  project     = google_cloud_run_service.app.project
  service     = google_cloud_run_service.app.name
  policy_data = data.google_iam_policy.noauth.policy_data
}



resource "google_storage_bucket_object" "archive" {
  name   = "index.zip"
  bucket = google_storage_bucket.function_bucket.name
  source = "../index.zip"
  depends_on = [
    google_project_service.all,
    google_storage_bucket.function_bucket,
    null_resource.cloudbuild_function
  ]
}

resource "google_cloudfunctions_function" "function" {
  name    = var.basename
  project = var.project_id
  region  = var.region

  runtime = "go116"

  available_memory_mb   = 128
  source_archive_bucket = google_storage_bucket.function_bucket.name
  source_archive_object = google_storage_bucket_object.archive.name
  entry_point           = "OnFileUpload"
  event_trigger {
    event_type = "google.storage.object.finalize"
    resource   = google_storage_bucket.target_bucket.name
  }

  depends_on = [
    google_storage_bucket.function_bucket,
    google_storage_bucket.target_bucket,
    google_storage_bucket_object.archive,
    google_project_service.all
  ]
}
