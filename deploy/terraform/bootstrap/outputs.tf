output "resource_group_name" {
  description = "Name of the resource group containing Terraform state backend"
  value       = azurerm_resource_group.tfstate.name
}

output "storage_account_name" {
  description = "Name of the storage account for Terraform state files"
  value       = azurerm_storage_account.tfstate.name
}

output "container_name" {
  description = "Name of the blob container for Terraform state files"
  value       = azurerm_storage_container.tfstate.name
}

output "storage_account_id" {
  description = "Azure resource ID of the storage account"
  value       = azurerm_storage_account.tfstate.id
}
