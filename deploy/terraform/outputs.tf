output "ayato_url" {
  description = "Public URL clients (lumine, ayaka) talk to."
  value       = google_cloud_run_v2_service.ayato.uri
}

output "miko_url" {
  description = "Internal-only URL ayato proxies to. Not reachable from outside the VPC."
  value       = google_cloud_run_v2_service.miko.uri
}
