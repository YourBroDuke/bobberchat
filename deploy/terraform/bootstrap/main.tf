terraform {
  required_version = ">= 1.0"

  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
  }
}

provider "azurerm" {
  features {}
}

# Resource group for Terraform state backend
resource "azurerm_resource_group" "tfstate" {
  name     = var.resource_group_name
  location = var.location

  tags = {
    Environment = "terraform-state"
    ManagedBy   = "terraform"
  }
}

# Storage account for Terraform state files
resource "azurerm_storage_account" "tfstate" {
  name                          = var.storage_account_name
  resource_group_name           = azurerm_resource_group.tfstate.name
  location                      = azurerm_resource_group.tfstate.location
  account_tier                  = "Standard"
  account_replication_type      = "GRS"
  https_traffic_only_enabled    = true
  shared_access_key_enabled     = false
  public_network_access_enabled = false
  min_tls_version               = "TLS1_2"

  blob_properties {
    versioning_enabled = true
  }

  tags = {
    Environment = "terraform-state"
    ManagedBy   = "terraform"
  }

  depends_on = [azurerm_resource_group.tfstate]
}

# Blob container for Terraform state files
resource "azurerm_storage_container" "tfstate" {
  name                  = var.container_name
  storage_account_name  = azurerm_storage_account.tfstate.name
  container_access_type = "private"

  depends_on = [azurerm_storage_account.tfstate]
}
