variable "create_dns_zone" {
  description = "Whether to create the DNS zone. Set to false if managing DNS elsewhere."
  type        = bool
  default     = true
}

variable "domain_name" {
  description = "The custom domain name for the application (e.g., bobbers.cc)"
  type        = string
}

variable "resource_group_name" {
  description = "The name of the Azure Resource Group where DNS zone will be created"
  type        = string
}

variable "staging_subdomain" {
  description = "Subdomain for staging environment (e.g., 'staging' for staging.bobbers.cc)"
  type        = string
  default     = "staging"
}

variable "production_subdomain" {
  description = "Subdomain for production environment (e.g., 'api' for api.bobbers.cc)"
  type        = string
  default     = "api"
}

variable "ingress_external_ip" {
  description = "External IP address of the nginx-ingress LoadBalancer. A records created only when non-empty."
  type        = string
  default     = ""
}

variable "dns_record_ttl" {
  description = "Time To Live (TTL) for DNS A records in seconds"
  type        = number
  default     = 300
}

variable "tags" {
  description = "Tags to assign to all resources"
  type        = map(string)
  default     = {}
}
