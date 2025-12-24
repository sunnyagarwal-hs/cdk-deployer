package git

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

// CloneRepository clones a public git repository to a local directory
func CloneRepository(repoURL, destDir string) (string, error) {
	// If destDir is empty, create a temp directory
	if destDir == "" {
		tmpDir, err := os.MkdirTemp("", "cdk-deployer-*")
		if err != nil {
			return "", fmt.Errorf("failed to create temp directory: %w", err)
		}
		destDir = tmpDir
	}

	// Extract repo name for subdirectory
	repoName := filepath.Base(repoURL)
	if filepath.Ext(repoName) == ".git" {
		repoName = repoName[:len(repoName)-4]
	}
	clonePath := filepath.Join(destDir, repoName)

	// Clone the repository
	fmt.Printf("Cloning %s to %s...\n", repoURL, clonePath)
	_, err := git.PlainClone(clonePath, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: os.Stdout,
		Depth:    1, // Shallow clone for faster operation
	})
	if err != nil {
		return "", fmt.Errorf("failed to clone repository: %w", err)
	}

	fmt.Println("Repository cloned successfully")
	return clonePath, nil
}

// CleanupRepository removes the cloned repository directory
func CleanupRepository(path string) error {
	return os.RemoveAll(path)
}
