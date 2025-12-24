package cdk

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

// Deployer handles CloudFormation deployment operations
type Deployer struct {
	cfnClient   *cloudformation.Client
	synthesizer *Synthesizer
}

// NewDeployer creates a new CloudFormation deployer
func NewDeployer(ctx context.Context, synthesizer *Synthesizer) (*Deployer, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Deployer{
		cfnClient:   cloudformation.NewFromConfig(cfg),
		synthesizer: synthesizer,
	}, nil
}

// Deploy deploys a CloudFormation stack
func (d *Deployer) Deploy(ctx context.Context, stackName string) (*DeployResult, error) {
	templateBody, err := d.synthesizer.GetTemplateBody(stackName)
	if err != nil {
		return nil, err
	}

	// Check if stack exists
	exists, err := d.stackExists(ctx, stackName)
	if err != nil {
		return nil, err
	}

	var stackID string
	if exists {
		stackID, err = d.updateStack(ctx, stackName, templateBody)
	} else {
		stackID, err = d.createStack(ctx, stackName, templateBody)
	}

	if err != nil {
		return nil, err
	}

	// Wait for stack operation to complete
	status, err := d.waitForStack(ctx, stackName)
	if err != nil {
		return nil, err
	}

	// Get stack outputs
	outputs, err := d.getStackOutputs(ctx, stackName)
	if err != nil {
		return nil, err
	}

	return &DeployResult{
		StackName: stackName,
		StackID:   stackID,
		Status:    status,
		Outputs:   outputs,
	}, nil
}

// stackExists checks if a CloudFormation stack exists
func (d *Deployer) stackExists(ctx context.Context, stackName string) (bool, error) {
	input := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}

	_, err := d.cfnClient.DescribeStacks(ctx, input)
	if err != nil {
		// Check if it's a "stack does not exist" error
		return false, nil
	}

	return true, nil
}

// createStack creates a new CloudFormation stack
func (d *Deployer) createStack(ctx context.Context, stackName, templateBody string) (string, error) {
	fmt.Printf("Creating stack: %s\n", stackName)

	input := &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(templateBody),
		Capabilities: []types.Capability{
			types.CapabilityCapabilityIam,
			types.CapabilityCapabilityNamedIam,
			types.CapabilityCapabilityAutoExpand,
		},
		OnFailure: types.OnFailureRollback,
	}

	output, err := d.cfnClient.CreateStack(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to create stack: %w", err)
	}

	return aws.ToString(output.StackId), nil
}

// updateStack updates an existing CloudFormation stack
func (d *Deployer) updateStack(ctx context.Context, stackName, templateBody string) (string, error) {
	fmt.Printf("Updating stack: %s\n", stackName)

	input := &cloudformation.UpdateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(templateBody),
		Capabilities: []types.Capability{
			types.CapabilityCapabilityIam,
			types.CapabilityCapabilityNamedIam,
			types.CapabilityCapabilityAutoExpand,
		},
	}

	output, err := d.cfnClient.UpdateStack(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to update stack: %w", err)
	}

	return aws.ToString(output.StackId), nil
}

// waitForStack waits for a stack operation to complete
func (d *Deployer) waitForStack(ctx context.Context, stackName string) (string, error) {
	fmt.Printf("Waiting for stack %s to complete...\n", stackName)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	timeout := time.After(30 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for stack %s", stackName)
		case <-ticker.C:
			status, err := d.getStackStatus(ctx, stackName)
			if err != nil {
				return "", err
			}

			fmt.Printf("Stack status: %s\n", status)

			switch status {
			case string(types.StackStatusCreateComplete),
				string(types.StackStatusUpdateComplete):
				return status, nil
			case string(types.StackStatusCreateFailed),
				string(types.StackStatusRollbackComplete),
				string(types.StackStatusRollbackFailed),
				string(types.StackStatusUpdateRollbackComplete),
				string(types.StackStatusUpdateRollbackFailed),
				string(types.StackStatusDeleteComplete),
				string(types.StackStatusDeleteFailed):
				return status, fmt.Errorf("stack operation failed with status: %s", status)
			}
		}
	}
}

// getStackStatus returns the current status of a stack
func (d *Deployer) getStackStatus(ctx context.Context, stackName string) (string, error) {
	input := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}

	output, err := d.cfnClient.DescribeStacks(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to describe stack: %w", err)
	}

	if len(output.Stacks) == 0 {
		return "", fmt.Errorf("stack %s not found", stackName)
	}

	return string(output.Stacks[0].StackStatus), nil
}

// getStackOutputs returns the outputs of a stack
func (d *Deployer) getStackOutputs(ctx context.Context, stackName string) ([]StackOutput, error) {
	input := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}

	output, err := d.cfnClient.DescribeStacks(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe stack: %w", err)
	}

	if len(output.Stacks) == 0 {
		return nil, nil
	}

	var outputs []StackOutput
	for _, o := range output.Stacks[0].Outputs {
		outputs = append(outputs, StackOutput{
			Key:   aws.ToString(o.OutputKey),
			Value: aws.ToString(o.OutputValue),
		})
	}

	return outputs, nil
}

