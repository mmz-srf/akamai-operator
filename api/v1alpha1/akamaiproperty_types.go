package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AkamaiPropertySpec defines the desired state of AkamaiProperty
type AkamaiPropertySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// PropertyName is the name of the Akamai property
	PropertyName string `json:"propertyName"`

	// GroupID is the Akamai group ID where the property should be created
	GroupID string `json:"groupId"`

	// ContractID is the Akamai contract ID
	ContractID string `json:"contractId"`

	// ProductID is the Akamai product ID (e.g., "prd_Fresca")
	ProductID string `json:"productId"`

	// Hostnames are the hostnames that this property should handle
	Hostnames []Hostname `json:"hostnames,omitempty"`

	// Rules contains the property rules configuration
	Rules *PropertyRules `json:"rules,omitempty"`

	// EdgeHostname specifies the edge hostname configuration
	EdgeHostname *EdgeHostnameSpec `json:"edgeHostname,omitempty"`
}

// Hostname represents a hostname configuration for the property
type Hostname struct {
	// CNAMEFrom is the hostname that will be CNAMEd
	CNAMEFrom string `json:"cnameFrom"`

	// CNAMETo is the edge hostname target
	CNAMETo string `json:"cnameTo"`

	// CertProvisioningType specifies how SSL certificates are provisioned
	CertProvisioningType string `json:"certProvisioningType,omitempty"`
}

// PropertyRules contains the rules configuration for the property
type PropertyRules struct {
	// Name is the name of the rule
	Name string `json:"name"`

	// Criteria defines the match criteria for the rule
	Criteria []RuleCriteria `json:"criteria,omitempty"`

	// Behaviors defines the behaviors to apply when criteria match
	Behaviors []RuleBehavior `json:"behaviors,omitempty"`

	// Children contains nested rules
	Children []PropertyRules `json:"children,omitempty"`
}

// RuleCriteria defines a criterion for rule matching
type RuleCriteria struct {
	// Name is the criterion type (e.g., "hostname", "path")
	Name string `json:"name"`

	// Options contains the criterion configuration
	// +kubebuilder:pruning:PreserveUnknownFields
	Options map[string]string `json:"options,omitempty"`
}

// RuleBehavior defines a behavior to apply
type RuleBehavior struct {
	// Name is the behavior type (e.g., "origin", "caching")
	Name string `json:"name"`

	// Options contains the behavior configuration
	// +kubebuilder:pruning:PreserveUnknownFields
	Options map[string]string `json:"options,omitempty"`
}

// EdgeHostnameSpec defines the edge hostname configuration
type EdgeHostnameSpec struct {
	// DomainPrefix is the prefix for the edge hostname
	DomainPrefix string `json:"domainPrefix"`

	// DomainSuffix is the suffix for the edge hostname
	DomainSuffix string `json:"domainSuffix"`

	// SecureNetwork specifies the secure network type
	SecureNetwork string `json:"secureNetwork,omitempty"`

	// IPVersionBehavior specifies IP version behavior
	IPVersionBehavior string `json:"ipVersionBehavior,omitempty"`
}

// AkamaiPropertyStatus defines the observed state of AkamaiProperty
type AkamaiPropertyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// PropertyID is the Akamai property ID
	PropertyID string `json:"propertyId,omitempty"`

	// LatestVersion is the latest version of the property
	LatestVersion int `json:"latestVersion,omitempty"`

	// StagingVersion is the version deployed to staging
	StagingVersion int `json:"stagingVersion,omitempty"`

	// ProductionVersion is the version deployed to production
	ProductionVersion int `json:"productionVersion,omitempty"`

	// Conditions represent the latest available observations of the property's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase represents the current phase of the property lifecycle
	Phase string `json:"phase,omitempty"`

	// LastUpdated is the timestamp when the property was last updated
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Property ID",type=string,JSONPath=`.status.propertyId`
//+kubebuilder:printcolumn:name="Latest Version",type=integer,JSONPath=`.status.latestVersion`
//+kubebuilder:printcolumn:name="Staging Version",type=integer,JSONPath=`.status.stagingVersion`
//+kubebuilder:printcolumn:name="Production Version",type=integer,JSONPath=`.status.productionVersion`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AkamaiProperty is the Schema for the akamaiproperties API
type AkamaiProperty struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AkamaiPropertySpec   `json:"spec,omitempty"`
	Status AkamaiPropertyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AkamaiPropertyList contains a list of AkamaiProperty
type AkamaiPropertyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AkamaiProperty `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AkamaiProperty{}, &AkamaiPropertyList{})
}
