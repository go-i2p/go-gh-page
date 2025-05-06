package git

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// RepositoryData contains all the information about a repository
type RepositoryData struct {
	Owner       string
	Name        string
	Description string
	URL         string

	// Content
	ReadmeContent string
	ReadmePath    string
	MarkdownFiles map[string]string // path -> content

	// Stats from git
	Contributors   []Contributor
	CommitCount    int
	LastCommitDate time.Time

	// License information if available
	License string

	// Set of image paths in the repository (to copy to output)
	ImageFiles map[string]string // path -> full path on disk
}

// Contributor represents a repository contributor
type Contributor struct {
	Name      string
	Email     string
	Commits   int
	AvatarURL string
}

// CloneRepository clones a Git repository to the specified directory
func CloneRepository(url, destination, branch string) (*git.Repository, error) {
	// Check if repository already exists
	if _, err := os.Stat(destination); err == nil {
		// Directory exists, try to open repository
		repo, err := git.PlainOpen(destination)
		if err == nil {
			fmt.Println("Using existing repository clone")
			return repo, nil
		}
		// If error, remove directory and clone fresh
		os.RemoveAll(destination)
	}

	// Clone options
	options := &git.CloneOptions{
		URL: url,
	}

	// Set branch if not default
	if branch != "main" && branch != "master" {
		options.ReferenceName = plumbing.NewBranchReferenceName(branch)
	}

	// Clone the repository
	return git.PlainClone(destination, false, options)
}

