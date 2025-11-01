# Property Activation in Akamai Operator

This document explains how property activation works in the Akamai Operator, including the activation lifecycle, API interactions, and status tracking.

## Overview

Property activation is the process of deploying property configurations to Akamai's edge network. The Akamai Operator automates this process by:

1. **Creating** or **updating** property configurations
2. **Activating** properties on specified networks (STAGING or PRODUCTION)
3. **Monitoring** activation progress and status
4. **Tracking** active versions across different networks

## Activation Workflow

```mermaid
flowchart TD
    A[AkamaiProperty Resource Created/Updated] --> B{Has Activation Spec?}
    B -->|No| C[Property Created/Updated Only]
    B -->|Yes| D[Check Current Activation Status]
    
    D --> E{Existing Activation?}
    E -->|No| F[Start New Activation]
    E -->|Yes| G{Activation in Progress?}
    
    G -->|Yes| H[Check Activation Status]
    G -->|No| I{Newer Version Available?}
    
    I -->|Yes| F
    I -->|No| J[No Action Required]
    
    H --> K{Status Check}
    K -->|PENDING/ACTIVATING| L[Continue Monitoring]
    K -->|ACTIVE| M[Activation Complete]
    K -->|FAILED| N[Report Error]
    
    F --> O[Call Akamai API]
    O --> P[Store Activation ID]
    P --> L
    
    L --> Q[Wait 2 Minutes]
    Q --> H
    
    M --> R[Update Resource Status]
    N --> S[Update Error Status]
    
    style F fill:#e1f5fe
    style M fill:#e8f5e8
    style N fill:#ffebee
```

## Activation States

The operator tracks activation through several states:

```mermaid
stateDiagram-v2
    [*] --> Creating: Property Creation
    Creating --> Ready: Property Created
    Ready --> Activating: Activation Requested
    Activating --> Activating: Status Polling
    Activating --> Ready: Activation Success
    Activating --> Error: Activation Failed
    Error --> Activating: Retry
    Ready --> Updating: Property Modified
    Updating --> Ready: Update Complete
    Updating --> Activating: Auto-activation
```

## API Interaction Flow

```mermaid
sequenceDiagram
    participant K as Kubernetes
    participant O as Operator
    participant A as Akamai API
    
    K->>O: Create/Update AkamaiProperty
    Note over O: Check activation spec
    
    alt Has activation configuration
        O->>A: GET /properties/{id}/activations
        A-->>O: Current activations list
        
        alt Needs new activation
            O->>A: POST /properties/{id}/activations
            Note over A: Start activation process
            A-->>O: Activation ID & Link
            O->>K: Update status with activation ID
            
            loop Monitor activation
                O->>A: GET /properties/{id}/activations/{id}
                A-->>O: Activation status
                
                alt Status: PENDING/ACTIVATING
                    O->>K: Update status (in progress)
                    Note over O: Wait 2 minutes
                else Status: ACTIVE
                    O->>K: Update status (complete)
                    Note over O: Activation successful
                else Status: FAILED
                    O->>K: Update status (error)
                    Note over O: Report failure
                end
            end
        end
    end
```

## Configuration Structure

### Activation Specification

```yaml
apiVersion: akamai.com/v1alpha1
kind: AkamaiProperty
metadata:
  name: my-property
spec:
  # ... property configuration ...
  
  activation:
    # Target network (required)
    network: "STAGING"  # or "PRODUCTION"
    
    # Notification emails (required)
    notifyEmails:
      - "admin@example.com"
      - "devops@example.com"
    
    # Optional fields
    note: "Automated activation via Kubernetes"
    acknowledgeAllWarnings: true
    useFastFallback: false
    fastPush: true
    ignoreHttpErrors: true
```

### Status Tracking

```yaml
status:
  propertyId: "prp_123456"
  latestVersion: 3
  
  # Staging activation info
  stagingVersion: 2
  stagingActivationId: "atv_789012"
  stagingActivationStatus: "ACTIVE"
  
  # Production activation info
  productionVersion: 1
  productionActivationId: "atv_345678"
  productionActivationStatus: "PENDING"
  
  phase: "Activating"
  conditions:
    - type: "Ready"
      status: "False"
      reason: "ActivationInProgress"
      message: "Activation pending on PRODUCTION network"
```

## Network Targeting

```mermaid
graph LR
    A[Property Version] --> B{Activation Network}
    B -->|STAGING| C[Staging Network]
    B -->|PRODUCTION| D[Production Network]
    
    C --> E[Testing Environment]
    D --> F[Live Traffic]
    
    E --> G[Quality Assurance]
    G --> H[Promote to Production]
    H --> D
    
    style C fill:#fff3e0
    style D fill:#e8f5e8
```

## Activation Lifecycle Management

### 1. **Initial Activation**
- Property created with activation spec
- Operator detects new activation requirement
- Calls Akamai API to start activation
- Begins status monitoring

