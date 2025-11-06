package akamai

// Property represents an Akamai property
type Property struct {
	PropertyID        string     `json:"propertyId"`
	PropertyName      string     `json:"propertyName"`
	AccountID         string     `json:"accountId"`
	ContractID        string     `json:"contractId"`
	GroupID           string     `json:"groupId"`
	ProductID         string     `json:"productId"`
	LatestVersion     int        `json:"latestVersion"`
	StagingVersion    int        `json:"stagingVersion"`
	ProductionVersion int        `json:"productionVersion"`
	Hostnames         []Hostname `json:"hostnames"`
}

// Hostname represents a hostname configuration
type Hostname struct {
	CNAMEFrom            string `json:"cnameFrom"`
	CNAMETo              string `json:"cnameTo"`
	CertProvisioningType string `json:"certProvisioningType"`
}

// Activation represents an activation status
type Activation struct {
	ActivationID    string   `json:"activationId"`
	PropertyID      string   `json:"propertyId"`
	PropertyVersion int      `json:"propertyVersion"`
	Network         string   `json:"network"`
	Status          string   `json:"status"`
	SubmitDate      string   `json:"submitDate"`
	UpdateDate      string   `json:"updateDate"`
	Note            string   `json:"note"`
	NotifyEmails    []string `json:"notifyEmails"`
	CanFastFallback bool     `json:"canFastFallback"`
	FallbackVersion int      `json:"fallbackVersion,omitempty"`
}

// PropertyRules represents a property rule tree response from Akamai
type PropertyRules struct {
	AccountID       string      `json:"accountId"`
	ContractID      string      `json:"contractId"`
	GroupID         string      `json:"groupId"`
	PropertyID      string      `json:"propertyId"`
	PropertyVersion int         `json:"propertyVersion"`
	Etag            string      `json:"etag"`
	RuleFormat      string      `json:"ruleFormat"`
	Rules           interface{} `json:"rules"`
}
