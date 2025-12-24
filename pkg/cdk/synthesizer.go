package cdk

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Synthesizer handles CDK synthesis operations
type Synthesizer struct {
	projectPath string
	outputDir   string
}

// NewSynthesizer creates a new CDK synthesizer
func NewSynthesizer(projectPath string) *Synthesizer {
	return &Synthesizer{
		projectPath: projectPath,
		outputDir:   filepath.Join(projectPath, "cdk.out"),
	}
}

// DetectProjectType detects the CDK project type (typescript, python, go, java, csharp)
func (s *Synthesizer) DetectProjectType() (string, error) {
	// Check for TypeScript/JavaScript
	if _, err := os.Stat(filepath.Join(s.projectPath, "package.json")); err == nil {
		return "typescript", nil
	}

	// Check for Python
	if _, err := os.Stat(filepath.Join(s.projectPath, "requirements.txt")); err == nil {
		return "python", nil
	}
	if _, err := os.Stat(filepath.Join(s.projectPath, "setup.py")); err == nil {
		return "python", nil
	}

	// Check for Go
	if _, err := os.Stat(filepath.Join(s.projectPath, "go.mod")); err == nil {
		return "go", nil
	}

	// Check for Java
	if _, err := os.Stat(filepath.Join(s.projectPath, "pom.xml")); err == nil {
		return "java", nil
	}

	// Check for C#
	files, _ := filepath.Glob(filepath.Join(s.projectPath, "*.csproj"))
	if len(files) > 0 {
		return "csharp", nil
	}

	return "", fmt.Errorf("unable to detect CDK project type")
}

// InstallDependencies installs project dependencies based on project type
func (s *Synthesizer) InstallDependencies(projectType string) error {
	var cmd *exec.Cmd

	switch projectType {
	case "typescript":
		// Check if node_modules exists
		if _, err := os.Stat(filepath.Join(s.projectPath, "node_modules")); os.IsNotExist(err) {
			fmt.Println("Installing npm dependencies...")
			cmd = exec.Command("npm", "install")
		} else {
			fmt.Println("Dependencies already installed")
			return nil
		}
	case "python":
		return s.installPythonDependencies()
	case "go":
		fmt.Println("Installing Go dependencies...")
		cmd = exec.Command("go", "mod", "download")
	case "java":
		fmt.Println("Installing Java dependencies...")
		cmd = exec.Command("mvn", "dependency:resolve")
	default:
		return fmt.Errorf("unsupported project type: %s", projectType)
	}

	if cmd != nil {
		cmd.Dir = s.projectPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install dependencies: %w", err)
		}
	}

	return nil
}

// installPythonDependencies creates a virtual environment and installs dependencies
func (s *Synthesizer) installPythonDependencies() error {
	venvPath := filepath.Join(s.projectPath, ".venv")

	// Try python3 first, then python
	pythonCmd := "python3"
	if _, err := exec.LookPath("python3"); err != nil {
		pythonCmd = "python"
	}

	// Check Python version compatibility
	if err := s.checkPythonCompatibility(pythonCmd); err != nil {
		return err
	}

	// Check if venv already exists
	if _, err := os.Stat(venvPath); os.IsNotExist(err) {
		fmt.Println("Creating Python virtual environment...")

		cmd := exec.Command(pythonCmd, "-m", "venv", ".venv")
		cmd.Dir = s.projectPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create virtual environment: %w", err)
		}
	}

	fmt.Println("Installing Python dependencies in virtual environment...")

	// Install dependencies using the venv pip
	pipPath := filepath.Join(venvPath, "bin", "pip")
	cmd := exec.Command(pipPath, "install", "-r", "requirements.txt")
	cmd.Dir = s.projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	return nil
}

