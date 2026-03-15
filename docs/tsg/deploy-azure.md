# Deploy to Azure (AKS)

This guide covers the manual setup and troubleshooting steps for deploying BobberChat to Azure using Azure Kubernetes Service (AKS), Azure PostgreSQL Flexible Server, and Terraform.

## Prerequisites

The following tools must be installed and configured on your local machine:

- Azure CLI (`az`)
- Terraform CLI (1.5+)
- Helm 3.x
- kubectl
- GitHub Repository Admin access

## 1. Azure Subscription Setup

First, authenticate with Azure and set the active subscription:

```bash
az login
az account set --subscription <YOUR_SUBSCRIPTION_ID>
```

## 2. Service Principal & OIDC Federation

BobberChat uses GitHub Actions with OIDC for secure, passwordless authentication to Azure.

### Create the Application and Service Principal

```bash
# Create the AD Application
az ad app create --display-name "bobberchat-github-actions"

# Create the Service Principal (use the appId from the command above)
az ad sp create --id <APP_ID>

# Assign the Contributor role at the subscription level
az role assignment create --assignee <SP_ID> --role "Contributor" --scope /subscriptions/<SUB_ID>
```

### Configure Federated Credentials

Create federated credentials to allow GitHub Actions to acquire tokens for your staging and production environments:

```bash
# For Staging
az ad app federated-credential create --id <APP_ID> --parameters '{
  "name": "bobberchat-staging",
  "issuer": "https://token.actions.githubusercontent.com",
  "subject": "repo:yourbroduke/bobberchat:environment:staging",
  "audiences": ["api://AzureADTokenExchange"]
}'

# For Production
az ad app federated-credential create --id <APP_ID> --parameters '{
  "name": "bobberchat-production",
  "issuer": "https://token.actions.githubusercontent.com",
  "subject": "repo:yourbroduke/bobberchat:environment:production",
  "audiences": ["api://AzureADTokenExchange"]
}'
```

## 3. Domain Registration

1. Register a domain through your preferred registrar.
2. After running the Terraform bootstrap and environment setup, obtain the Azure DNS nameservers from the Terraform output.
3. Delegate your DNS to these nameservers at your registrar.

## 4. Terraform Bootstrap

Initialize the remote backend storage account. This is a one-time setup.

```bash
cd deploy/terraform/bootstrap/
terraform init
terraform apply
```

Record the `storage_account_name` and `resource_group_name` from the output.

## 5. Terraform Infrastructure Setup

Deploy the core infrastructure (VNet, AKS, PostgreSQL, DNS).

### Staging Environment

```bash
cd deploy/terraform/environments/staging/
terraform init \
  -backend-config="resource_group_name=rg-bobberchat-tfstate" \
  -backend-config="storage_account_name=<STORAGE_ACCT_FROM_BOOTSTRAP>" \
  -backend-config="container_name=tfstate" \
  -backend-config="key=staging.tfstate"

terraform apply
```

### Production Environment

```bash
cd deploy/terraform/environments/production/
terraform init \
  -backend-config="resource_group_name=rg-bobberchat-tfstate" \
  -backend-config="storage_account_name=<STORAGE_ACCT_FROM_BOOTSTRAP>" \
  -backend-config="container_name=tfstate" \
  -backend-config="key=production.tfstate"

terraform apply
```

## 6. Kubernetes Cluster Configuration

Connect to your AKS cluster:

```bash
az aks get-credentials --resource-group <RG_NAME> --name <CLUSTER_NAME>
```

### Install cert-manager

```bash
helm repo add jetstack https://charts.jetstack.io
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set crds.enabled=true
```

### Install nginx-ingress

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --create-namespace
```

## 7. DNS and SSL Configuration

### Create DNS A Records

Obtain the external IP of the ingress controller:

```bash
kubectl get svc -n ingress-nginx ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

