# Akamai Operator Credentials Setup

This document explains how to configure Akamai EdgeGrid API credentials for the Akamai Operator.

## Prerequisites

1. Akamai EdgeGrid API credentials with Property Manager API access
2. Kubernetes cluster with the operator deployed
3. `kubectl` configured to access your cluster

## Credential Configuration

### Option 1: Using Environment Variables + Makefile (Recommended)

1. **Set your Akamai credentials as environment variables:**

```bash
export AKAMAI_HOST="your-host.akamaiapis.net"
export AKAMAI_CLIENT_TOKEN="your-client-token"
export AKAMAI_CLIENT_SECRET="your-client-secret"
export AKAMAI_ACCESS_TOKEN="your-access-token"
```

2. **Create the secret using the Makefile:**

```bash
make create-secret
```

### Option 2: Manual Secret Creation

1. **Create the secret directly with kubectl:**

```bash
kubectl create secret generic akamai-credentials \
  --from-literal=host="your-host.akamaiapis.net" \
  --from-literal=client_token="your-client-token" \
  --from-literal=client_secret="your-client-secret" \
  --from-literal=access_token="your-access-token" \
  --namespace=akamai-operator-system
```

### Option 3: Using YAML File

1. **Create a secret YAML file (DO NOT commit this to version control):**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: akamai-credentials
  namespace: akamai-operator-system
type: Opaque
stringData:
  host: your-host.akamaiapis.net
  client_token: your-client-token
  client_secret: your-client-secret
  access_token: your-access-token
```

2. **Apply the secret:**

```bash
kubectl apply -f akamai-credentials.yaml
```

## Getting Akamai EdgeGrid Credentials

1. **Log in to Akamai Control Center**
2. **Navigate to:** ☰ → ACCOUNT ADMIN → Identity & access → API users
3. **Create a new API client** or use an existing one
4. **Ensure the client has access to:**
   - Property Manager API
   - Required authorization groups

## Verification

Verify the secret was created correctly:

```bash
kubectl get secret akamai-credentials -n akamai-operator-system
kubectl describe secret akamai-credentials -n akamai-operator-system
```

## Security Best Practices

- ⚠️ **Never commit credentials to version control**
- 🔐 Use Kubernetes RBAC to restrict access to the secret
- 🔄 Rotate credentials regularly
- 📝 Use separate credentials for different environments (dev/staging/prod)

## Troubleshooting

### Secret Not Found
```bash
# Check if the namespace exists
kubectl get namespace akamai-operator-system

# Check if the secret exists
kubectl get secrets -n akamai-operator-system
```

### Permission Denied
- Verify your Akamai API credentials have Property Manager API access
- Check the authorization groups in Akamai Control Center
- Ensure the credentials are not expired

### Operator Not Starting
```bash
# Check operator logs
kubectl logs -n akamai-operator-system deployment/akamai-operator-controller-manager

# Check if secret is properly mounted
kubectl describe pod -n akamai-operator-system -l control-plane=controller-manager
```