// getPythonVersion returns the version of the specified Python command
func getPythonVersion(pythonCmd string) (major, minor, patch int, err error) {
	cmd := exec.Command(pythonCmd, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get Python version: %w", err)
	}

	// Python version output format: "Python 3.9.7"
	versionStr := strings.TrimSpace(string(output))
	re := regexp.MustCompile(`Python (\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(versionStr)
	if len(matches) != 4 {
		// Try simpler pattern for versions like "Python 3.9"
		re = regexp.MustCompile(`Python (\d+)\.(\d+)`)
		matches = re.FindStringSubmatch(versionStr)
		if len(matches) != 3 {
			return 0, 0, 0, fmt.Errorf("failed to parse Python version from: %s", versionStr)
		}
		major, _ = strconv.Atoi(matches[1])
		minor, _ = strconv.Atoi(matches[2])
		patch = 0
		return major, minor, patch, nil
	}

	major, _ = strconv.Atoi(matches[1])
	minor, _ = strconv.Atoi(matches[2])
	patch, _ = strconv.Atoi(matches[3])
	return major, minor, patch, nil
}

// getRequiredPythonVersion reads Python version requirements from project files
func (s *Synthesizer) getRequiredPythonVersion() (minMajor, minMinor int, err error) {
	// Check .python-version file (commonly used by pyenv)
	pythonVersionFile := filepath.Join(s.projectPath, ".python-version")
	if data, err := os.ReadFile(pythonVersionFile); err == nil {
		versionStr := strings.TrimSpace(string(data))
		re := regexp.MustCompile(`^(\d+)\.(\d+)`)
		matches := re.FindStringSubmatch(versionStr)
		if len(matches) == 3 {
			major, _ := strconv.Atoi(matches[1])
			minor, _ := strconv.Atoi(matches[2])
			return major, minor, nil
		}
	}

	// Check setup.py for python_requires
	setupPyFile := filepath.Join(s.projectPath, "setup.py")
	if data, err := os.ReadFile(setupPyFile); err == nil {
		content := string(data)
		// Look for python_requires='>=3.8' or python_requires=">=3.8"
		re := regexp.MustCompile(`python_requires\s*=\s*['"]>=(\d+)\.(\d+)`)
		matches := re.FindStringSubmatch(content)
		if len(matches) == 3 {
			major, _ := strconv.Atoi(matches[1])
			minor, _ := strconv.Atoi(matches[2])
			return major, minor, nil
		}
	}

	// Check pyproject.toml for requires-python
	pyprojectFile := filepath.Join(s.projectPath, "pyproject.toml")
	if data, err := os.ReadFile(pyprojectFile); err == nil {
		content := string(data)
		// Look for requires-python = ">=3.8" or requires-python = '>=3.8'
		re := regexp.MustCompile(`requires-python\s*=\s*['"]>=(\d+)\.(\d+)`)
		matches := re.FindStringSubmatch(content)
		if len(matches) == 3 {
			major, _ := strconv.Atoi(matches[1])
			minor, _ := strconv.Atoi(matches[2])
			return major, minor, nil
		}
	}

	// No specific version requirement found, assume Python 3.7+ (AWS CDK minimum)
	return 3, 7, nil
}

// checkPythonCompatibility verifies that the Python version meets project requirements
func (s *Synthesizer) checkPythonCompatibility(pythonCmd string) error {
	// Get installed Python version
	major, minor, patch, err := getPythonVersion(pythonCmd)
	if err != nil {
		return err
	}

	installedVersion := fmt.Sprintf("%d.%d.%d", major, minor, patch)
	fmt.Printf("Detected Python version: %s\n", installedVersion)

	// Get required Python version
	reqMajor, reqMinor, err := s.getRequiredPythonVersion()
	if err != nil {
		return err
	}

	requiredVersion := fmt.Sprintf("%d.%d", reqMajor, reqMinor)
	fmt.Printf("Required Python version: >=%s\n", requiredVersion)

	// Check compatibility
	if major < reqMajor || (major == reqMajor && minor < reqMinor) {
		return fmt.Errorf("python version %s is incompatible with project requirements (>=%s)", installedVersion, requiredVersion)
	}

	fmt.Printf("Python version %s is compatible\n", installedVersion)
	return nil
}

// Synth synthesizes the CDK app and returns the CloudFormation templates
func (s *Synthesizer) Synth() (*SynthResult, error) {
	// Read cdk.json to get the app command
	cdkConfig, err := s.readCDKConfig()
	if err != nil {
		return nil, err
	}

	fmt.Printf("CDK app command: %s\n", cdkConfig.App)

	// Run the CDK app to generate CloudFormation templates
	// The app command outputs to cdk.out by default
	if err := s.runCDKSynth(cdkConfig.App); err != nil {
		return nil, err
	}

	// Find all generated stack templates
	stacks, err := s.findGeneratedStacks()
	if err != nil {
		return nil, err
	}

	return &SynthResult{
		TemplateDir: s.outputDir,
		Stacks:      stacks,
	}, nil
}

// readCDKConfig reads and parses the cdk.json file
func (s *Synthesizer) readCDKConfig() (*CDKConfig, error) {
	configPath := filepath.Join(s.projectPath, "cdk.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cdk.json: %w", err)
	}

	var config CDKConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse cdk.json: %w", err)
	}

	return &config, nil
}

