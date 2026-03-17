resource_group_name  = "rg-bobberchat-production"
location             = "southeastasia"
vnet_name            = "vnet-bobberchat-production"
vnet_cidr            = "10.2.0.0/16"
aks_subnet_cidr      = "10.2.1.0/24"
postgres_subnet_cidr = "10.2.2.0/24"

cluster_name       = "aks-bobberchat-production"
kubernetes_version = "1.33"
node_count         = 1
vm_size            = "Standard_B2als_v2"
os_disk_size_gb    = 30
sku_tier           = "Free"
service_cidr       = "10.3.0.0/16"
dns_service_ip     = "10.3.0.10"

postgres_server_name                  = "postgres-bobberchat-production"
postgres_admin_login                  = "bobberchat"
postgres_sku_name                     = "B_Standard_B1ms"
postgres_storage_mb                   = 65536
postgres_storage_tier                 = "P6"
postgres_backup_retention_days        = 7
postgres_geo_redundant_backup_enabled = false
postgres_high_availability_mode       = "Disabled"
postgres_database_name                = "bobberchat"

create_dns_zone      = false # DNS zone already created in staging environment
domain_name          = "bobbers.cc"
staging_subdomain    = "staging"
production_subdomain = "api"
ingress_external_ip  = "20.24.177.45"
dns_record_ttl       = 300

additional_tags = {}
