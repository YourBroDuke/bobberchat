variable "resource_group_name" {
  description = "The name of the resource group where the virtual network will be created"
  type        = string
}

variable "location" {
  description = "The Azure region where the virtual network will be deployed"
  type        = string
}

variable "vnet_name" {
  description = "The name of the virtual network"
  type        = string
}

variable "vnet_cidr" {
  description = "The CIDR block for the virtual network"
  type        = string
  default     = "10.0.0.0/16"
}

variable "aks_subnet_name" {
  description = "The name of the AKS subnet"
  type        = string
  default     = "aks-subnet"
}

variable "aks_subnet_cidr" {
  description = "The CIDR block for the AKS subnet"
  type        = string
  default     = "10.0.1.0/24"
}

variable "postgres_subnet_name" {
  description = "The name of the PostgreSQL subnet (delegated for Flexible Server)"
  type        = string
  default     = "postgres-subnet"
}

variable "postgres_subnet_cidr" {
  description = "The CIDR block for the PostgreSQL subnet"
  type        = string
  default     = "10.0.2.0/24"
}

variable "tags" {
  description = "A map of tags to apply to all resources"
  type        = map(string)
  default     = {}
}
