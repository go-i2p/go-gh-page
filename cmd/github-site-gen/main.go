package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-i2p/go-gh-page/pkg/generator"
	"github.com/go-i2p/go-gh-page/pkg/git"
	"github.com/go-i2p/go-gh-page/pkg/templates"
	github "github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

func main() {
	// Define command-line flags
	repoFlag := flag.String("repo", "", "GitHub repository in format 'owner/repo-name'")
	outputFlag := flag.String("output", "./output", "Output directory for generated site")
	branchFlag := flag.String("branch", "main", "Branch to use (default: main)")
	workDirFlag := flag.String("workdir", "", "Working directory for cloning (default: temporary directory)")
	githost := flag.String("githost", "github.com", "Git host (default: github.com)")
	mainTemplateOverride := flag.String("main-template", "", "Path to custom main template")
	docTemplateOverride := flag.String("doc-template", "", "Path to custom documentation template")
	styleTemplateOverride := flag.String("style-template", "", "Path to custom style template")
	setupYaml := flag.Bool("page-yaml", false, "Generate .github/workflows/page.yaml file")
	setupPage := flag.Bool("setup-page", false, "Setup GitHub Pages to build from gh-pages branch")

	flag.Parse()

	if *setupYaml {
		if err := os.MkdirAll(".github/workflows", 0o755); err != nil {
			log.Fatalf("Failed to create .github/workflows directory: %v", err)
		}
		// Generate the page.yaml file
		if err := os.WriteFile(".github/workflows/page.yml", []byte(templates.CITemplate), 0o644); err != nil {
			log.Fatalf("Failed to generate page.yml: %v", err)
		}
		fmt.Printf("Generated .github/workflows/page.yaml in %s\n", *outputFlag)
		if err := exec.Command("git", "add", ".github/workflows/page.yml").Run(); err != nil {
			log.Fatalf("Failed to add page.yml to git: %v", err)
		}
		if err := exec.Command("git", "commit", "-m", "Add GitHub Actions workflow for page generation").Run(); err != nil {
			log.Fatalf("Failed to commit page.yml: %v", err)
		}
		if err := exec.Command("git", "push").Run(); err != nil {
			log.Fatalf("Failed to push page.yml: %v", err)
		}
		fmt.Println("Added .github/workflows/page.yml to git staging area.")
		fmt.Println("You can now commit and push this file to your repository.")
		os.Exit(0)
	}

	// Validate repository flag
	if *repoFlag == "" {
		fmt.Println("Error: -repo flag is required (format: owner/repo-name)")
		flag.Usage()
		os.Exit(1)
	}

	repoParts := strings.Split(*repoFlag, "/")
	if len(repoParts) != 2 {
		fmt.Println("Error: -repo flag must be in format 'owner/repo-name'")
		flag.Usage()
		os.Exit(1)
	}
	if *setupPage {
		if err := enableGithubPage(repoParts[0], repoParts[1]); err != nil {
			log.Fatalf("Failed to enable GitHub Pages: %v", err)
		}
		fmt.Printf("Enabled GitHub Pages for %s/%s\n", strings.Split(*repoFlag, "/")[0], strings.Split(*repoFlag, "/")[1])
		os.Exit(0)
	}
	// if mainTemplateOverride is not empty, check if a file exists
	if *mainTemplateOverride != "" {
		if _, err := os.Stat(*mainTemplateOverride); os.IsNotExist(err) {
			fmt.Printf("Error: main template file %s does not exist\n", *mainTemplateOverride)
			os.Exit(1)
		} else {
			fmt.Printf("Using custom main template: %s\n", *mainTemplateOverride)
			// read the file in and override templates.MainTemplate
			data, err := os.ReadFile(*mainTemplateOverride)
			if err != nil {
				fmt.Printf("Error: failed to read main template file %s: %v\n", *mainTemplateOverride, err)
				os.Exit(1)
			}
			templates.MainTemplate = string(data)
		}
	}
	// if docTemplateOverride is not empty, check if a file exists
	if *docTemplateOverride != "" {
		if _, err := os.Stat(*docTemplateOverride); os.IsNotExist(err) {
			fmt.Printf("Error: doc template file %s does not exist\n", *docTemplateOverride)
			os.Exit(1)
		} else {
			fmt.Printf("Using custom docs template: %s\n", *docTemplateOverride)
			// read the file in and override templates.MainTemplate
			data, err := os.ReadFile(*docTemplateOverride)
			if err != nil {
				fmt.Printf("Error: failed to read docs template file %s: %v\n", *docTemplateOverride, err)
				os.Exit(1)
			}
			templates.DocTemplate = string(data)
		}
	}
	// if styleTemplateOverride is not empty, check if a file exists
	if *styleTemplateOverride != "" {
		if _, err := os.Stat(*styleTemplateOverride); os.IsNotExist(err) {
			fmt.Printf("Error: style template file %s does not exist\n", *styleTemplateOverride)
			os.Exit(1)
		} else {
			fmt.Printf("Using custom style template: %s\n", *styleTemplateOverride)
			// read the file in and override templates.MainTemplate
			data, err := os.ReadFile(*styleTemplateOverride)
			if err != nil {
				fmt.Printf("Error: failed to read style template file %s: %v\n", *styleTemplateOverride, err)
				os.Exit(1)
			}
			templates.StyleTemplate = string(data)
		}
	}

	owner, repo := repoParts[0], repoParts[1]
	repoURL := fmt.Sprintf("https://%s/%s/%s.git", *githost, owner, repo)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputFlag, 0o755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Determine working directory
	workDir := *workDirFlag
	if workDir == "" {
		// Create temporary directory
		tempDir, err := os.MkdirTemp("", "github-site-gen-*")
		if err != nil {
			log.Fatalf("Failed to create temporary directory: %v", err)
		}
		workDir = tempDir
		defer os.RemoveAll(tempDir) // Clean up when done
	} else {
		// Ensure the specified work directory exists
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			log.Fatalf("Failed to create working directory: %v", err)
		}
	}

	cloneDir := filepath.Join(workDir, repo)

	// Clone the repository
	fmt.Printf("Cloning %s/%s into %s...\n", owner, repo, cloneDir)
	startTime := time.Now()
	gitRepo, err := git.CloneRepository(repoURL, cloneDir, *branchFlag)
	if err != nil {
		log.Fatalf("Failed to clone repository: %v", err)
	}
	fmt.Printf("Repository cloned in %.2f seconds\n", time.Since(startTime).Seconds())

	// Get repository data
	repoData, err := git.GetRepositoryData(gitRepo, owner, repo, cloneDir)
	if err != nil {
		log.Fatalf("Failed to gather repository data: %v", err)
	}

	// Create generator
	gen := generator.NewGenerator(repoData, *outputFlag)

	// Generate site
	fmt.Println("Generating static site...")
	startGenTime := time.Now()
	result, err := gen.GenerateSite()
	if err != nil {
		log.Fatalf("Failed to generate site: %v", err)
	}

	// Print summary
	fmt.Printf("\nRepository site for %s/%s successfully generated in %.2f seconds:\n",
		owner, repo, time.Since(startGenTime).Seconds())
	fmt.Printf("- Main page: %s\n", filepath.Join(*outputFlag, "index.html"))
	fmt.Printf("- Documentation pages: %d markdown files converted\n", result.DocsCount)

	if result.ImagesCount > 0 {
		fmt.Printf("- Images directory: %s/images/\n", *outputFlag)
	}

	fmt.Printf("\nSite structure:\n%s\n", result.SiteStructure)
	fmt.Printf("\nYou can open index.html directly in your browser\n")
	fmt.Printf("or deploy the entire directory to any static web host.\n")

	fmt.Printf("\nTotal time: %.2f seconds\n", time.Since(startTime).Seconds())
}

func enableGithubPage(userName, repoName string) error {
	branch := "gh-pages"
	token := os.Getenv("GITHUB_TOKEN")
	if len(token) == 0 {
		return fmt.Errorf("GITHUB_TOKEN not set")
	}
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	path := "/"
	_, _, err := client.Repositories.EnablePages(ctx, userName, repoName, &github.Pages{
		Source: &github.PagesSource{
			Branch: github.String(branch),
			Path:   github.String(path),
		},
		Public: github.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("could not enable github pages: %v", err)
	}

	return nil
}
