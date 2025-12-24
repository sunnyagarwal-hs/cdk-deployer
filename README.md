# CDK Deployer

A Go program that clones a public Git repository containing an AWS CDK project and runs CDK operations (synth/deploy) using the AWS CloudFormation SDK.

## Features

- **Git Clone**: Clones public Git repositories using go-git
- **CDK Synth**: Synthesizes CloudFormation templates from CDK code
- **CDK Deploy**: Deploys stacks directly via AWS CloudFormation API
- **Multi-language Support**: Detects and handles TypeScript, Python, Go, Java, and C# CDK projects

## Prerequisites

- Go 1.23+
- AWS credentials configured (via environment variables, AWS CLI, or IAM role)
- Node.js and npm (for TypeScript/JavaScript CDK projects)
- Python and pip (for Python CDK projects)
- CDK CLI installed globally: `npm install -g aws-cdk`

## Installation

```bash
go build -o cdk-deployer .
```

## Usage

```bash
# Deploy a CDK project from a Git repository
./cdk-deployer -repo https://github.com/user/cdk-project.git

# Only synthesize (generate CloudFormation templates)
./cdk-deployer -repo https://github.com/user/cdk-project.git -cmd synth

# Deploy without cleaning up the cloned repo
./cdk-deployer -repo https://github.com/user/cdk-project.git -cleanup=false

# Clone to a specific directory
./cdk-deployer -repo https://github.com/user/cdk-project.git -dest /tmp/my-cdk-project
```

## CLI Options

| Flag | Default | Description |
|------|---------|-------------|
| `-repo` | (required) | Public Git repository URL |
| `-cmd` | `deploy` | Command to run: `synth` or `deploy` |
| `-cleanup` | `true` | Clean up cloned repository after operation |
| `-dest` | temp dir | Destination directory for cloning |

## Architecture

```
cdk-deployer/
├── main.go                 # CLI entry point
├── pkg/
│   ├── git/
│   │   └── clone.go        # Git operations (clone, cleanup)
│   └── cdk/
│       ├── cdk.go          # Main CDK interface
│       ├── types.go        # Type definitions
│       ├── synthesizer.go  # CDK synthesis logic
│       └── deployer.go     # CloudFormation deployment
└── go.mod
```

## How It Works

1. **Clone**: Uses go-git to shallow clone the repository
2. **Detect**: Identifies the CDK project type (TypeScript, Python, etc.)
3. **Install**: Installs project dependencies (npm install, pip install, etc.)
4. **Synth**: Runs `cdk synth` to generate CloudFormation templates
5. **Deploy**: Uses AWS CloudFormation SDK to create/update stacks

## AWS Permissions

The deployer requires CloudFormation permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "cloudformation:CreateStack",
        "cloudformation:UpdateStack",
        "cloudformation:DescribeStacks",
        "cloudformation:DescribeStackEvents"
      ],
      "Resource": "*"
    }
  ]
}
```

Additional permissions depend on the resources your CDK stacks create (IAM, S3, Lambda, etc.).

## License

MIT
