package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cdk-deployer/pkg/cdk"
	"cdk-deployer/pkg/git"
)

func main() {
	// Define CLI flags
	repoURL := flag.String("repo", "", "Public Git repository URL to clone")
	command := flag.String("cmd", "deploy", "CDK command to run: synth, deploy, or drift")
	stackName := flag.String("stack", "", "Stack name for drift detection (optional, uses synth to discover stacks if not provided)")
	cleanup := flag.Bool("cleanup", true, "Clean up cloned repository after operation")
	destDir := flag.String("dest", "", "Destination directory for cloning (default: temp directory)")

	flag.Parse()

	if *repoURL == "" {
		fmt.Println("Usage: cdk-deployer -repo <git-url> [-cmd synth|deploy|drift] [-cleanup=true|false] [-dest <dir>]")
		fmt.Println("\nExamples:")
		fmt.Println("  cdk-deployer -repo https://github.com/user/cdk-project.git")
		fmt.Println("  cdk-deployer -repo https://github.com/user/cdk-project.git -cmd synth")
		fmt.Println("  cdk-deployer -repo https://github.com/user/cdk-project.git -cmd deploy -cleanup=false")
		fmt.Println("  cdk-deployer -repo https://github.com/user/cdk-project.git -cmd drift")
		fmt.Println("  cdk-deployer -repo https://github.com/user/cdk-project.git -cmd drift -stack MyStack")
		os.Exit(1)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, cleaning up...")
		cancel()
	}()

	// Run the CDK deployer
	if err := run(ctx, *repoURL, *command, *destDir, *stackName, *cleanup); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, repoURL, command, destDir, stackName string, cleanup bool) error {
	// Clone the repository
	projectPath, err := git.CloneRepository(repoURL, destDir)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Cleanup if requested
	if cleanup {
		defer func() {
			fmt.Printf("Cleaning up %s...\n", projectPath)
			if err := git.CleanupRepository(projectPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to cleanup: %v\n", err)
			}
		}()
	} else {
		fmt.Printf("Repository cloned to: %s\n", projectPath)
	}

	// Create CDK instance
	cdkApp := cdk.New(projectPath)

	// Initialize the project
	if err := cdkApp.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize CDK project: %w", err)
	}

	switch command {
	case "synth":
		result, err := cdkApp.Synth()
		if err != nil {
			return fmt.Errorf("synthesis failed: %w", err)
		}
		fmt.Printf("\nSynthesis complete!\n")
		fmt.Printf("Template directory: %s\n", result.TemplateDir)
		fmt.Printf("Stacks: %v\n", result.Stacks)

	case "deploy":
		// First synthesize
		synthResult, err := cdkApp.Synth()
		if err != nil {
			return fmt.Errorf("synthesis failed: %w", err)
		}
		fmt.Printf("Synthesized %d stack(s)\n", len(synthResult.Stacks))

		// Then deploy
		results, err := cdkApp.Deploy(ctx, synthResult.Stacks)
		if err != nil {
			return fmt.Errorf("deployment failed: %w", err)
		}

		fmt.Printf("\nDeployment complete!\n")
		for _, r := range results {
			fmt.Printf("\nStack: %s\n", r.StackName)
			fmt.Printf("Status: %s\n", r.Status)
			if len(r.Outputs) > 0 {
				fmt.Println("Outputs:")
				for _, o := range r.Outputs {
					fmt.Printf("  %s: %s\n", o.Key, o.Value)
				}
			}
		}

	case "drift":
		var stacks []string
		if stackName != "" {
			stacks = []string{stackName}
		} else {
			// Synthesize to discover stack names
			synthResult, err := cdkApp.Synth()
			if err != nil {
				return fmt.Errorf("synthesis failed: %w", err)
			}
			stacks = synthResult.Stacks
		}

		fmt.Printf("Detecting drift for %d stack(s)...\n", len(stacks))

		results, err := cdkApp.DetectDrift(ctx, stacks)
		if err != nil {
			return fmt.Errorf("drift detection failed: %w", err)
		}

		fmt.Printf("\nDrift Detection Complete!\n")
		for _, r := range results {
			fmt.Printf("\nStack: %s\n", r.StackName)
			fmt.Printf("Drift Status: %s\n", r.DriftStatus)
			if len(r.DriftedResources) > 0 {
				fmt.Println("Drifted Resources:")
				for _, dr := range r.DriftedResources {
					fmt.Printf("  - %s (%s)\n", dr.LogicalID, dr.ResourceType)
					fmt.Printf("    Physical ID: %s\n", dr.PhysicalID)
					fmt.Printf("    Status: %s\n", dr.DriftStatus)
					if len(dr.PropertyDiffs) > 0 {
						fmt.Println("    Property Differences:")
						for _, pd := range dr.PropertyDiffs {
							fmt.Printf("      %s: expected=%s, actual=%s (%s)\n",
								pd.PropertyPath, pd.ExpectedValue, pd.ActualValue, pd.DifferenceType)
						}
					}
				}
			} else {
				fmt.Println("No drifted resources found.")
			}
		}

	default:
		return fmt.Errorf("unknown command: %s (use 'synth', 'deploy', or 'drift')", command)
	}

	return nil
}
