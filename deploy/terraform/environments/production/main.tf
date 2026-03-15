terraform {
  required_version = ">= 1.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 3.0"
    }
  }
}

provider "azurerm" {
  features {
    virtual_machine {
      delete_os_disk_on_deletion     = true
      graceful_shutdown              = false
      skip_shutdown_and_force_delete = true
    }
  }
}

resource "azurerm_resource_group" "main" {
  name     = var.resource_group_name
  location = var.location
  tags     = local.tags
}

module "network" {
  source = "../../modules/network"

  resource_group_name = var.resource_group_name
  location            = var.location
  vnet_name           = var.vnet_name
  vnet_cidr           = var.vnet_cidr
  tags                = local.tags
}

module "aks" {
  source = "../../modules/aks"

  resource_group_name = var.resource_group_name
  location            = var.location
  cluster_name        = var.cluster_name
  kubernetes_version  = var.kubernetes_version
  node_count          = var.node_count
  vm_size             = var.vm_size
  os_disk_size_gb     = var.os_disk_size_gb
  sku_tier            = var.sku_tier
  aks_subnet_id       = module.network.aks_subnet_id
  service_cidr        = var.service_cidr
  dns_service_ip      = var.dns_service_ip
  tags                = local.tags
}

module "postgres" {
  source = "../../modules/postgres"

  resource_group_name          = var.resource_group_name
  location                     = var.location
  server_name                  = var.postgres_server_name
  administrator_login          = var.postgres_admin_login
  administrator_password       = var.postgres_admin_password
  sku_name                     = var.postgres_sku_name
  storage_mb                   = var.postgres_storage_mb
  storage_tier                 = var.postgres_storage_tier
  delegated_subnet_id          = module.network.postgres_subnet_id
  virtual_network_id           = module.network.vnet_id
  backup_retention_days        = var.postgres_backup_retention_days
  geo_redundant_backup_enabled = var.postgres_geo_redundant_backup_enabled
  high_availability_mode       = var.postgres_high_availability_mode
  database_name                = var.postgres_database_name
  tags                         = local.tags
}

module "dns" {
  source = "../../modules/dns"

  create_dns_zone      = var.create_dns_zone
  domain_name          = var.domain_name
  resource_group_name  = var.resource_group_name
  staging_subdomain    = var.staging_subdomain
  production_subdomain = var.production_subdomain
  ingress_external_ip  = var.ingress_external_ip
  dns_record_ttl       = var.dns_record_ttl
  tags                 = local.tags
}

locals {
  tags = merge(
    {
      environment = "production"
      project     = "bobberchat"
      managed_by  = "terraform"
    },
    var.additional_tags
  )
}
