output "cluster_name" {
  description = "Name of the AKS cluster"
  value       = azurerm_kubernetes_cluster.aks.name
}

output "cluster_id" {
  description = "Resource ID of the AKS cluster"
  value       = azurerm_kubernetes_cluster.aks.id
}

output "kube_config" {
  description = "Kubernetes configuration for accessing the cluster"
  value       = azurerm_kubernetes_cluster.aks.kube_config_raw
  sensitive   = true
}

output "host" {
  description = "Kubernetes API server host URL"
  value       = azurerm_kubernetes_cluster.aks.kube_config[0].host
}

output "cluster_identity_principal_id" {
  description = "Principal ID of the SystemAssigned identity for the AKS cluster"
  value       = azurerm_kubernetes_cluster.aks.identity[0].principal_id
}

output "cluster_fqdn" {
  description = "FQDN of the AKS cluster"
  value       = azurerm_kubernetes_cluster.aks.fqdn
}
