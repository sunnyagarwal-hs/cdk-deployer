package cdk

// CDKConfig represents the cdk.json configuration
type CDKConfig struct {
	App     string                 `json:"app"`
	Context map[string]interface{} `json:"context"`
}

// StackOutput represents a CloudFormation stack output
type StackOutput struct {
	Key   string
	Value string
}

// DeployResult contains the result of a deployment
type DeployResult struct {
	StackName string
	StackID   string
	Status    string
	Outputs   []StackOutput
}

// SynthResult contains the result of synthesis
type SynthResult struct {
	TemplateDir string
	Stacks      []string
}

// DriftResult contains the result of drift detection
type DriftResult struct {
	StackName        string
	DriftStatus      string
	DriftedResources []DriftedResource
}

// DriftedResource represents a resource that has drifted
type DriftedResource struct {
	LogicalID     string
	PhysicalID    string
	ResourceType  string
	DriftStatus   string
	PropertyDiffs []PropertyDiff
}

// PropertyDiff represents a property difference in a drifted resource
type PropertyDiff struct {
	PropertyPath   string
	ExpectedValue  string
	ActualValue    string
	DifferenceType string
}
