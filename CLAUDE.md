# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CDK Deployer is a Go CLI tool that clones public Git repositories containing AWS CDK projects and executes CDK operations (synth/deploy/drift detection) using the AWS CloudFormation SDK. It supports multi-language CDK projects: TypeScript, Python, Go, Java, and C#.

## Build and Run Commands

```bash
# Build the binary
go build -o cdk-deployer .

# Run examples
./cdk-deployer -repo https://github.com/user/cdk-project.git
./cdk-deployer -repo <repo-url> -cmd synth
./cdk-deployer -repo <repo-url> -cmd deploy
./cdk-deployer -repo <repo-url> -cmd drift -stack MyStack
./cdk-deployer -repo <repo-url> -cleanup=false -dest /tmp/my-project
```

## Architecture Overview

### Execution Flow

The application follows this pipeline:

1. **Clone** (`pkg/git/clone.go`) - Shallow clones the repository to temp directory
2. **Initialize** (`pkg/cdk/cdk.go`) - Detects project type and installs dependencies
3. **Synthesize** (`pkg/cdk/synthesizer.go`) - Runs `cdk synth` to generate CloudFormation templates
4. **Deploy/Drift** (`pkg/cdk/deployer.go`) - Uses AWS CloudFormation SDK directly (not CDK CLI)

### Core Components

**`pkg/cdk/cdk.go`** - Main orchestrator coordinating synthesizer and deployer operations.

**`pkg/cdk/synthesizer.go`** - Handles CDK synthesis:
- Detects project language by checking marker files (package.json, requirements.txt, go.mod, pom.xml, *.csproj)
- Installs dependencies using language-specific package managers
- **Python-specific**: Creates virtual environment at `.venv/`, modifies app command to use venv's python, sets VIRTUAL_ENV and PATH environment variables
- **TypeScript-specific**: Attempts `npx tsc` compilation if tsconfig.json exists (ignores errors for ts-node projects)
- Reads `cdk.json` to extract the `app` command
- Runs `npx cdk synth --app <cmd> --output cdk.out`
- Discovers stack names by finding `*.template.json` files in `cdk.out/`

**`pkg/cdk/deployer.go`** - CloudFormation operations via AWS SDK v2:
- **Deployment**: Checks if stack exists via DescribeStacks, then creates or updates with full IAM capabilities (CAPABILITY_IAM, CAPABILITY_NAMED_IAM, CAPABILITY_AUTO_EXPAND)
- **Polling**: Checks stack status every 10 seconds with 30-minute timeout
- **Drift detection**: Initiates DetectStackDrift, polls every 5 seconds with 10-minute timeout, retrieves resource drifts filtered for MODIFIED/DELETED/NOT_CHECKED status, includes property-level differences

**`pkg/cdk/types.go`** - Type definitions for SynthResult, DeployResult, DriftResult, and related structures.

### Critical Implementation Details

- Always uses `npx cdk synth` (not language-specific CDK binaries) because it delegates to the app command from cdk.json
- CloudFormation templates are read directly from filesystem at `cdk.out/<stack-name>.template.json`
- Deployments use AWS SDK CloudFormation client, NOT the CDK CLI deployment mechanism
- Stack operations (deploy/drift) run sequentially through `DeployAll`/`DetectDriftAll`
- Python projects: app command is rewritten to use venv's python path (replaces `python` or `python3` prefix)
- Signal handling (SIGINT/SIGTERM) triggers context cancellation for graceful cleanup

### Data Types

```go
SynthResult {TemplateDir, Stacks}
DeployResult {StackName, StackID, Status, Outputs}
DriftResult {StackName, DriftStatus, DriftedResources}
DriftedResource {LogicalID, PhysicalID, ResourceType, DriftStatus, PropertyDiffs}
```

## AWS Configuration

Requires AWS credentials configured via environment variables, AWS CLI, or IAM role. See README.md for required CloudFormation permissions plus permissions for resources created by CDK stacks.
