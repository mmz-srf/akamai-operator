# Hostname Management

The Akamai Operator fully manages Property Hostnames using the Akamai Property Manager API. This document explains how hostname management works and how to configure it.

## Overview

The operator manages hostnames for Akamai properties by:
- **Automatically creating edge hostnames** if they don't exist
- Creating and updating hostnames when a property is created or modified
- Detecting changes in hostname configuration and updating the property version
- Supporting SSL certificate provisioning types
- Maintaining hostname state across property versions

## API Reference

The operator uses the [Akamai Property Manager API - PATCH Property Version Hostnames](https://techdocs.akamai.com/property-mgr/reference/patch-property-version-hostnames) endpoint to manage hostnames.

## Hostname Configuration

Hostnames are configured in the `spec.hostnames` field of the AkamaiProperty resource:

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
  
  # Hostname configuration
  hostnames:
    - cnameFrom: "www.my-website.com"
      cnameTo: "my-website.com.edgesuite.net"
      certProvisioningType: "CPS_MANAGED"
    - cnameFrom: "my-website.com"
      cnameTo: "my-website.com.edgesuite.net"
      certProvisioningType: "CPS_MANAGED"
```

### Hostname Fields

- **cnameFrom** (required): The hostname that will be served through Akamai (e.g., `www.example.com`)
- **cnameTo** (required): The edge hostname target (e.g., `example.com.edgesuite.net`)
- **certProvisioningType** (optional): How SSL certificates are provisioned
  - `CPS_MANAGED`: Certificates managed through Akamai Certificate Provisioning System
  - `DEFAULT`: Use default certificate provisioning

## How It Works

### Property Creation

When you create a new AkamaiProperty resource with hostnames:

1. The operator checks if the referenced edge hostnames exist
2. If edge hostnames don't exist and an `edgeHostname` spec is provided, they are created automatically
3. The operator creates the property in Akamai
4. After creation, it sets the initial hostnames for version 1
5. The property is now ready with the configured hostnames

### Property Updates

When you modify the hostnames in an existing AkamaiProperty:

1. The operator detects the change by comparing desired vs current hostnames
2. It ensures any new edge hostnames exist (creating them if necessary)
3. It creates a new property version
4. It updates the hostnames for the new version
5. The new version can then be activated

### Edge Hostname Auto-Creation

The operator can automatically create edge hostnames if they don't exist. When you specify hostnames with a `cnameTo` target that doesn't exist, the operator will:

1. Check if the edge hostname exists in Akamai
2. If not found and an `edgeHostname` spec is provided, create it using that configuration
3. Use the created edge hostname for the property hostnames

This feature simplifies property setup by eliminating the need to pre-create edge hostnames manually.

### Hostname Comparison

The operator compares hostnames by:
- Checking if the count of hostnames matches
- Verifying each `cnameFrom` exists in both sets
- Comparing `cnameTo` values
- Comparing `certProvisioningType` if specified

## Examples

### Single Hostname

```yaml
apiVersion: akamai.com/v1alpha1
kind: AkamaiProperty
metadata:
  name: simple-site
spec:
  propertyName: "simple-site.com"
  contractId: "ctr_C-1234567"
  groupId: "grp_12345"
  productId: "prd_Fresca"
  
  hostnames:
    - cnameFrom: "www.simple-site.com"
      cnameTo: "simple-site.com.edgesuite.net"
      certProvisioningType: "CPS_MANAGED"
```

### With Automatic Edge Hostname Creation

```yaml
apiVersion: akamai.com/v1alpha1
kind: AkamaiProperty
metadata:
  name: auto-edge-hostname
spec:
  propertyName: "my-website.com"
  contractId: "ctr_C-1234567"
  groupId: "grp_12345"
  productId: "prd_Fresca"
  
  # Edge hostname configuration for auto-creation
  edgeHostname:
    domainPrefix: "my-website.com"
    domainSuffix: "edgesuite.net"
    secureNetwork: "ENHANCED_TLS"
    ipVersionBehavior: "IPV4"
  
  hostnames:
    - cnameFrom: "www.my-website.com"
      cnameTo: "my-website.com.edgesuite.net"  # Will be created if it doesn't exist
      certProvisioningType: "CPS_MANAGED"
```

**Note:** If the edge hostname `my-website.com.edgesuite.net` doesn't exist, the operator will create it using the `edgeHostname` specification.

### Multiple Hostnames

```yaml
apiVersion: akamai.com/v1alpha1
kind: AkamaiProperty
metadata:
  name: multi-domain-site
spec:
  propertyName: "multi-domain-site.com"
  contractId: "ctr_C-1234567"
  groupId: "grp_12345"
  productId: "prd_Fresca"
  
  hostnames:
    # Main website
    - cnameFrom: "www.example.com"
      cnameTo: "example.com.edgesuite.net"
      certProvisioningType: "CPS_MANAGED"
    
    # Apex domain
    - cnameFrom: "example.com"
      cnameTo: "example.com.edgesuite.net"
      certProvisioningType: "CPS_MANAGED"
    
    # API subdomain
    - cnameFrom: "api.example.com"
      cnameTo: "example.com.edgekey.net"
      certProvisioningType: "CPS_MANAGED"
    
    # Static assets subdomain
    - cnameFrom: "static.example.com"
      cnameTo: "example.com.akamaized.net"
      certProvisioningType: "CPS_MANAGED"
```

### Different Edge Hostnames

You can use different edge hostname types based on your needs:

```yaml
hostnames:
  # Standard Enhanced TLS edge hostname
  - cnameFrom: "www.example.com"
    cnameTo: "example.com.edgesuite.net"
    certProvisioningType: "CPS_MANAGED"
  
  # Secure edge hostname with additional features
  - cnameFrom: "secure.example.com"
    cnameTo: "example.com.edgekey.net"
    certProvisioningType: "CPS_MANAGED"
  
  # Media delivery edge hostname
  - cnameFrom: "media.example.com"
    cnameTo: "example.com.akamaized.net"
    certProvisioningType: "CPS_MANAGED"
```

## Updating Hostnames

To update hostnames, simply modify the `spec.hostnames` field:

```bash
kubectl edit akamaiproperty my-property
```

Or update your YAML file and apply:

```bash
kubectl apply -f my-property.yaml
```

The operator will:
1. Detect the change
2. Create a new property version
3. Update the hostnames
4. Update the status to reflect the new version

## Hostname Lifecycle

### Adding Hostnames

Add new hostnames to the list:

```yaml
hostnames:
  - cnameFrom: "www.example.com"
    cnameTo: "example.com.edgesuite.net"
    certProvisioningType: "CPS_MANAGED"
  - cnameFrom: "new.example.com"  # New hostname
    cnameTo: "example.com.edgesuite.net"
    certProvisioningType: "CPS_MANAGED"
```

### Removing Hostnames

Remove hostnames from the list:

```yaml
hostnames:
  - cnameFrom: "www.example.com"
    cnameTo: "example.com.edgesuite.net"
    certProvisioningType: "CPS_MANAGED"
  # old.example.com removed
```

### Modifying Hostnames

Change the edge hostname or certificate provisioning:

```yaml
hostnames:
  - cnameFrom: "www.example.com"
    cnameTo: "example.com.edgekey.net"  # Changed from edgesuite.net
    certProvisioningType: "CPS_MANAGED"
```

## Best Practices

1. **Plan Your Hostnames**: Think about all the hostnames you'll need before creating the property
2. **Use CPS_MANAGED**: For SSL certificates, use CPS_MANAGED for automatic certificate management
3. **Consistent Edge Hostnames**: Group related hostnames under the same edge hostname when possible
4. **Test in Staging**: Always test hostname changes in staging before production
5. **DNS Configuration**: Remember to create DNS CNAME records pointing to the edge hostnames

## DNS Configuration

After configuring hostnames in the operator, you need to create DNS records:

```
www.example.com.     CNAME   example.com.edgesuite.net.
api.example.com.     CNAME   example.com.edgekey.net.
static.example.com.  CNAME   example.com.akamaized.net.
```

## Troubleshooting

### Hostname Update Failed

If hostname updates fail, check:
- The edge hostname exists and is properly configured
- You have permissions to modify the property
- The hostname isn't already in use by another property
- Certificate provisioning is set up correctly

### Hostnames Not Applied

If hostnames aren't being applied:
- Check the operator logs: `kubectl logs -l app=akamai-operator`
- Verify the property status: `kubectl describe akamaiproperty <name>`
- Ensure the property version was created successfully

### Certificate Provisioning Issues

If you have certificate issues:
- Verify `certProvisioningType` is set correctly
- Check if certificates are enrolled in CPS
- Ensure the certificate covers all configured hostnames

## Monitoring

Monitor hostname management through:

1. **Property Status**:
   ```bash
   kubectl get akamaiproperty my-property -o yaml
   ```

2. **Operator Logs**:
   ```bash
   kubectl logs -l app=akamai-operator -f
   ```

3. **Akamai Control Center**:
   - Check property hostnames in the Property Manager
   - Verify edge hostnames are configured correctly
   - Review activation status

## Integration with Activation

When you activate a property with hostnames:

```yaml
spec:
  hostnames:
    - cnameFrom: "www.example.com"
      cnameTo: "example.com.edgesuite.net"
      certProvisioningType: "CPS_MANAGED"
  
  activation:
    network: "STAGING"
    notifyEmails:
      - "ops@example.com"
    note: "Activating with new hostnames"
```

The operator will:
1. Update hostnames if needed
2. Create a new version
3. Activate that version to the specified network
4. Track activation status

## API Client Methods

The operator provides these methods for hostname management:

- `GetPropertyHostnames()`: Retrieve current hostnames for a property version
- `UpdatePropertyHostnames()`: Update hostnames using PATCH (additive)
- `SetPropertyHostnames()`: Replace all hostnames
- `CompareHostnames()`: Compare desired vs current hostname configuration

## See Also

- [Akamai Property Manager API Documentation](https://techdocs.akamai.com/property-mgr/reference/api)
- [Activation Documentation](ACTIVATION.md)
- [Rules Management](RULESET_MANAGEMENT.md)
