resource_group_name = "rg-bobberchat-staging"
location            = "southeastasia"
vnet_name           = "vnet-bobberchat-staging"
vnet_cidr           = "10.0.0.0/16"

cluster_name       = "aks-bobberchat-staging"
kubernetes_version = "1.33"
node_count         = 1
vm_size            = "Standard_B2als_v2"
os_disk_size_gb    = 30
sku_tier           = "Free"
service_cidr       = "10.1.0.0/16"
dns_service_ip     = "10.1.0.10"

postgres_server_name                  = "postgres-bobberchat-staging"
postgres_admin_login                  = "bobberchat"
postgres_admin_password               = "/ETJfw8az2+MnCR3cFCFimudatBIEsr0oznu5t57nV8="
postgres_sku_name                     = "B_Standard_B1ms"
postgres_storage_mb                   = 32768
postgres_storage_tier                 = "P4"
postgres_backup_retention_days        = 7
postgres_geo_redundant_backup_enabled = false
postgres_high_availability_mode       = "Disabled"
postgres_database_name                = "bobberchat"

create_dns_zone      = false
domain_name          = "bobbers.cc"
staging_subdomain    = "staging"
production_subdomain = "api"
ingress_external_ip  = "20.197.123.66"
dns_record_ttl       = 300

additional_tags = {
  environment = "staging"
  project     = "bobberchat"
  managed_by  = "terraform"
}