// DeployAll deploys all stacks from the synthesized output
func (d *Deployer) DeployAll(ctx context.Context, stacks []string) ([]DeployResult, error) {
	var results []DeployResult

	for _, stackName := range stacks {
		result, err := d.Deploy(ctx, stackName)
		if err != nil {
			return results, fmt.Errorf("failed to deploy stack %s: %w", stackName, err)
		}
		results = append(results, *result)
	}

	return results, nil
}

// DetectDrift initiates drift detection for a stack and returns the results
func (d *Deployer) DetectDrift(ctx context.Context, stackName string) (*DriftResult, error) {
	// Check if stack exists
	exists, err := d.stackExists(ctx, stackName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("stack %s does not exist", stackName)
	}

	fmt.Printf("Initiating drift detection for stack: %s\n", stackName)

	// Start drift detection
	detectInput := &cloudformation.DetectStackDriftInput{
		StackName: aws.String(stackName),
	}

	detectOutput, err := d.cfnClient.DetectStackDrift(ctx, detectInput)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate drift detection: %w", err)
	}

	driftDetectionId := aws.ToString(detectOutput.StackDriftDetectionId)
	fmt.Printf("Drift detection started (ID: %s)\n", driftDetectionId)

	// Wait for drift detection to complete
	status, err := d.waitForDriftDetection(ctx, driftDetectionId)
	if err != nil {
		return nil, err
	}

	if status != string(types.StackDriftDetectionStatusDetectionComplete) {
		return nil, fmt.Errorf("drift detection failed with status: %s", status)
	}

	// Get drift detection results
	return d.getDriftResults(ctx, stackName)
}

// waitForDriftDetection waits for drift detection to complete
func (d *Deployer) waitForDriftDetection(ctx context.Context, driftDetectionId string) (string, error) {
	fmt.Println("Waiting for drift detection to complete...")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(10 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for drift detection")
		case <-ticker.C:
			input := &cloudformation.DescribeStackDriftDetectionStatusInput{
				StackDriftDetectionId: aws.String(driftDetectionId),
			}

			output, err := d.cfnClient.DescribeStackDriftDetectionStatus(ctx, input)
			if err != nil {
				return "", fmt.Errorf("failed to get drift detection status: %w", err)
			}

			status := string(output.DetectionStatus)
			fmt.Printf("Drift detection status: %s\n", status)

			switch output.DetectionStatus {
			case types.StackDriftDetectionStatusDetectionComplete:
				return status, nil
			case types.StackDriftDetectionStatusDetectionFailed:
				reason := aws.ToString(output.DetectionStatusReason)
				return status, fmt.Errorf("drift detection failed: %s", reason)
			}
		}
	}
}

// getDriftResults retrieves the drift detection results for a stack
func (d *Deployer) getDriftResults(ctx context.Context, stackName string) (*DriftResult, error) {
	// Get stack drift status
	describeInput := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}

	describeOutput, err := d.cfnClient.DescribeStacks(ctx, describeInput)
	if err != nil {
		return nil, fmt.Errorf("failed to describe stack: %w", err)
	}

	if len(describeOutput.Stacks) == 0 {
		return nil, fmt.Errorf("stack %s not found", stackName)
	}

	stack := describeOutput.Stacks[0]
	result := &DriftResult{
		StackName:   stackName,
		DriftStatus: string(stack.DriftInformation.StackDriftStatus),
	}

	// Get detailed resource drifts
	resourceDriftsInput := &cloudformation.DescribeStackResourceDriftsInput{
		StackName: aws.String(stackName),
		StackResourceDriftStatusFilters: []types.StackResourceDriftStatus{
			types.StackResourceDriftStatusModified,
			types.StackResourceDriftStatusDeleted,
			types.StackResourceDriftStatusNotChecked,
		},
	}

	resourceDriftsOutput, err := d.cfnClient.DescribeStackResourceDrifts(ctx, resourceDriftsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource drifts: %w", err)
	}

	for _, rd := range resourceDriftsOutput.StackResourceDrifts {
		driftedResource := DriftedResource{
			LogicalID:    aws.ToString(rd.LogicalResourceId),
			PhysicalID:   aws.ToString(rd.PhysicalResourceId),
			ResourceType: aws.ToString(rd.ResourceType),
			DriftStatus:  string(rd.StackResourceDriftStatus),
		}

		// Parse property differences
		for _, pd := range rd.PropertyDifferences {
			driftedResource.PropertyDiffs = append(driftedResource.PropertyDiffs, PropertyDiff{
				PropertyPath:   aws.ToString(pd.PropertyPath),
				ExpectedValue:  aws.ToString(pd.ExpectedValue),
				ActualValue:    aws.ToString(pd.ActualValue),
				DifferenceType: string(pd.DifferenceType),
			})
		}

		result.DriftedResources = append(result.DriftedResources, driftedResource)
	}

	return result, nil
}

// DetectDriftAll detects drift for all stacks
func (d *Deployer) DetectDriftAll(ctx context.Context, stacks []string) ([]DriftResult, error) {
	var results []DriftResult

	for _, stackName := range stacks {
		result, err := d.DetectDrift(ctx, stackName)
		if err != nil {
			return results, fmt.Errorf("failed to detect drift for stack %s: %w", stackName, err)
		}
		results = append(results, *result)
	}

	return results, nil
}
