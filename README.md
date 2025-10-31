# Akamai Operator

A Kubernetes operator for managing Akamai Properties through the Property Manager API. This operator is OLM (Operator Lifecycle Manager) compatible and allows you to manage Akamai edge delivery configurations as Kubernetes custom resources.

## Features

- **Declarative Property Management**: Define Akamai properties as Kubernetes custom resources
- **Full Lifecycle Management**: Create, update, and delete Akamai properties through Kubernetes
- **OLM Compatible**: Can be installed and managed through Operator Lifecycle Manager
- **EdgeGrid Authentication**: Secure authentication using Akamai EdgeGrid
- **Status Reporting**: Real-time status updates with property versions and deployment state
- **Rule Configuration**: Support for complex property rules, behaviors, and criteria

## Prerequisites

- Kubernetes cluster (v1.19+)
- Akamai API credentials with Property Manager permissions
- `kubectl` configured to access your cluster

## Quick Start

### 1. Install the Operator

#### Option A: Using OLM (Recommended for production)

```bash
# Install OLM if not already installed
curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.25.0/install.sh | bash -s v0.25.0

# Install the Akamai Operator
kubectl apply -f https://github.com/mmz-srf/akamai-operator/releases/latest/download/akamai-operator.yaml
```

#### Option B: Direct Installation

```bash
# Clone the repository
git clone https://github.com/mmz-srf/akamai-operator.git
cd akamai-operator

# Install CRDs
make install

# Deploy the operator
make deploy
```

### 2. Configure Akamai Credentials

Create a secret with your Akamai API credentials:

```bash
kubectl create secret generic akamai-credentials \
  --from-literal=host="akaa-baseurl-xxxxxxxxxxx-xxxxxxxxxxxxx.luna.akamaiapis.net" \
  --from-literal=client_token="akab-xxxxxxxxxxxxxxxx-xxxxxxxxxxxxxxxx" \
  --from-literal=client_secret="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" \
  --from-literal=access_token="akab-xxxxxxxxxxxxxxxx-xxxxxxxxxxxxxxxx" \
  --namespace=akamai-operator-system
```

**Note:** The operator watches for cluster-scoped `AkamaiProperty` resources across all namespaces, but the operator itself and its credentials are deployed in the `akamai-operator-system` namespace.

### 3. Create an Akamai Property

Create a sample property configuration:

```yaml
apiVersion: akamai.com/v1alpha1
kind: AkamaiProperty
metadata:
  name: my-website
spec:
  propertyName: "my-website.com"
  contractId: "ctr_C-1234567"
  groupId: "grp_12345"
  productId: "prd_Fresca"
  hostnames:
    - cnameFrom: "my-website.com"
      cnameTo: "my-website.com.edgesuite.net"
      certProvisioningType: "CPS_MANAGED"
  rules:
    name: "default"
    behaviors:
      - name: "origin"
        options:
          originType: "CUSTOMER"
          hostname: "origin.my-website.com"
          forwardHostHeader: "REQUEST_HOST_HEADER"
```

Apply the configuration:

```bash
kubectl apply -f my-property.yaml
```

### 4. Monitor the Property

Check the status of your property:

```bash
# List all Akamai properties
kubectl get akamaiproperties

# Get detailed status
kubectl describe akamaiproperty my-website
```

## Configuration

### AkamaiProperty Custom Resource

The `AkamaiProperty` custom resource supports the following specifications:

#### Required Fields

- `propertyName`: Name of the Akamai property
- `contractId`: Akamai contract ID (format: `ctr_C-XXXXXXX`)
- `groupId`: Akamai group ID (format: `grp_XXXXX`)
- `productId`: Akamai product ID (e.g., `prd_Fresca`)

#### Optional Fields

- `hostnames`: Array of hostname configurations
- `rules`: Property rules configuration with behaviors and criteria
- `edgeHostname`: Edge hostname configuration

### Hostnames Configuration

```yaml
hostnames:
  - cnameFrom: "example.com"
    cnameTo: "example.com.edgesuite.net"
    certProvisioningType: "CPS_MANAGED"
```

### Rules Configuration

The rules system supports nested rules with criteria and behaviors:

```yaml
rules:
  name: "default"
  behaviors:
    - name: "origin"
      options:
        originType: "CUSTOMER"
        hostname: "origin.example.com"
  children:
    - name: "Static Assets"
      criteria:
        - name: "fileExtension"
          options:
            matchOperator: "IS_ONE_OF"
            values: ["css", "js", "png"]
      behaviors:
        - name: "caching"
          options:
            behavior: "MAX_AGE"
            ttl: "7d"
```

## Authentication

The operator uses Akamai EdgeGrid authentication. You need to provide the following credentials:

1. **Host**: Your API endpoint hostname
2. **Client Token**: Your client token
3. **Client Secret**: Your client secret  
4. **Access Token**: Your access token

These can be obtained from the Akamai Control Center under "Identity & Access Management" > "API User".

## Examples

### Basic Website Property

```yaml
apiVersion: akamai.com/v1alpha1
kind: AkamaiProperty
metadata:
  name: basic-website
spec:
  propertyName: "basic-website.com"
  contractId: "ctr_C-1234567"
  groupId: "grp_12345"
  productId: "prd_Fresca"
  hostnames:
    - cnameFrom: "basic-website.com"
      cnameTo: "basic-website.com.edgesuite.net"
  rules:
    name: "default"
    behaviors:
      - name: "origin"
        options:
          originType: "CUSTOMER"
          hostname: "origin.basic-website.com"
```

### Advanced Property with Multiple Rules

See [config/samples/akamai_v1alpha1_akamaiproperty.yaml](config/samples/akamai_v1alpha1_akamaiproperty.yaml) for a comprehensive example.

## Monitoring and Troubleshooting

### Check Operator Logs

```bash
kubectl logs -n akamai-operator-system deployment/akamai-operator-controller-manager
```

### Property Status

The operator reports detailed status information:

```yaml
status:
  propertyId: "prp_123456"
  latestVersion: 2
  stagingVersion: 1
  productionVersion: 1
  phase: "Ready"
  conditions:
    - type: "Ready"
      status: "True"
      reason: "PropertyReady"
      message: "Property is ready"
```

### Common Issues

1. **Authentication Errors**: Verify your API credentials are correct
2. **Contract/Group Not Found**: Ensure the contract and group IDs are valid
3. **Property Creation Failed**: Check the operator logs for detailed error messages

## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/mmz-srf/akamai-operator.git
cd akamai-operator

# Build the operator
make build

# Run tests
make test

# Build Docker image
make docker-build IMG=your-registry/akamai-operator:latest
```

### Local Development

```bash
# Install CRDs
make install

# Run the operator locally
make run
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE) for details.

## Support

For support questions, please:

1. Check the [troubleshooting guide](#monitoring-and-troubleshooting)
2. Review [existing issues](https://github.com/mmz-srf/akamai-operator/issues)
3. Create a new issue with detailed information

For Akamai API questions, consult the [Property Manager API documentation](https://techdocs.akamai.com/property-mgr/reference/post-properties).

## Roadmap

- [ ] Support for property activation/deactivation
- [ ] Bulk property operations
- [ ] Advanced rule validation
- [ ] Integration with GitOps workflows
- [ ] Property backup and restore functionality
- [ ] Webhook support for property changes