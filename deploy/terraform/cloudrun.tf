# ── miko: internal-only build server ─────────────────────────────────────────
# ingress = "internal" means only callers inside the project's VPC / VPC-SC
# perimeter reach it; the public internet cannot. We also withhold the
# `run.invoker` role from allUsers, so a request still needs both a network path
# AND a Google-signed identity token — on top of miko's own API key.

resource "google_cloud_run_v2_service" "miko" {
  name     = "kamisato-miko"
  location = var.region
  ingress  = "INGRESS_TRAFFIC_INTERNAL_ONLY"

  template {
    service_account = google_service_account.miko.email

    containers {
      image = var.miko_image
      # Entrypoint runs `kamisato miko`; the image's command must select it.

      ports {
        container_port = 8081
      }

      env {
        name  = "MIKO_PORT"
        value = "8081"
      }

      # The shared key. NOTE: koanf's "_" delimiter cannot map an env var onto
      # the `api_keys` koanf tag, so in a real deployment miko reads this from a
      # mounted config file (see compose.yml / deploy/README.md) rather than the
      # plain env var shown here. Kept as an env reference to make the
      # secret→service dependency explicit in the graph.
      env {
        name = "KAMISATO_BUILD_API_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.build_api_key.secret_id
            version = "latest"
          }
        }
      }
    }
  }
}

# ── ayato: public edge / distribution ────────────────────────────────────────
# Clients (lumine, ayaka) hit ayato; ayato proxies build/job traffic to miko.
# Public ingress here; swap to INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER if you
# front it with an external HTTPS load balancer instead.

resource "google_cloud_run_v2_service" "ayato" {
  name     = "kamisato-ayato"
  location = var.region
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    service_account = google_service_account.ayato.email

    # Keep one instance warm: pacman aborts a download after 10s without a byte,
    # and a scale-from-zero cold start can exceed that before ayato binds its
    # listener. Billing stays request-based (cpu_idle left at its true default),
    # so the idle minimum instance is charged memory only, not CPU.
    scaling {
      min_instance_count = 1
    }

    # Direct VPC egress so ayato can reach miko's internal ingress. Without a
    # route into the VPC, "internal" ingress on miko has nothing to admit.
    # Provide your own subnet; a Serverless VPC Access connector is the
    # alternative if you are not using direct egress.
    #
    # vpc_access {
    #   network_interfaces {
    #     network    = "your-vpc"
    #     subnetwork = "your-subnet"
    #   }
    #   egress = "PRIVATE_RANGES_ONLY"
    # }

    containers {
      image = var.ayato_image
      # Entrypoint runs `kamisato ayato`.

      ports {
        container_port = 9000
      }

      env {
        name  = "AYATO_PORT"
        value = "9000"
      }

      # miko's internal Cloud Run URL. Available after apply; for internal
      # ingress this is typically reached via the run.app host over the VPC.
      env {
        name  = "AYATO_MIKO_URL"
        value = google_cloud_run_v2_service.miko.uri
      }

      # Same shared secret. Same koanf caveat as miko: a real deployment renders
      # `miko.api_key` into a config file rather than relying on this env var.
      env {
        name = "AYATO_MIKO_API_KEY"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.build_api_key.secret_id
            version = "latest"
          }
        }
      }
    }
  }
}

# ── IAM: only ayato may invoke miko ──────────────────────────────────────────
# The identity half of the boundary. ayato's service account gets run.invoker on
# miko; nobody else (and not allUsers) does. ayato's outbound requests to miko
# must carry an identity token for miko's audience — handle that token minting in
# ayato's proxy layer (metadata-server identity token).

resource "google_cloud_run_v2_service_iam_member" "ayato_invokes_miko" {
  project  = google_cloud_run_v2_service.miko.project
  location = google_cloud_run_v2_service.miko.location
  name     = google_cloud_run_v2_service.miko.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.ayato.email}"
}

# ayato itself is public-facing; expose it to unauthenticated callers (the app's
# own Basic auth guards the build endpoint). Drop this if you require IAM auth.
resource "google_cloud_run_v2_service_iam_member" "ayato_public" {
  project  = google_cloud_run_v2_service.ayato.project
  location = google_cloud_run_v2_service.ayato.location
  name     = google_cloud_run_v2_service.ayato.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
