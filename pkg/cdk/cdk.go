package cdk

import (
	"context"
	"fmt"
)

// CDK is the main interface for CDK operations
type CDK struct {
	projectPath string
	synthesizer *Synthesizer
	deployer    *Deployer
}

// New creates a new CDK instance for a project
func New(projectPath string) *CDK {
	return &CDK{
		projectPath: projectPath,
		synthesizer: NewSynthesizer(projectPath),
	}
}

// Initialize prepares the CDK project for synthesis
func (c *CDK) Initialize() error {
	// Detect project type
	projectType, err := c.synthesizer.DetectProjectType()
	if err != nil {
		return fmt.Errorf("failed to detect project type: %w", err)
	}
	fmt.Printf("Detected project type: %s\n", projectType)

	// Install dependencies
	if err := c.synthesizer.InstallDependencies(projectType); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	return nil
}

// Synth synthesizes the CDK app
func (c *CDK) Synth() (*SynthResult, error) {
	return c.synthesizer.Synth()
}

// Deploy deploys all stacks
func (c *CDK) Deploy(ctx context.Context, stacks []string) ([]DeployResult, error) {
	if c.deployer == nil {
		deployer, err := NewDeployer(ctx, c.synthesizer)
		if err != nil {
			return nil, err
		}
		c.deployer = deployer
	}

	return c.deployer.DeployAll(ctx, stacks)
}

// SynthAndDeploy synthesizes and deploys all stacks
func (c *CDK) SynthAndDeploy(ctx context.Context) ([]DeployResult, error) {
	// Initialize project
	if err := c.Initialize(); err != nil {
		return nil, err
	}

	// Synthesize
	synthResult, err := c.Synth()
	if err != nil {
		return nil, fmt.Errorf("synthesis failed: %w", err)
	}

	fmt.Printf("Synthesized %d stack(s): %v\n", len(synthResult.Stacks), synthResult.Stacks)

	// Deploy
	return c.Deploy(ctx, synthResult.Stacks)
}

// DetectDrift detects drift for specified stacks
func (c *CDK) DetectDrift(ctx context.Context, stacks []string) ([]DriftResult, error) {
	if c.deployer == nil {
		deployer, err := NewDeployer(ctx, c.synthesizer)
		if err != nil {
			return nil, err
		}
		c.deployer = deployer
	}

	return c.deployer.DetectDriftAll(ctx, stacks)
}