### 2. **Status Monitoring**
- Polls activation status every 2 minutes
- Updates Kubernetes resource status
- Handles state transitions (PENDING → ACTIVATING → ACTIVE)

### 3. **Version Updates**
- When property is updated, new version is created
- Operator detects version mismatch
- Automatically triggers activation of new version

### 4. **Error Handling**
- Failed activations are reported in resource status
- Retry logic with exponential backoff
- Detailed error messages in conditions

## Multi-Network Support

```mermaid
graph TB
    subgraph "Property Management"
        A[AkamaiProperty Resource]
        A --> B[Latest Version: 3]
    end
    
    subgraph "Staging Network"
        C[Active Version: 3]
        D[Activation ID: atv_111]
        E[Status: ACTIVE]
    end
    
    subgraph "Production Network"
        F[Active Version: 2]
        G[Activation ID: atv_222]
        H[Status: ACTIVE]
    end
    
    A -.->|Activate STAGING| C
    A -.->|Activate PRODUCTION| F
    
    style C fill:#fff3e0
    style F fill:#e8f5e8
```

## Monitoring and Observability

### Resource Status Commands

```bash
# View all properties and their activation status
kubectl get akamaiproperties

# Get detailed status including activation info
kubectl describe akamaiproperty my-property

# Watch activation progress in real-time
kubectl get akamaiproperty my-property -w

# Check activation status in YAML format
kubectl get akamaiproperty my-property -o yaml | grep -A 20 status
```

### Example Status Output

```
NAME          PROPERTY ID   LATEST VERSION   STAGING VERSION   PRODUCTION VERSION   PHASE
my-property   prp_123456    3               3                 2                    Activating
```

## Best Practices

### 1. **Staging First Approach**
```mermaid
flowchart LR
    A[Develop] --> B[Deploy to Staging]
    B --> C[Test & Validate]
    C --> D{Tests Pass?}
    D -->|Yes| E[Deploy to Production]
    D -->|No| F[Fix Issues]
    F --> A
    
    style B fill:#fff3e0
    style E fill:#e8f5e8
```

### 2. **Notification Setup**
- Configure multiple email addresses for activation notifications
- Include both technical and business stakeholders
- Use team distribution lists for broader coverage

### 3. **Fast Fallback Configuration**
- Enable `useFastFallback: true` for production activations
- Allows quick rollback within 1 hour if issues are detected
- Only available when `canFastFallback` is true

### 4. **Activation Notes**
- Use descriptive notes to track activation purposes
- Include ticket numbers, change descriptions, or deployment contexts
- Helps with audit trails and troubleshooting

## Error Scenarios and Recovery

```mermaid
flowchart TD
    A[Activation Started] --> B{Activation Status}
    B -->|PENDING| C[Wait & Monitor]
    B -->|ACTIVATING| D[Continue Monitoring]
    B -->|ACTIVE| E[Success]
    B -->|FAILED| F[Activation Failed]
    
    F --> G{Error Type}
    G -->|Validation Error| H[Check Property Config]
    G -->|Network Error| I[Retry Activation]
    G -->|Permission Error| J[Check API Credentials]
    
    H --> K[Fix Configuration]
    I --> L[Automatic Retry]
    J --> M[Update Credentials]
    
    K --> N[Update Resource]
    L --> A
    M --> A
    
    style E fill:#e8f5e8
    style F fill:#ffebee
```

## Advanced Features

### Fast Fallback

When enabled, fast fallback allows quick rollback to the previous version:

```yaml
activation:
  network: "PRODUCTION"
  useFastFallback: true  # Enable fast rollback
  notifyEmails: ["ops@example.com"]
```

- Available for 1 hour after activation
- Only when `canFastFallback` is true
- Provides rapid recovery from deployment issues

### Activation Validation

The operator validates activation requests before submission:

- **Network validation**: Must be "STAGING" or "PRODUCTION"
- **Email validation**: At least one email address required
- **Version validation**: Ensures version exists and is activatable
- **Permission validation**: Checks API credentials and access rights

## Troubleshooting

### Common Issues

1. **Activation Stuck in PENDING**
   - Check Akamai API status
   - Verify network connectivity
   - Review activation logs

2. **Validation Errors**
   - Check property configuration
   - Verify hostnames and certificates
   - Review rule tree structure

3. **Permission Errors**
   - Verify API credentials
   - Check contract and group access
   - Ensure activation permissions

### Debug Commands

```bash
# Check operator logs
kubectl logs -n akamai-operator-system deployment/akamai-operator-controller-manager

# Get resource events
kubectl get events --field-selector involvedObject.name=my-property

# Describe resource for detailed status
kubectl describe akamaiproperty my-property
```

This comprehensive guide covers all aspects of property activation in the Akamai Operator, from basic concepts to advanced troubleshooting scenarios.
