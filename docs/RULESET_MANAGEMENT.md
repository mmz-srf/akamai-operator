# Akamai Property Ruleset Management

This document explains how to use the enhanced ruleset management features in the Akamai Operator.

## Overview

The Akamai Operator now supports comprehensive property rule tree management through the `rules` field in the `AkamaiProperty` custom resource. This allows you to define and manage your Akamai property rules directly in Kubernetes.

## Rule Structure

The rule structure follows the Akamai Property Manager API format with the following key components:

### Top-Level Rule
- **name**: Must be "default" for the top-level rule
- **comment**: Optional descriptive comment
- **variables**: Global variables that can be used throughout the rule tree
- **options**: Top-level rule options (e.g., `is_secure`)
- **behaviors**: Default behaviors applied to all requests
- **criteria**: Match conditions (typically empty for top-level rule)
- **children**: Nested rules for specific scenarios

### Variables
Variables allow you to define reusable values that can be referenced throughout your rule tree:

```yaml
variables:
  - name: "PMUSER_ORIGIN_HOST"
    value: "origin.example.com"
    description: "Primary origin hostname"
    hidden: false
    sensitive: false
```

### Behaviors
Behaviors define actions that Akamai should take when processing requests:

```yaml
behaviors:
  - name: "origin"
    comment: "Configure origin server"
    options:
      originType: "CUSTOMER"
      hostname: "{{user.PMUSER_ORIGIN_HOST}}"
      forwardHostHeader: "REQUEST_HOST_HEADER"
      compress: true
```

### Criteria
Criteria define conditions that must be met for rules to apply:

```yaml
criteria:
  - name: "fileExtension"
    options:
      matchOperator: "IS_ONE_OF"
      matchCaseSensitive: false
      values:
        - "css"
        - "js"
        - "png"
```

### Child Rules
Child rules allow you to create conditional logic for specific content types or scenarios:

```yaml
children:
  - name: "Static Assets"
    comment: "Cache static files longer"
    criteria:
      - name: "fileExtension"
        options:
          matchOperator: "IS_ONE_OF"
          values: ["css", "js", "png"]
    behaviors:
      - name: "caching"
        options:
          behavior: "MAX_AGE"
          ttl: "30d"
```

## Common Use Cases

### 1. Basic Origin Configuration
```yaml
behaviors:
  - name: "origin"
    options:
      originType: "CUSTOMER"
      hostname: "backend.example.com"
      forwardHostHeader: "REQUEST_HOST_HEADER"
      compress: true
```

### 2. Caching Policies
```yaml
# Default caching
behaviors:
  - name: "caching"
    options:
      behavior: "MAX_AGE"
      ttl: "6h"

# Long-term static asset caching
children:
  - name: "Static Assets"
    criteria:
      - name: "fileExtension"
        options:
          matchOperator: "IS_ONE_OF"
          values: ["css", "js", "png", "jpg"]
    behaviors:
      - name: "caching"
        options:
          behavior: "MAX_AGE"
          ttl: "30d"
```

### 3. API Endpoint Handling
```yaml
children:
  - name: "API Endpoints"
    criteria:
      - name: "path"
        options:
          matchOperator: "MATCHES_ONE_OF"
          values: ["/api/*", "/v1/*"]
    behaviors:
      - name: "caching"
        options:
          behavior: "NO_STORE"
      - name: "corsSupport"
        options:
          enabled: true
```

### 4. Security Headers
```yaml
children:
  - name: "Security Headers"
    criteria: []  # Applies to all requests
    behaviors:
      - name: "modifyOutgoingResponseHeader"
        options:
          action: "ADD"
          standardAddHeaderName: "X-Content-Type-Options"
          headerValue: "nosniff"
```

## Rule Validation

The operator performs automatic validation of rule configurations:

- **Required fields**: Ensures all required fields are present
- **Variable names**: Validates variable naming conventions (uppercase, no spaces)
- **Rule structure**: Validates the overall rule tree structure
- **Behavior options**: Basic validation for common behaviors

Validation errors will be reported in the AkamaiProperty status and events.

## Best Practices

### 1. Use Variables for Reusable Values
```yaml
variables:
  - name: "PMUSER_ORIGIN_HOST"
    value: "origin.example.com"
    description: "Primary origin hostname"

behaviors:
  - name: "origin"
    options:
      hostname: "{{user.PMUSER_ORIGIN_HOST}}"
```

### 2. Structure Rules Hierarchically
Organize your rules from most general to most specific:
- Top-level: Default behaviors for all content
- Child rules: Specific content types or paths
- Nested child rules: Fine-grained control

### 3. Use Descriptive Names and Comments
```yaml
- name: "Mobile Image Optimization"
  comment: "Optimize images for mobile devices with smaller screens"
  criteria:
    - name: "userAgent"
      options:
        matchOperator: "MATCHES_ONE_OF"
        values: ["*Mobile*", "*Android*"]
```

### 4. Test on Staging First
Always test rule changes on the staging network before production:
```yaml
activation:
  network: "STAGING"
  notifyEmails: ["devops@example.com"]
  note: "Testing new rule configuration"
```

## Examples

See the sample configurations in the `config/samples/` directory:
- `akamai_v1alpha1_akamaiproperty.yaml`: Comprehensive example with advanced features
- `akamai_v1alpha1_akamaiproperty_simple_rules.yaml`: Simple example for basic use cases

## Troubleshooting

### Rule Validation Errors
Check the AkamaiProperty status for validation error details:
```bash
kubectl describe akamaiproperty <property-name>
```

### Update Failures
If rule updates fail, check:
1. Rule syntax and structure
2. Akamai API permissions
3. Property version conflicts
4. Network connectivity

### Common Issues
- **Variable naming**: Variable names must be uppercase and contain no spaces
- **Behavior options**: Ensure all required options are provided for behaviors
- **Criteria matching**: Verify criteria operators and values are correct
- **Rule names**: Top-level rule must be named "default"

## Advanced Features

### Conditional Logic
Use multiple criteria with different operators:
```yaml
criteria:
  - name: "hostname"
    options:
      matchOperator: "IS_ONE_OF"
      values: ["example.com", "www.example.com"]
  - name: "path"
    options:
      matchOperator: "DOES_NOT_MATCH_ONE_OF"
      values: ["/admin/*"]
```

### Nested Rules
Create complex rule hierarchies:
```yaml
children:
  - name: "Content Type Rules"
    criteria:
      - name: "contentType"
        options:
          matchOperator: "IS_ONE_OF"
          values: ["text/html"]
    children:
      - name: "Mobile HTML"
        criteria:
          - name: "userAgent"
            options:
              matchOperator: "MATCHES_ONE_OF"
              values: ["*Mobile*"]
        behaviors:
          - name: "imageManager"
            options:
              enabled: true
```

For more detailed information about specific behaviors and criteria, refer to the [Akamai Property Manager API documentation](https://techdocs.akamai.com/property-mgr/reference).