// runCDKSynth runs the CDK synthesis process
func (s *Synthesizer) runCDKSynth(appCmd string) error {
	fmt.Println("Synthesizing CDK app...")

	// Parse the app command
	parts := strings.Fields(appCmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty app command in cdk.json")
	}

	// Set CDK_OUTDIR environment variable
	env := os.Environ()
	env = append(env, fmt.Sprintf("CDK_OUTDIR=%s", s.outputDir))

	projectType, _ := s.DetectProjectType()

	// For TypeScript projects, we need to compile first if using ts-node isn't in the command
	if projectType == "typescript" && !strings.Contains(appCmd, "ts-node") {
		// Try to compile TypeScript first
		if _, err := os.Stat(filepath.Join(s.projectPath, "tsconfig.json")); err == nil {
			fmt.Println("Compiling TypeScript...")
			compileCmd := exec.Command("npx", "tsc")
			compileCmd.Dir = s.projectPath
			compileCmd.Stdout = os.Stdout
			compileCmd.Stderr = os.Stderr
			// Ignore compile errors as the project might use ts-node
			_ = compileCmd.Run()
		}
	}

	// For Python projects, use the virtual environment's Python
	if projectType == "python" {
		venvPython := filepath.Join(s.projectPath, ".venv", "bin", "python")
		// Replace "python" or "python3" with the venv python path in the app command
		if strings.HasPrefix(appCmd, "python3 ") {
			appCmd = venvPython + appCmd[7:]
		} else if strings.HasPrefix(appCmd, "python ") {
			appCmd = venvPython + appCmd[6:]
		}
		// Add venv bin to PATH
		venvBin := filepath.Join(s.projectPath, ".venv", "bin")
		env = append(env, fmt.Sprintf("PATH=%s:%s", venvBin, os.Getenv("PATH")))
		env = append(env, fmt.Sprintf("VIRTUAL_ENV=%s", filepath.Join(s.projectPath, ".venv")))
	}

	// Run cdk synth using npx cdk
	cmd := exec.Command("npx", "cdk", "synth", "--app", appCmd, "--output", s.outputDir)
	cmd.Dir = s.projectPath
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("CDK synthesis failed: %w", err)
	}

	return nil
}

// findGeneratedStacks finds all generated CloudFormation stack templates
func (s *Synthesizer) findGeneratedStacks() ([]string, error) {
	var stacks []string

	// Look for .template.json files in cdk.out
	pattern := filepath.Join(s.outputDir, "*.template.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find templates: %w", err)
	}

	for _, match := range matches {
		stackName := filepath.Base(match)
		stackName = strings.TrimSuffix(stackName, ".template.json")
		stacks = append(stacks, stackName)
	}

	if len(stacks) == 0 {
		return nil, fmt.Errorf("no CloudFormation templates found in %s", s.outputDir)
	}

	return stacks, nil
}

// GetTemplateBody returns the CloudFormation template body for a stack
func (s *Synthesizer) GetTemplateBody(stackName string) (string, error) {
	templatePath := filepath.Join(s.outputDir, stackName+".template.json")
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template for stack %s: %w", stackName, err)
	}
	return string(data), nil
}
