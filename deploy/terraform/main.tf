# ──────────────────────────────────────────────────────────────────────────────
# EXAMPLE / SKELETON — GCP Cloud Run wiring for the closed Kamisato topology.
#
# This is a readable map of the resources and how they connect, not a turnkey
# module. Some attributes are illustrative; treat every value as a starting
# point and reconcile with the provider docs before applying.
#
# Big caveat about miko on GCP: miko builds packages by talking to a Docker
# daemon (Docker-out-of-Docker via a mounted docker.sock in the compose
# deployment). Cloud Run gives you no Docker socket and no nested containers, so
# the `container` executor cannot run there as-is. A real GCP build backend for
# miko would be Cloud Build, or a GCE VM running the compose stack, fronted by
# the identity/network wiring shown below. What this skeleton demonstrates is the
# network boundary and the IAM identity — the "miko is private, only ayato may
# call it, both share one secret" shape — independent of where miko's heavy
# lifting actually executes.
# ──────────────────────────────────────────────────────────────────────────────

terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.0"
    }
  }
}

provider "google" {
  project = var.project
  region  = var.region
}

# ── Service accounts ──────────────────────────────────────────────────────────
# Each service runs as its own identity so that ayato→miko access can be granted
# narrowly (least privilege) instead of via a shared default account.

resource "google_service_account" "ayato" {
  account_id   = "kamisato-ayato"
  display_name = "Kamisato ayato (edge / distribution)"
}

resource "google_service_account" "miko" {
  account_id   = "kamisato-miko"
  display_name = "Kamisato miko (internal build server)"
}

# ── Shared API key (second layer of defense) ─────────────────────────────────
# One secret, referenced by both services: ayato sends it, miko requires it.
# Even with Cloud Run's "internal" ingress on miko, the key is the wall that
# survives a misconfigured network boundary. Create the secret here; populate
# its value out of band (CI, `gcloud secrets versions add`, etc.).

resource "google_secret_manager_secret" "build_api_key" {
  secret_id = "kamisato-build-api-key"
  replication {
    auto {}
  }
}

# Both runtime identities may read the secret value.
resource "google_secret_manager_secret_iam_member" "ayato_reads_key" {
  secret_id = google_secret_manager_secret.build_api_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.ayato.email}"
}

resource "google_secret_manager_secret_iam_member" "miko_reads_key" {
  secret_id = google_secret_manager_secret.build_api_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.miko.email}"
}