// GetRepositoryData extracts information from a cloned repository
func GetRepositoryData(repo *git.Repository, owner, name, repoPath string) (*RepositoryData, error) {
	repoData := &RepositoryData{
		Owner:         owner,
		Name:          name,
		URL:           fmt.Sprintf("https://github.com/%s/%s", owner, name),
		MarkdownFiles: make(map[string]string),
		ImageFiles:    make(map[string]string),
	}

	// Get the repository description from the repository
	config, err := repo.Config()
	if err == nil && config != nil {
		repoData.Description = config.Raw.Section("").Option("description")
	}

	// Get HEAD reference
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	// Get commit history
	cIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit history: %w", err)
	}

	// Process commits
	contributors := make(map[string]*Contributor)
	err = cIter.ForEach(func(c *object.Commit) error {
		// Count commits
		repoData.CommitCount++

		// Update last commit date if needed
		if repoData.LastCommitDate.IsZero() || c.Author.When.After(repoData.LastCommitDate) {
			repoData.LastCommitDate = c.Author.When
		}

		// Track contributors
		email := c.Author.Email
		if _, exists := contributors[email]; !exists {
			contributors[email] = &Contributor{
				Name:    c.Author.Name,
				Email:   email,
				Commits: 0,
				// GitHub avatar URL uses MD5 hash of email, which we'd generate here
				// but for simplicity we'll use a default avatar
				AvatarURL: fmt.Sprintf("https://avatars.githubusercontent.com/u/0?v=4"),
			}
		}
		contributors[email].Commits++

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to process commits: %w", err)
	}

	// Convert contributors map to slice and sort by commit count
	for _, contributor := range contributors {
		repoData.Contributors = append(repoData.Contributors, *contributor)
	}

	// Sort contributors by commit count (we'll implement this in utils)
	sortContributorsByCommits(repoData.Contributors)

	// If we have more than 5 contributors, limit to top 5
	if len(repoData.Contributors) > 5 {
		repoData.Contributors = repoData.Contributors[:5]
	}

	// Walk the repository to find markdown and image files
	err = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		// Skip other common directories we don't want
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "vendor" || d.Name() == ".github") {
			return filepath.SkipDir
		}

		// Process files
		if !d.IsDir() {
			relativePath, err := filepath.Rel(repoPath, path)
			if err != nil {
				return err
			}

			// Handle markdown files
			if isMarkdownFile(d.Name()) {
				content, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("failed to read file %s: %w", path, err)
				}

				// Store markdown content
				repoData.MarkdownFiles[relativePath] = string(content)

				// Check if this is a README file
				if isReadmeFile(d.Name()) && (repoData.ReadmePath == "" || relativePath == "README.md") {
					repoData.ReadmePath = relativePath
					repoData.ReadmeContent = string(content)
				}

				fmt.Printf("Found markdown file: %s\n", relativePath)
			}

			// Handle image files
			if isImageFile(d.Name()) {
				repoData.ImageFiles[relativePath] = path
				fmt.Printf("Found image file: %s\n", relativePath)
			}

			// Check for license file
			if isLicenseFile(d.Name()) && repoData.License == "" {
				content, err := os.ReadFile(path)
				if err == nil {
					// Try to determine license type from content
					licenseType := detectLicenseType(string(content))
					if licenseType != "" {
						repoData.License = licenseType
					} else {
						repoData.License = "License"
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk repository: %w", err)
	}

	// If we didn't find a description, try to extract from README
	if repoData.Description == "" && repoData.ReadmeContent != "" {
		repoData.Description = extractDescriptionFromReadme(repoData.ReadmeContent)
	}

	return repoData, nil
}

// isMarkdownFile checks if a filename has a markdown extension
func isMarkdownFile(filename string) bool {
	extensions := []string{".md", ".markdown", ".mdown", ".mkdn"}
	lowerFilename := strings.ToLower(filename)
	for _, ext := range extensions {
		if strings.HasSuffix(lowerFilename, ext) {
			return true
		}
	}
	return false
}

// isReadmeFile checks if a file is a README
func isReadmeFile(filename string) bool {
	lowerFilename := strings.ToLower(filename)
	return strings.HasPrefix(lowerFilename, "readme.") && isMarkdownFile(filename)
}

// isImageFile checks if a filename has an image extension
func isImageFile(filename string) bool {
	extensions := []string{".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp"}
	lowerFilename := strings.ToLower(filename)
	for _, ext := range extensions {
		if strings.HasSuffix(lowerFilename, ext) {
			return true
		}
	}
	return false
}

// isLicenseFile checks if a file is likely a license file
func isLicenseFile(filename string) bool {
	lowerFilename := strings.ToLower(filename)
	return lowerFilename == "license" || lowerFilename == "license.md" ||
		lowerFilename == "license.txt" || lowerFilename == "copying"
}

// detectLicenseType tries to determine the license type from its content
func detectLicenseType(content string) string {
	content = strings.ToLower(content)

	// Check for common license types
	if strings.Contains(content, "mit license") {
		return "MIT License"
	} else if strings.Contains(content, "apache license") {
		return "Apache License"
	} else if strings.Contains(content, "gnu general public license") ||
		strings.Contains(content, "gpl") {
		return "GPL License"
	} else if strings.Contains(content, "bsd") {
		return "BSD License"
	} else if strings.Contains(content, "mozilla public license") {
		return "Mozilla Public License"
	}

	return ""
}

// extractDescriptionFromReadme tries to get a short description from README
func extractDescriptionFromReadme(content string) string {
	// Try to find the first paragraph after the title
	re := regexp.MustCompile(`(?m)^#\s+.+\n+(.+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		// Return first paragraph, up to 150 chars
		desc := matches[1]
		if len(desc) > 150 {
			desc = desc[:147] + "..."
		}
		return desc
	}

	// If no match, just take the first non-empty line
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			if len(line) > 150 {
				line = line[:147] + "..."
			}
			return line
		}
	}

	return ""
}

// sortContributorsByCommits sorts contributors by commit count (descending)
func sortContributorsByCommits(contributors []Contributor) {
	// Simple bubble sort implementation
	for i := 0; i < len(contributors); i++ {
		for j := i + 1; j < len(contributors); j++ {
			if contributors[i].Commits < contributors[j].Commits {
				contributors[i], contributors[j] = contributors[j], contributors[i]
			}
		}
	}
}

// GetCommitStats gets commit statistics for the repository
func GetCommitStats(repo *git.Repository) (int, error) {
	// Get HEAD reference
	ref, err := repo.Head()
	if err != nil {
		return 0, fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	// Get commit history
	cIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return 0, fmt.Errorf("failed to get commit history: %w", err)
	}

	// Count commits
	count := 0
	err = cIter.ForEach(func(c *object.Commit) error {
		count++
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to process commits: %w", err)
	}

	return count, nil
}
