variable "resource_group_name" {
  description = "Name of the resource group"
  type        = string
}

variable "location" {
  description = "Azure region for resources"
  type        = string
}

variable "server_name" {
  description = "Name of the PostgreSQL Flexible Server"
  type        = string
}

variable "administrator_login" {
  description = "Administrator username for PostgreSQL"
  type        = string
  default     = "bobberchat"
}

variable "administrator_password" {
  description = "Administrator password for PostgreSQL"
  type        = string
  sensitive   = true
}

variable "sku_name" {
  description = "SKU name for PostgreSQL Flexible Server (e.g., B_Standard_B2s for staging, GP_Standard_D4s_v3 for production)"
  type        = string
  default     = "B_Standard_B2s"
}

variable "storage_mb" {
  description = "Storage size in MB for PostgreSQL Flexible Server"
  type        = number
  default     = 32768 # 32GB
}

variable "storage_tier" {
  description = "Storage tier for PostgreSQL Flexible Server (P4, P6, P10, P15, P20, P30, P40, P50, P60, P70, P80)"
  type        = string
  default     = "P4"
}

variable "delegated_subnet_id" {
  description = "ID of the subnet delegated to Microsoft.DBforPostgreSQL/flexibleServers"
  type        = string
}

variable "virtual_network_id" {
  description = "ID of the virtual network for private DNS zone link"
  type        = string
}

variable "backup_retention_days" {
  description = "Backup retention period in days"
  type        = number
  default     = 7
}

variable "geo_redundant_backup_enabled" {
  description = "Enable geo-redundant backups"
  type        = bool
  default     = false
}

variable "high_availability_mode" {
  description = "High availability mode (Disabled, ZoneRedundant, or SameZone)"
  type        = string
  default     = "Disabled"

  validation {
    condition     = contains(["Disabled", "ZoneRedundant", "SameZone"], var.high_availability_mode)
    error_message = "high_availability_mode must be one of: Disabled, ZoneRedundant, SameZone"
  }
}

variable "zone" {
  description = "Availability zone for the PostgreSQL server"
  type        = string
  default     = "1"
}

variable "database_name" {
  description = "Name of the database to create"
  type        = string
  default     = "bobberchat"
}

variable "tags" {
  description = "Tags to apply to all resources"
  type        = map(string)
  default     = {}
}
