# Edge Hostname Creation

The Akamai Operator can automatically create edge hostnames when they don't exist. This document explains how edge hostname creation works and how to configure it.

## Overview

Edge hostnames are the Akamai-provided hostnames that your property hostnames point to via CNAME records. The operator can automatically:

- Check if edge hostnames exist before creating or updating properties
- Create missing edge hostnames based on your specification
- Reuse existing edge hostnames when they match your configuration

## Edge Hostname Specification

Define edge hostname configuration in the `spec.edgeHostname` field:

```yaml
apiVersion: akamai.com/v1alpha1
kind: AkamaiProperty
metadata:
  name: my-property
spec:
  propertyName: "my-website.com"
  contractId: "ctr_C-1234567"
  groupId: "grp_12345"
  productId: "prd_Fresca"
  
  # Edge hostname configuration
  edgeHostname:
    domainPrefix: "my-website.com"
    domainSuffix: "edgesuite.net"
    secureNetwork: "ENHANCED_TLS"
    ipVersionBehavior: "IPV4"
  
  hostnames:
    - cnameFrom: "www.my-website.com"
      cnameTo: "my-website.com.edgesuite.net"  # Will be created if needed
      certProvisioningType: "CPS_MANAGED"
```

### Edge Hostname Fields

- **domainPrefix** (required): The prefix for the edge hostname (e.g., `my-website.com`)
- **domainSuffix** (required): The suffix for the edge hostname (e.g., `edgesuite.net`, `edgekey.net`, `akamaized.net`)
- **secureNetwork** (optional): The secure network type
  - `ENHANCED_TLS`: Enhanced TLS security
  - `STANDARD_TLS`: Standard TLS security
- **ipVersionBehavior** (optional): IP version behavior
  - `IPV4`: IPv4 only (default)
  - `IPV6_COMPLIANCE`: IPv6 compliance mode
  - `IPV6_PERFORMANCE`: IPv6 performance mode

## How It Works

### Automatic Creation Flow

1. **Property Creation/Update**: When you create or update a property with hostnames
2. **Edge Hostname Check**: The operator checks if each referenced edge hostname exists
3. **Auto-Creation**: If an edge hostname doesn't exist:
   - The operator extracts the prefix and suffix from the `cnameTo` value
   - It uses the `edgeHostname` spec to create the edge hostname
   - The created edge hostname is then available for the property
4. **Property Configuration**: The property is configured with the hostnames

### Edge Hostname Reuse

If an edge hostname already exists in your contract/group, the operator will:
- Detect the existing edge hostname
- Reuse it for your property
- Skip creation to avoid conflicts

## Domain Suffix Types

### Standard Edge Hostname (edgesuite.net)

Best for: General web delivery

```yaml
edgeHostname:
  domainPrefix: "example.com"
  domainSuffix: "edgesuite.net"
  ipVersionBehavior: "IPV4"
```

### Secure Edge Hostname (edgekey.net)

Best for: Secure content with enhanced TLS

```yaml
edgeHostname:
  domainPrefix: "example.com"
  domainSuffix: "edgekey.net"
  secureNetwork: "ENHANCED_TLS"
  ipVersionBehavior: "IPV4"
```

### Media Delivery (akamaized.net)

Best for: Media and large file delivery

```yaml
edgeHostname:
  domainPrefix: "example.com"
  domainSuffix: "akamaized.net"
  secureNetwork: "ENHANCED_TLS"
  ipVersionBehavior: "IPV6_COMPLIANCE"
```

## Complete Example

```yaml
apiVersion: akamai.com/v1alpha1
kind: AkamaiProperty
metadata:
  name: complete-auto-edge-hostname
spec:
  propertyName: "my-app.com"
  contractId: "ctr_C-1234567"
  groupId: "grp_12345"
  productId: "prd_Fresca"
  
  # Edge hostname spec for auto-creation
  edgeHostname:
    domainPrefix: "my-app.com"
    domainSuffix: "edgesuite.net"
    secureNetwork: "ENHANCED_TLS"
    ipVersionBehavior: "IPV4"
  
  # Multiple hostnames using the edge hostname
  hostnames:
    - cnameFrom: "www.my-app.com"
      cnameTo: "my-app.com.edgesuite.net"
      certProvisioningType: "CPS_MANAGED"
    - cnameFrom: "my-app.com"
      cnameTo: "my-app.com.edgesuite.net"
      certProvisioningType: "CPS_MANAGED"
    - cnameFrom: "api.my-app.com"
      cnameTo: "my-app.com.edgesuite.net"
      certProvisioningType: "CPS_MANAGED"
  
  rules:
    name: "default"
    behaviors:
      - name: "origin"
        options:
          originType: "CUSTOMER"
          hostname: "origin.my-app.com"
  
  activation:
    network: "STAGING"
    notifyEmails:
      - "ops@my-app.com"
```

