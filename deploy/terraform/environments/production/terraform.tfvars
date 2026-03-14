resource_group_name = "rg-bobberchat-production"
location            = "eastus"
vnet_name           = "vnet-bobberchat-production"
vnet_cidr           = "10.0.0.0/16"

cluster_name       = "aks-bobberchat-production"
kubernetes_version = "1.30"
node_count         = 2
vm_size            = "Standard_D2s_v5"
os_disk_size_gb    = 30
sku_tier           = "Standard"
service_cidr       = "10.1.0.0/16"
dns_service_ip     = "10.1.0.10"

postgres_server_name                  = "postgres-bobberchat-production"
postgres_admin_login                  = "bobberchat"
postgres_sku_name                     = "GP_Standard_D2ds_v5"
postgres_storage_mb                   = 65536
postgres_storage_tier                 = "P4"
postgres_backup_retention_days        = 7
postgres_geo_redundant_backup_enabled = false
postgres_high_availability_mode       = "ZoneRedundant"
postgres_database_name                = "bobberchat"

create_dns_zone      = true
domain_name          = "bobbers.cc"
staging_subdomain    = "staging"
production_subdomain = "api"
ingress_external_ip  = ""
dns_record_ttl       = 300

additional_tags = {}
