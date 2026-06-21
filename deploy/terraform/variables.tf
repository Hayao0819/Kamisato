# Inputs for the GCP example. Override per environment.

variable "project" {
  description = "GCP project ID."
  type        = string
}

variable "region" {
  description = "Region for both Cloud Run services."
  type        = string
  default     = "asia-northeast1"
}

variable "ayato_image" {
  description = "Container image for ayato (entrypoint runs `kamisato ayato`)."
  type        = string
}

variable "miko_image" {
  description = "Container image for miko (entrypoint runs `kamisato miko`)."
  type        = string
}