## Multiple Edge Hostnames

You can use multiple edge hostnames for different purposes:

```yaml
spec:
  propertyName: "multi-edge.com"
  contractId: "ctr_C-1234567"
  groupId: "grp_12345"
  productId: "prd_Fresca"
  
  # Default edge hostname config (used as fallback)
  edgeHostname:
    domainPrefix: "multi-edge.com"
    domainSuffix: "edgesuite.net"
    secureNetwork: "ENHANCED_TLS"
    ipVersionBehavior: "IPV4"
  
  hostnames:
    # Web traffic - edgesuite
    - cnameFrom: "www.multi-edge.com"
      cnameTo: "multi-edge.com.edgesuite.net"
      certProvisioningType: "CPS_MANAGED"
    
    # API traffic - edgekey for enhanced security
    - cnameFrom: "api.multi-edge.com"
      cnameTo: "api.multi-edge.com.edgekey.net"
      certProvisioningType: "CPS_MANAGED"
    
    # Media traffic - akamaized for large files
    - cnameFrom: "media.multi-edge.com"
      cnameTo: "media.multi-edge.com.akamaized.net"
      certProvisioningType: "CPS_MANAGED"
```

**Note:** When using multiple different edge hostname suffixes, ensure you create each edge hostname manually or provide separate edge hostname specs, as the operator uses the single `edgeHostname` spec as a template.

## Best Practices

1. **Use Consistent Prefixes**: Use your domain name as the prefix for easy identification
2. **Choose Appropriate Suffixes**: Select the suffix based on your use case (web, API, media)
3. **Enable Secure Networks**: Always specify `secureNetwork` for production properties
4. **Plan IP Strategy**: Choose appropriate `ipVersionBehavior` based on your audience
5. **Test in Staging**: Test edge hostname creation in staging before production

## Troubleshooting

### Edge Hostname Creation Failed

**Symptoms**: Property creation fails with edge hostname error

**Possible Causes:**
- Invalid domain prefix or suffix
- Insufficient permissions to create edge hostnames
- Edge hostname already exists in a different contract/group

**Solutions:**
- Verify the edge hostname spec is correct
- Check API credentials have edge hostname creation permissions
- Check if the edge hostname exists elsewhere and remove/transfer it

### Edge Hostname Not Found

**Symptoms**: Property creation succeeds but hostname configuration fails

**Possible Causes:**
- Edge hostname creation is pending
- Edge hostname was created but not yet propagated

**Solutions:**
- Wait a few minutes for edge hostname to propagate
- Check edge hostname status in Akamai Control Center
- Requeue the reconciliation

### Multiple Edge Hostnames Not Created

**Symptoms**: Only one edge hostname is created when multiple are referenced

**Possible Causes:**
- The operator uses the single `edgeHostname` spec as a template
- Different suffixes require different configurations

**Solutions:**
- Create additional edge hostnames manually in Akamai
- Or ensure all hostnames use the same edge hostname suffix

## Monitoring

Check edge hostname creation status:

```bash
# Check operator logs
kubectl logs -n akamai-operator-system deployment/akamai-operator-controller-manager | grep -i "edge hostname"

# Check property status
kubectl describe akamaiproperty my-property | grep -i "edge"
```

## Limitations

1. **Single Edge Hostname Spec**: Only one `edgeHostname` spec per property
2. **Template-Based Creation**: All auto-created edge hostnames use the same configuration
3. **No Edge Hostname Updates**: Once created, edge hostnames cannot be updated (Akamai limitation)
4. **Contract/Group Bound**: Edge hostnames are specific to a contract and group

## API Client Methods

The operator provides these methods for edge hostname management:

- `CreateEdgeHostname()`: Create a new edge hostname
- `GetEdgeHostname()`: Retrieve an edge hostname by ID
- `ListEdgeHostnames()`: List all edge hostnames for a contract/group
- `FindEdgeHostnameByName()`: Search for an edge hostname by name
- `GetOrCreateEdgeHostname()`: Get existing or create new edge hostname
- `EnsureEdgeHostnamesExist()`: Ensure all referenced edge hostnames exist

## See Also

- [Hostname Management](HOSTNAME_MANAGEMENT.md)
- [Akamai Edge Hostname API Documentation](https://techdocs.akamai.com/property-mgr/reference/post-edgehostnames)
- [Testing Documentation](TESTING.md)
