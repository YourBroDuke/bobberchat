output "cluster_name" {
  description = "The name of the AKS cluster"
  value       = module.aks.cluster_name
}

output "cluster_fqdn" {
  description = "The FQDN of the AKS cluster"
  value       = module.aks.cluster_fqdn
}

output "kube_config" {
  description = "Raw kubeconfig for the AKS cluster"
  value       = module.aks.kube_config
  sensitive   = true
}

output "postgres_dsn" {
  description = "PostgreSQL connection string (sensitive)"
  value       = module.postgres.connection_dsn
  sensitive   = true
}

output "postgres_server_fqdn" {
  description = "Fully qualified domain name of the PostgreSQL server"
  value       = module.postgres.server_fqdn
}

output "dns_nameservers" {
  description = "Azure DNS nameservers for the domain"
  value       = module.dns.name_servers
}

output "dns_zone_id" {
  description = "The DNS zone ID"
  value       = module.dns.dns_zone_id
}

output "resource_group_name" {
  description = "The name of the resource group"
  value       = var.resource_group_name
}

output "vnet_id" {
  description = "The ID of the virtual network"
  value       = module.network.vnet_id
}

output "aks_subnet_id" {
  description = "The ID of the AKS subnet"
  value       = module.network.aks_subnet_id
}

output "postgres_subnet_id" {
  description = "The ID of the PostgreSQL subnet"
  value       = module.network.postgres_subnet_id
}
