# Installation Guide for Akamai Operator

## Prerequisites

1. Kubernetes cluster (version 1.19+)
2. kubectl configured to access your cluster
3. Akamai EdgeGrid API credentials

## Step 1: Install CRDs First

Before installing the operator, you need to install the Custom Resource Definitions:

```bash
# Apply only the CRDs first
kubectl apply -f https://github.com/mmz-srf/akamai-operator/releases/download/v0.0.1/akamai-operator-v0.0.1.yaml --dry-run=client -o yaml | kubectl apply -f - --validate=false --wait=true
```

Or manually extract and apply CRDs:

```bash
# Download the release manifest
curl -LO https://github.com/mmz-srf/akamai-operator/releases/download/v0.0.1/akamai-operator-v0.0.1.yaml

# Extract and apply CRDs first
grep -A 1000 "kind: CustomResourceDefinition" akamai-operator-v0.0.1.yaml | grep -B 1000 "^---$" | head -n -1 > crds.yaml
kubectl apply -f crds.yaml
kubectl wait --for condition=established --timeout=60s crd/akamaiproperties.akamai.com
```

## Step 2: Create Akamai Credentials Secret

```bash
kubectl create secret generic akamai-credentials \
  --from-literal=host="your-host.akamaiapis.net" \
  --from-literal=client_token="your-client-token" \
  --from-literal=client_secret="your-client-secret" \
  --from-literal=access_token="your-access-token" \
  --namespace=akamai-operator-system
```

## Step 3: Install the Operator

```bash
# Apply the complete manifest
kubectl apply -f https://github.com/mmz-srf/akamai-operator/releases/download/v0.0.1/akamai-operator-v0.0.1.yaml
```

## Step 4: Verify Installation

```bash
# Check if CRDs are installed
kubectl get crd akamaiproperties.akamai.com

# Check if the operator is running
kubectl get pods -n akamai-operator-system

# Check operator logs
kubectl logs -n akamai-operator-system deployment/akamai-operator-controller-manager -c manager
```

## Troubleshooting

### "unable to retrieve the complete list of server APIs" Error

This error occurs when CRDs are not properly installed or not yet established. Solutions:

1. **Ensure CRDs are established:**
   ```bash
   kubectl wait --for condition=established --timeout=60s crd/akamaiproperties.akamai.com
   ```

2. **Restart the operator:**
   ```bash
   kubectl rollout restart deployment/akamai-operator-controller-manager -n akamai-operator-system
   ```

3. **Check API resources:**
   ```bash
   kubectl api-resources | grep akamai
   ```

### Operator Not Starting

1. **Check secret exists:**
   ```bash
   kubectl get secret akamai-credentials -n akamai-operator-system
   ```

2. **Check RBAC permissions:**
   ```bash
   kubectl auth can-i create akamaiproperties.akamai.com --as=system:serviceaccount:akamai-operator-system:akamai-operator-controller-manager
   ```

### Image Pull Issues

If you see image pull errors, the operator images are available at:
- Operator: `ghcr.io/mmz-srf/akamai-operator:v0.0.1`
- Bundle: `ghcr.io/mmz-srf/akamai-operator-bundle:v0.0.1`