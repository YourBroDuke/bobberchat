variable "resource_group_name" {
  description = "Name of the Azure resource group for Terraform state backend"
  type        = string
  default     = "rg-bobberchat-tfstate"
}

variable "location" {
  description = "Azure region for the Terraform state backend resources"
  type        = string
  default     = "East US"
}

variable "storage_account_name" {
  description = "Name of the Azure storage account for Terraform state files (must be globally unique, lowercase alphanumeric)"
  type        = string
  default     = "stbobberchatstate"
}

variable "container_name" {
  description = "Name of the blob container within the storage account for Terraform state files"
  type        = string
  default     = "tfstate"
}
