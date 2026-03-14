# General variables
variable "resource_group_name" {
  description = "The name of the Azure resource group"
  type        = string
}

variable "location" {
  description = "The Azure region for resources"
  type        = string
  default     = "eastus"
}

# Network variables
variable "vnet_name" {
  description = "The name of the virtual network"
  type        = string
}

variable "vnet_cidr" {
  description = "The CIDR block for the virtual network"
  type        = string
  default     = "10.0.0.0/16"
}

# AKS variables
variable "cluster_name" {
  description = "The name of the AKS cluster"
  type        = string
}

variable "kubernetes_version" {
  description = "Kubernetes version for the AKS cluster"
  type        = string
  default     = "1.30"
}

variable "node_count" {
  description = "Number of nodes in the AKS cluster"
  type        = number
  default     = 2
}

variable "vm_size" {
  description = "VM size for AKS nodes"
  type        = string
  default     = "Standard_D2s_v5"
}

variable "os_disk_size_gb" {
  description = "OS disk size in GB"
  type        = number
  default     = 30
}

variable "sku_tier" {
  description = "SKU tier for AKS (Free, Standard, Premium)"
  type        = string
  default     = "Standard"
}

variable "service_cidr" {
  description = "CIDR block for Kubernetes services"
  type        = string
  default     = "10.1.0.0/16"
}

variable "dns_service_ip" {
  description = "IP address for DNS service"
  type        = string
  default     = "10.1.0.10"
}

# PostgreSQL variables
variable "postgres_server_name" {
  description = "The name of the PostgreSQL server"
  type        = string
}

variable "postgres_admin_login" {
  description = "PostgreSQL administrator username"
  type        = string
  default     = "bobberchat"
}

variable "postgres_admin_password" {
  description = "PostgreSQL administrator password"
  type        = string
  sensitive   = true
}

variable "postgres_sku_name" {
  description = "PostgreSQL SKU name"
  type        = string
  default     = "GP_Standard_D2ds_v5"
}

variable "postgres_storage_mb" {
  description = "PostgreSQL storage size in MB"
  type        = number
  default     = 65536
}

variable "postgres_storage_tier" {
  description = "PostgreSQL storage tier"
  type        = string
  default     = "P4"
}

variable "postgres_backup_retention_days" {
  description = "Backup retention period in days"
  type        = number
  default     = 7
}

variable "postgres_geo_redundant_backup_enabled" {
  description = "Enable geo-redundant backups"
  type        = bool
  default     = false
}

variable "postgres_high_availability_mode" {
  description = "High availability mode for PostgreSQL"
  type        = string
  default     = "ZoneRedundant"
}

variable "postgres_database_name" {
  description = "PostgreSQL database name"
  type        = string
  default     = "bobberchat"
}

# DNS variables
variable "create_dns_zone" {
  description = "Whether to create a DNS zone"
  type        = bool
  default     = true
}

variable "domain_name" {
  description = "The domain name for DNS"
  type        = string
}

variable "staging_subdomain" {
  description = "Subdomain for staging"
  type        = string
  default     = "staging"
}

variable "production_subdomain" {
  description = "Subdomain for production"
  type        = string
  default     = "api"
}

variable "ingress_external_ip" {
  description = "External IP for ingress"
  type        = string
  default     = ""
}

variable "dns_record_ttl" {
  description = "DNS record TTL in seconds"
  type        = number
  default     = 300
}

# Additional tags
variable "additional_tags" {
  description = "Additional tags to apply to all resources"
  type        = map(string)
  default     = {}
}
