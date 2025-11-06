package controllers

const (
	// FinalizerName is the finalizer added to AkamaiProperty resources
	FinalizerName = "akamai.com/finalizer"

	// Condition types
	ConditionTypeReady       = "Ready"
	ConditionTypeAvailable   = "Available"
	ConditionTypeProgressing = "Progressing"

	// Phase constants
	PhaseCreating   = "Creating"
	PhaseReady      = "Ready"
	PhaseUpdating   = "Updating"
	PhaseActivating = "Activating"
	PhaseError      = "Error"
	PhaseDeleting   = "Deleting"
)
