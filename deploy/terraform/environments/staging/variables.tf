# Common Variables
variable "resource_group_name" {
  description = "The name of the Azure Resource Group"
  type        = string
  default     = "rg-bobberchat-staging"
}

variable "location" {
  description = "The Azure region for staging environment"
  type        = string
  default     = "eastus"
}

variable "additional_tags" {
  description = "Additional tags to apply to all resources"
  type        = map(string)
  default     = {}
}

# Network Variables
variable "vnet_name" {
  description = "The name of the virtual network"
  type        = string
  default     = "vnet-bobberchat-staging"
}

variable "vnet_cidr" {
  description = "The CIDR block for the virtual network"
  type        = string
  default     = "10.0.0.0/16"
}

# AKS Variables
variable "cluster_name" {
  description = "The name of the AKS cluster"
  type        = string
  default     = "aks-bobberchat-staging"
}

variable "kubernetes_version" {
  description = "Kubernetes version for AKS cluster"
  type        = string
  default     = "1.30"
}

variable "node_count" {
  description = "Number of nodes in AKS cluster"
  type        = number
  default     = 1
}

variable "vm_size" {
  description = "VM size for AKS nodes"
  type        = string
  default     = "Standard_D2s_v5"
}

variable "os_disk_size_gb" {
  description = "OS disk size in GB for AKS nodes"
  type        = number
  default     = 30
}

variable "sku_tier" {
  description = "SKU tier for AKS cluster (Free, Standard, Premium)"
  type        = string
  default     = "Free"
}

variable "service_cidr" {
  description = "CIDR block for Kubernetes services"
  type        = string
  default     = "10.1.0.0/16"
}

variable "dns_service_ip" {
  description = "IP address for DNS service in Kubernetes"
  type        = string
  default     = "10.1.0.10"
}

# PostgreSQL Variables
variable "postgres_server_name" {
  description = "The name of the PostgreSQL Flexible Server"
  type        = string
  default     = "postgres-bobberchat-staging"
}

variable "postgres_admin_login" {
  description = "Administrator username for PostgreSQL"
  type        = string
  default     = "bobberchat"
}

variable "postgres_admin_password" {
  description = "Administrator password for PostgreSQL"
  type        = string
  sensitive   = true
}

variable "postgres_sku_name" {
  description = "SKU name for PostgreSQL Flexible Server"
  type        = string
  default     = "B_Standard_B2s"
}

variable "postgres_storage_mb" {
  description = "Storage size in MB for PostgreSQL Flexible Server"
  type        = number
  default     = 32768 # 32GB
}

variable "postgres_storage_tier" {
  description = "Storage tier for PostgreSQL Flexible Server"
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
  description = "High availability mode (Disabled, ZoneRedundant, or SameZone)"
  type        = string
  default     = "Disabled"
}

variable "postgres_database_name" {
  description = "Name of the database to create"
  type        = string
  default     = "bobberchat"
}

# DNS Variables
variable "create_dns_zone" {
  description = "Whether to create the DNS zone"
  type        = bool
  default     = true
}

variable "domain_name" {
  description = "The custom domain name for the application"
  type        = string
}

variable "staging_subdomain" {
  description = "Subdomain for staging environment"
  type        = string
  default     = "staging"
}

variable "production_subdomain" {
  description = "Subdomain for production environment"
  type        = string
  default     = "api"
}

variable "ingress_external_ip" {
  description = "External IP address of the nginx-ingress LoadBalancer"
  type        = string
  default     = ""
}

variable "dns_record_ttl" {
  description = "Time To Live (TTL) for DNS A records in seconds"
  type        = number
  default     = 300
}
