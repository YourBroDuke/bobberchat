variable "resource_group_name" {
  description = "Name of the resource group where the AKS cluster will be created"
  type        = string
}

variable "location" {
  description = "Azure region where the AKS cluster will be deployed"
  type        = string
}

variable "cluster_name" {
  description = "Name of the AKS cluster"
  type        = string
}

variable "kubernetes_version" {
  description = "Kubernetes version to use for the AKS cluster"
  type        = string
  default     = "1.30"
}

variable "node_count" {
  description = "Number of nodes in the default node pool"
  type        = number
  default     = 2
}

variable "vm_size" {
  description = "VM size for the default node pool"
  type        = string
  default     = "Standard_D2s_v5"
}

variable "os_disk_size_gb" {
  description = "OS disk size in GB for nodes in the default node pool"
  type        = number
  default     = 30
}

variable "sku_tier" {
  description = "SKU tier for the AKS cluster (Free, Standard, Premium)"
  type        = string
  default     = "Free"
}

variable "aks_subnet_id" {
  description = "ID of the subnet where AKS nodes will be deployed"
  type        = string
}

variable "service_cidr" {
  description = "CIDR block for Kubernetes services"
  type        = string
  default     = "10.1.0.0/16"
}

variable "dns_service_ip" {
  description = "IP address within service_cidr for DNS service"
  type        = string
  default     = "10.1.0.10"
}

variable "tags" {
  description = "Tags to apply to the AKS cluster"
  type        = map(string)
  default     = {}
}
