package generator

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/go-i2p/go-gh-page/pkg/git"
	"github.com/go-i2p/go-gh-page/pkg/templates"
	"github.com/go-i2p/go-gh-page/pkg/utils"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// GenerationResult contains information about the generated site
type GenerationResult struct {
	DocsCount     int
	ImagesCount   int
	SiteStructure string
}

// Generator handles the site generation
type Generator struct {
	repoData      *git.RepositoryData
	outputDir     string
	templateCache map[string]*template.Template
}

// PageData contains the data passed to HTML templates
type PageData struct {
	RepoOwner    string
	RepoName     string
	RepoFullName string
	Description  string
	CommitCount  int
	LastUpdate   string
	License      string
	RepoURL      string

	ReadmeHTML   string
	Contributors []git.Contributor

	// Navigation
	DocsPages []utils.DocPage

	// Current page info
	CurrentPage string
	PageTitle   string
	PageContent string

	// Generation info
	GeneratedAt string
}

// NewGenerator creates a new site generator
func NewGenerator(repoData *git.RepositoryData, outputDir string) *Generator {
	return &Generator{
		repoData:      repoData,
		outputDir:     outputDir,
		templateCache: make(map[string]*template.Template),
	}
}

// GenerateSite generates the complete static site
func (g *Generator) GenerateSite() (*GenerationResult, error) {
	result := &GenerationResult{}

	// Create docs directory
	docsDir := filepath.Join(g.outputDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create docs directory: %w", err)
	}

	// Write style.css to the output directory
	if err := GenerateRootStyle(g.outputDir); err != nil {
		return nil, fmt.Errorf("failed to write style.css: %w", err)
	}

	// Create image directory if needed
	imagesDir := filepath.Join(g.outputDir, "images")
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create images directory: %w", err)
	}

	// Parse all templates first
	if err := g.parseTemplates(); err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	// Copy image files to output directory
	for relativePath, sourcePath := range g.repoData.ImageFiles {
		destPath := filepath.Join(g.outputDir, "images", filepath.Base(relativePath))
		if err := copyFile(sourcePath, destPath); err != nil {
			return nil, fmt.Errorf("failed to copy image %s: %w", relativePath, err)
		}
		result.ImagesCount++
	}

	// Prepare the list of documentation pages for navigation
	var docsPages []utils.DocPage
	for path := range g.repoData.MarkdownFiles {
		// Skip README as it's on the main page
		if isReadmeFile(filepath.Base(path)) {
			continue
		}

		title := utils.GetTitleFromMarkdown(g.repoData.MarkdownFiles[path])
		if title == "" {
			title = utils.PrettifyFilename(filepath.Base(path))
		}

		outputPath := utils.GetOutputPath(path, "docs")
		docsPages = append(docsPages, utils.DocPage{
			Title: title,
			Path:  outputPath,
		})
	}

	// Sort docsPages by title for consistent navigation
	utils.SortDocPagesByTitle(docsPages)

	// Generate main index page
	if err := g.generateMainPage(docsPages); err != nil {
		return nil, fmt.Errorf("failed to generate main page: %w", err)
	}

	// Generate documentation pages
	for path, content := range g.repoData.MarkdownFiles {
		// Skip README as it's on the main page
		if isReadmeFile(filepath.Base(path)) {
			continue
		}

		if err := g.generateDocPage(path, content, docsPages); err != nil {
			return nil, fmt.Errorf("failed to generate doc page for %s: %w", path, err)
		}

		result.DocsCount++
	}

	// Generate site structure summary
	var buffer bytes.Buffer
	buffer.WriteString(g.outputDir + "/\n")
	buffer.WriteString("  ├── index.html\n")
	buffer.WriteString("  ├── docs/\n")

	if len(docsPages) > 0 {
		for i, page := range docsPages {
			prefix := "  │   ├── "
			if i == len(docsPages)-1 {
				prefix = "  │   └── "
			}
			buffer.WriteString(prefix + filepath.Base(page.Path) + "\n")
		}
	} else {
		buffer.WriteString("  │   └── (empty)\n")
	}

	if result.ImagesCount > 0 {
		buffer.WriteString("  └── images/\n")
		buffer.WriteString("      └── ... (" + fmt.Sprintf("%d", result.ImagesCount) + " files)\n")
	} else {
		buffer.WriteString("  └── images/\n")
		buffer.WriteString("      └── (empty)\n")
	}

	result.SiteStructure = buffer.String()

	return result, nil
}

// parseTemplates parses all the HTML templates
func (g *Generator) parseTemplates() error {
	// Parse main template
	mainTmpl, err := template.New("main").Parse(templates.MainTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse main template: %w", err)
	}
	g.templateCache["main"] = mainTmpl

	// Parse documentation template
	docTmpl, err := template.New("doc").Parse(templates.DocTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse doc template: %w", err)
	}
	g.templateCache["doc"] = docTmpl

	return nil
}

