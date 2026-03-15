output "cluster_name" {
  description = "Name of the AKS cluster"
  value       = module.aks.cluster_name
}

output "cluster_id" {
  description = "Resource ID of the AKS cluster"
  value       = module.aks.cluster_id
}

output "cluster_fqdn" {
  description = "FQDN of the AKS cluster"
  value       = module.aks.cluster_fqdn
}

output "kube_config" {
  description = "Kubernetes configuration for accessing the cluster"
  value       = module.aks.kube_config
  sensitive   = true
}

output "host" {
  description = "Kubernetes API server host URL"
  value       = module.aks.host
}

output "cluster_identity_principal_id" {
  description = "Principal ID of the SystemAssigned identity for the AKS cluster"
  value       = module.aks.cluster_identity_principal_id
}

output "vnet_id" {
  description = "ID of the virtual network"
  value       = module.network.vnet_id
}

output "vnet_name" {
  description = "Name of the virtual network"
  value       = module.network.vnet_name
}

output "aks_subnet_id" {
  description = "ID of the AKS subnet"
  value       = module.network.aks_subnet_id
}

output "postgres_fqdn" {
  description = "FQDN of the PostgreSQL server"
  value       = module.postgres.server_fqdn
}

output "postgres_connection_dsn" {
  description = "PostgreSQL connection string (DSN)"
  value       = module.postgres.connection_dsn
  sensitive   = true
}

output "postgres_database_name" {
  description = "Name of the PostgreSQL database"
  value       = module.postgres.database_name
}

output "dns_zone_name" {
  description = "Name of the created DNS zone (empty if create_dns_zone=false)"
  value       = module.dns.dns_zone_name
}

output "name_servers" {
  description = "Name servers for the DNS zone (empty if create_dns_zone=false)"
  value       = module.dns.name_servers
}

output "resource_group_name" {
  description = "Name of the Azure resource group"
  value       = var.resource_group_name
}
