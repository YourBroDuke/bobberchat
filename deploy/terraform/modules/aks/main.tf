resource "azurerm_kubernetes_cluster" "aks" {
  name                = var.cluster_name
  location            = var.location
  resource_group_name = var.resource_group_name
  dns_prefix          = "${var.cluster_name}-dns"
  kubernetes_version  = var.kubernetes_version
  sku_tier            = var.sku_tier

  default_node_pool {
    name            = "default"
    node_count      = var.node_count
    vm_size         = var.vm_size
    vnet_subnet_id  = var.aks_subnet_id
    os_disk_size_gb = var.os_disk_size_gb
  }

  network_profile {
    network_plugin      = "azure"
    network_plugin_mode = "overlay"
    service_cidr        = var.service_cidr
    dns_service_ip      = var.dns_service_ip
  }

  identity {
    type = "SystemAssigned"
  }

  tags = var.tags
}
