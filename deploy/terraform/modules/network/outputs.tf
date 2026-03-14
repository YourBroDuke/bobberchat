output "vnet_id" {
  description = "The ID of the virtual network"
  value       = azurerm_virtual_network.main.id
}

output "vnet_name" {
  description = "The name of the virtual network"
  value       = azurerm_virtual_network.main.name
}

output "aks_subnet_id" {
  description = "The ID of the AKS subnet"
  value       = azurerm_subnet.aks.id
}

output "aks_subnet_name" {
  description = "The name of the AKS subnet"
  value       = azurerm_subnet.aks.name
}

output "postgres_subnet_id" {
  description = "The ID of the PostgreSQL subnet"
  value       = azurerm_subnet.postgres.id
}

output "postgres_subnet_name" {
  description = "The name of the PostgreSQL subnet"
  value       = azurerm_subnet.postgres.name
}