// generateMainPage creates the main index.html
func (g *Generator) generateMainPage(docsPages []utils.DocPage) error {
	// Prepare data for template
	data := PageData{
		RepoOwner:    g.repoData.Owner,
		RepoName:     g.repoData.Name,
		RepoFullName: g.repoData.Owner + "/" + g.repoData.Name,
		Description:  g.repoData.Description,
		CommitCount:  g.repoData.CommitCount,
		License:      g.repoData.License,
		RepoURL:      g.repoData.URL,
		LastUpdate:   g.repoData.LastCommitDate.Format("January 2, 2006"),

		ReadmeHTML:   renderMarkdown(g.repoData.ReadmeContent),
		Contributors: g.repoData.Contributors,

		DocsPages:   docsPages,
		CurrentPage: "index.html",
		PageTitle:   g.repoData.Owner + "/" + g.repoData.Name,

		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Render template
	var buf bytes.Buffer
	if err := g.templateCache["main"].Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute main template: %w", err)
	}

	// Write to file
	outputPath := filepath.Join(g.outputDir, "index.html")
	if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("failed to write index.html: %w", err)
	}

	return nil
}

// generateDocPage creates an HTML page for a markdown file
func (g *Generator) generateDocPage(path, content string, docsPages []utils.DocPage) error {
	// Get the title from the markdown content
	title := utils.GetTitleFromMarkdown(content)
	if title == "" {
		title = utils.PrettifyFilename(filepath.Base(path))
	}

	// Process relative links in the markdown
	processedContent := utils.ProcessRelativeLinks(content, path, g.repoData.Owner, g.repoData.Name)

	// Process image links to point to our local images
	processedContent = processImageLinks(processedContent, path)

	// Render markdown to HTML
	contentHTML := renderMarkdown(processedContent)

	// Create a copy of docsPages with current page marked as active
	currentDocsPages := make([]utils.DocPage, len(docsPages))
	copy(currentDocsPages, docsPages)
	outputPath := utils.GetOutputPath(path, "docs")

	for i := range currentDocsPages {
		if currentDocsPages[i].Path == outputPath {
			currentDocsPages[i].IsActive = true
		}
	}

	// Prepare data for template
	data := PageData{
		RepoOwner:    g.repoData.Owner,
		RepoName:     g.repoData.Name,
		RepoFullName: g.repoData.Owner + "/" + g.repoData.Name,
		Description:  g.repoData.Description,
		CommitCount:  g.repoData.CommitCount,
		License:      g.repoData.License,
		RepoURL:      g.repoData.URL,
		LastUpdate:   g.repoData.LastCommitDate.Format("January 2, 2006"),

		DocsPages:   currentDocsPages,
		CurrentPage: outputPath,
		PageTitle:   title + " - " + g.repoData.Owner + "/" + g.repoData.Name,
		PageContent: contentHTML,

		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Render template
	var buf bytes.Buffer
	if err := g.templateCache["doc"].Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute doc template: %w", err)
	}

	// Ensure output directory exists
	outPath := filepath.Join(g.outputDir, outputPath)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", outPath, err)
	}

	// Write to file
	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", outPath, err)
	}

	return nil
}

// isReadmeFile checks if a file is a README
func isReadmeFile(filename string) bool {
	lowerFilename := strings.ToLower(filename)
	return strings.HasPrefix(lowerFilename, "readme.")
}

// renderMarkdown converts markdown content to HTML
func renderMarkdown(md string) string {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse([]byte(md))

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return string(markdown.Render(doc, renderer))
}

// processImageLinks updates image links to point to our local images
func processImageLinks(content, filePath string) string {
	// Replace image links with links to our local images directory
	re := utils.GetImageLinkRegex()

	baseDir := filepath.Dir(filePath)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		submatch := re.FindStringSubmatch(match)
		if len(submatch) < 3 {
			return match
		}

		altText := submatch[1]
		imagePath := submatch[2]

		// Skip absolute URLs
		if strings.HasPrefix(imagePath, "http") {
			return match
		}

		// Make the path relative to the root
		if !strings.HasPrefix(imagePath, "/") {
			// Handle ./image.jpg style paths
			if strings.HasPrefix(imagePath, "./") {
				imagePath = imagePath[2:]
			}

			// If in a subdirectory, make path relative to root
			if baseDir != "." {
				imagePath = filepath.Join(baseDir, imagePath)
			}
		} else {
			// Remove leading slash if any
			imagePath = strings.TrimPrefix(imagePath, "/")
		}

		// Create a path to our local images directory
		localPath := "../images/" + filepath.Base(imagePath)

		return fmt.Sprintf("![%s](%s)", altText, localPath)
	})
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Open source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create destination file
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy the contents
	_, err = io.Copy(destFile, sourceFile)
	return err
}

func GenerateRootStyle(outputDir string) error {
	// write the templates.StyleTemplate to the root of the output directory
	stylePath := filepath.Join(outputDir, "style.css")
	if err := os.WriteFile(stylePath, []byte(templates.StyleTemplate), 0o644); err != nil {
		return fmt.Errorf("failed to write style.css: %w", err)
	}
	return nil
}
