#!/bin/bash

# Akamai Operator Installation Script
# This script ensures CRDs are installed and established before the operator starts

set -e

NAMESPACE="akamai-operator-system"
RELEASE_VERSION="v0.0.1"
MANIFEST_URL="https://github.com/mmz-srf/akamai-operator/releases/download/${RELEASE_VERSION}/akamai-operator-${RELEASE_VERSION}.yaml"

echo "Installing Akamai Operator ${RELEASE_VERSION}"
echo "============================================"

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
if ! command_exists kubectl; then
    echo "Error: kubectl is not installed or not in PATH"
    exit 1
fi

if ! kubectl cluster-info >/dev/null 2>&1; then
    echo "Error: Cannot connect to Kubernetes cluster"
    exit 1
fi

echo "✓ kubectl is available and cluster is accessible"

# Step 1: Create namespace
echo ""
echo "Step 1: Creating namespace..."
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -
echo "✓ Namespace ${NAMESPACE} created/updated"

# Step 2: Download manifest
echo ""
echo "Step 2: Downloading manifest..."
TEMP_DIR=$(mktemp -d)
MANIFEST_FILE="${TEMP_DIR}/akamai-operator.yaml"
if command_exists curl; then
    curl -fsSL "${MANIFEST_URL}" -o "${MANIFEST_FILE}"
elif command_exists wget; then
    wget -q "${MANIFEST_URL}" -O "${MANIFEST_FILE}"
else
    echo "Error: Neither curl nor wget is available"
    exit 1
fi
echo "✓ Manifest downloaded to ${MANIFEST_FILE}"

# Step 3: Extract and apply CRDs first
echo ""
echo "Step 3: Installing CRDs..."
CRD_FILE="${TEMP_DIR}/crds.yaml"
awk '/^apiVersion: apiextensions.k8s.io\/v1$/{p=1} p; /^---$/ && p{p=0; if(getline && /^apiVersion/ && !/apiextensions.k8s.io/){print; p=0}}' "${MANIFEST_FILE}" > "${CRD_FILE}"

if [ -s "${CRD_FILE}" ]; then
    kubectl apply -f "${CRD_FILE}"
    echo "✓ CRDs applied"
    
    # Wait for CRDs to be established
    echo "Waiting for CRDs to be established..."
    kubectl wait --for condition=established --timeout=60s crd/akamaiproperties.akamai.com
    echo "✓ CRDs are established"
else
    echo "Warning: No CRDs found in manifest"
fi

# Step 4: Check for Akamai credentials
echo ""
echo "Step 4: Checking Akamai credentials..."
if kubectl get secret akamai-credentials -n ${NAMESPACE} >/dev/null 2>&1; then
    echo "✓ Akamai credentials secret found"
else
    echo "⚠ Akamai credentials secret not found!"
    echo ""
    echo "Please create the secret with your Akamai EdgeGrid API credentials:"
    echo ""
    echo "kubectl create secret generic akamai-credentials \\"
    echo "  --from-literal=host=\"your-host.akamaiapis.net\" \\"
    echo "  --from-literal=client_token=\"your-client-token\" \\"
    echo "  --from-literal=client_secret=\"your-client-secret\" \\"
    echo "  --from-literal=access_token=\"your-access-token\" \\"
    echo "  --namespace=${NAMESPACE}"
    echo ""
    read -p "Do you want to continue without the secret? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Installation cancelled. Please create the secret and run this script again."
        exit 1
    fi
fi

# Step 5: Apply the complete manifest
echo ""
echo "Step 5: Installing operator..."
kubectl apply -f "${MANIFEST_FILE}"
echo "✓ Operator manifests applied"

# Step 6: Wait for operator to be ready
echo ""
echo "Step 6: Waiting for operator to be ready..."
kubectl rollout status deployment/akamai-operator-controller-manager -n ${NAMESPACE} --timeout=300s
echo "✓ Operator is ready"

# Step 7: Verify installation
echo ""
echo "Step 7: Verifying installation..."

echo -n "Checking CRD registration... "
if kubectl api-resources | grep -q akamaiproperties; then
    echo "✓"
else
    echo "✗"
    echo "Warning: AkamaiProperty CRD not found in API resources"
fi

echo -n "Checking operator pod... "
READY_PODS=$(kubectl get pods -n ${NAMESPACE} -l control-plane=controller-manager --field-selector=status.phase=Running -o name | wc -l)
if [ "${READY_PODS}" -gt 0 ]; then
    echo "✓"
else
    echo "✗"
    echo "Warning: No running operator pods found"
fi

# Cleanup
rm -rf "${TEMP_DIR}"

echo ""
echo "Installation complete!"
echo "====================="
echo ""
echo "Next steps:"
echo "1. Verify the installation:"
echo "   kubectl get pods -n ${NAMESPACE}"
echo "   kubectl logs -n ${NAMESPACE} deployment/akamai-operator-controller-manager"
echo ""
echo "2. Create an AkamaiProperty resource:"
echo "   kubectl apply -f config/samples/"
echo ""
echo "3. Check the operator logs for any issues:"
echo "   kubectl logs -n ${NAMESPACE} deployment/akamai-operator-controller-manager -f"