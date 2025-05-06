package utils

import (
	"path/filepath"
	"regexp"
	"strings"
)

// GetOutputPath converts a markdown file path to its HTML output path
func GetOutputPath(path, baseDir string) string {
	// Replace extension with .html
	baseName := filepath.Base(path)
	dir := filepath.Dir(path)

	// Remove extension
	baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName)) + ".html"

	// If it's in root, put it directly in baseDir
	if dir == "." {
		return filepath.Join(baseDir, baseName)
	}

	// Otherwise preserve directory structure
	return filepath.Join(baseDir, dir, baseName)
}

// GetTitleFromMarkdown extracts the first heading from markdown content
func GetTitleFromMarkdown(content string) string {
	// Look for the first heading
	re := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// PrettifyFilename converts a filename to a more readable title
func PrettifyFilename(filename string) string {
	// Remove extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Replace hyphens and underscores with spaces
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Capitalize words
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[0:1]) + word[1:]
		}
	}

	return strings.Join(words, " ")
}

// ProcessRelativeLinks handles relative links in markdown content
func ProcessRelativeLinks(content, filePath, owner, repo string) string {
	baseDir := filepath.Dir(filePath)

	// Replace relative links to markdown files with links to their HTML versions
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	return re.ReplaceAllStringFunc(content, func(match string) string {
		submatch := re.FindStringSubmatch(match)
		if len(submatch) < 3 {
			return match
		}

		linkText := submatch[1]
		linkTarget := submatch[2]

		// Skip absolute URLs and anchors
		if strings.HasPrefix(linkTarget, "http") || strings.HasPrefix(linkTarget, "#") {
			return match
		}

		// Skip image links (we'll handle these separately)
		if isImageLink(linkTarget) {
			return match
		}

		// Handle markdown links - convert to HTML links
		if isMarkdownLink(linkTarget) {
			// Remove anchor if present
			anchor := ""
			if idx := strings.Index(linkTarget, "#"); idx > -1 {
				anchor = linkTarget[idx:]
				linkTarget = linkTarget[:idx]
			}

			// If the link is relative, resolve it
			resolvedPath := linkTarget
			if !strings.HasPrefix(resolvedPath, "/") {
				// Handle ./file.md style links
				if strings.HasPrefix(resolvedPath, "./") {
					resolvedPath = resolvedPath[2:]
				}

				if baseDir != "." {
					resolvedPath = filepath.Join(baseDir, resolvedPath)
				}
			} else {
				// Remove leading slash
				resolvedPath = resolvedPath[1:]
			}

			htmlPath := "../" + GetOutputPath(resolvedPath, "docs")
			return "[" + linkText + "](" + htmlPath + anchor + ")"
		}

		return match
	})
}

// GetImageLinkRegex returns a regex for matching image links in markdown
func GetImageLinkRegex() *regexp.Regexp {
	return regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
}

// isImageLink checks if a link points to an image
func isImageLink(link string) bool {
	extensions := []string{".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp"}
	lower := strings.ToLower(link)

	for _, ext := range extensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}

	return false
}

// isMarkdownLink checks if a link points to a markdown file
func isMarkdownLink(link string) bool {
	extensions := []string{".md", ".markdown", ".mdown", ".mkdn"}
	lower := strings.ToLower(link)

	for _, ext := range extensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}

	return false
}

// SortDocPagesByTitle sorts doc pages by title
func SortDocPagesByTitle(pages []DocPage) {
	// Simple bubble sort
	for i := 0; i < len(pages); i++ {
		for j := i + 1; j < len(pages); j++ {
			if pages[i].Title > pages[j].Title {
				pages[i], pages[j] = pages[j], pages[i]
			}
		}
	}
}

// DocPage represents a documentation page for navigation
type DocPage struct {
	Title    string
	Path     string
	IsActive bool
}