Update your Terraform variables with `ingress_external_ip=<IP>` and re-run `terraform apply`, or manually create A records:

```bash
az network dns record-set a add-record \
  --resource-group <RG_NAME> \
  --zone-name <YOURDOMAIN.com> \
  --record-set-name staging \
  --ipv4-address <INGRESS_IP>
```

### Create ClusterIssuers

Apply the following ClusterIssuers for Let's Encrypt:

```yaml
# letsencrypt-staging.yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    email: admin@<YOURDOMAIN.com>
    privateKeySecretRef:
      name: letsencrypt-staging
    solvers:
    - http01:
        ingress:
          class: nginx
---
# letsencrypt-prod.yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@<YOURDOMAIN.com>
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
```

```bash
kubectl apply -f letsencrypt-staging.yaml
kubectl apply -f letsencrypt-prod.yaml
```

## 8. GitHub Secrets Configuration

Configure repository-level secrets in **Settings > Secrets and variables > Actions**:

| Secret Name | Description |
| --- | --- |
| `AZURE_CLIENT_ID` | Application ID of the Service Principal |
| `AZURE_TENANT_ID` | Azure Tenant ID |
| `AZURE_SUBSCRIPTION_ID` | Azure Subscription ID |

Configure environment-level secrets in **Settings > Environments > staging**:

| Secret Name | Description |
| --- | --- |
| `STAGING_POSTGRES_HOST` | Azure PostgreSQL FQDN (from Terraform output) |
| `STAGING_POSTGRES_PASSWORD` | Password for staging DB |
| `STAGING_POSTGRES_DSN` | `postgres://user:pass@host:5432/db?sslmode=require` |
| `STAGING_JWT_SECRET` | Secure random string for staging JWTs |

Configure environment-level secrets in **Settings > Environments > production**:

| Secret Name | Description |
| --- | --- |
| `PRODUCTION_POSTGRES_HOST` | Azure PostgreSQL FQDN (from Terraform output) |
| `PRODUCTION_POSTGRES_PASSWORD` | Password for production DB |
| `PRODUCTION_POSTGRES_DSN` | `postgres://user:pass@host:5432/db?sslmode=require` |
| `PRODUCTION_JWT_SECRET` | Secure random string for production JWTs |

For full CI/CD pipeline details, see [ci-cd.md](ci-cd.md).

## 9. First Deployment

### Staging

Push or merge to `master`. The CI pipeline runs all checks, pushes a Docker image tagged with the commit SHA, and automatically deploys to staging.

```bash
git push origin master
```

### Production

Create and push a release tag. The Release pipeline builds the image with a semver tag and automatically deploys to production.

```bash
git tag v1.0.0
git push origin v1.0.0
```

## 10. Verification

Verify the health of the deployment:

```bash
# Check pod status
kubectl get pods -n bobberchat

# View application logs
kubectl logs -l app.kubernetes.io/name=bobberchat -n bobberchat

# Verify API health
curl -v https://staging.<YOURDOMAIN.com>/v1/health
```

## Expected Costs

| Environment | Estimated Cost (Monthly) | Components |
| --- | --- | --- |
| Staging | $80 - $120 | 1x DS2v2 AKS Node, B1ms PostgreSQL, Bandwidth |
| Production | $300 - $500 | 3x DS2v2 AKS Nodes, GP PostgreSQL, Managed Disks, Bandwidth |

## Future Migration Guide

To migrate BobberChat to other providers (e.g., AWS or Vercel):
- **Terraform**: Swap the `azurerm` provider for `aws` or `vercel` and rewrite infrastructure modules.
- **Helm**: Update `values.yaml` to match the target environment's ingress and database requirements.
- **CI/CD**: Update the GitHub Actions workflow to target EKS or the respective deployment platform.

Ultraworked with [Sisyphus](https://github.com/code-yeongyu/oh-my-opencode)
Co-authored-by: Sisyphus <clio-agent@sisyphuslabs.ai>
