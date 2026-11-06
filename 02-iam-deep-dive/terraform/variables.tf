variable "project_id" {
  type        = string
  description = "Target project. Set via TF_VAR_project_id."
}

variable "region" {
  type        = string
  description = "Region. Set via TF_VAR_region."
}

variable "oidc_issuer_uri" {
  type        = string
  description = "OIDC issuer trusted by the WIF pool (any provider whose JWTs you can mint)."
  default     = "https://token.actions.githubusercontent.com"
}

variable "oidc_subject" {
  type        = string
  description = "External subject (assertion.sub) allowed to impersonate the WIF service account."
  default     = "repo:example-org/example-repo:ref:refs/heads/main"
}

variable "enable_deny_policy" {
  type        = bool
  description = "Create the deny policy (denies storage.objects.delete to caller SA)."
  default     = true
}

variable "enable_org_policy" {
  type        = bool
  description = "Create project-level org policy constraint (blocks SA key creation)."
  default     = true
}
