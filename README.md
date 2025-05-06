# go-gh-page

A tool that automatically generates GitHub Pages static websites from repository content. It converts your repository's Markdown files to HTML with a consistent layout, handles navigation, and preserves your documentation structure.

## Features

- Generates a complete static website from your repository's content
- Converts Markdown files to HTML with proper rendering
- Creates navigation structure based on your documentation
- Displays repository information (commits, contributors, license)
- Preserves images and handles relative links
- Supports custom templates and styles
- Includes GitHub Actions workflow for automatic deployment

## Installation

```bash
go install github.com/go-i2p/go-gh-page/cmd/github-site-gen@latest
```

## Usage

### Basic Usage

Generate a site for a GitHub repository:

```bash
github-site-gen -repo owner/repo-name -output ./site
```

### Command Line Options

| Flag | Description | Default |
|------|-------------|---------|
| `-repo` | GitHub repository in format 'owner/repo-name' | (Required) |
| `-output` | Output directory for generated site | `./output` |
| `-branch` | Branch to use | `main` |
| `-workdir` | Working directory for cloning | (Temporary directory) |
| `-githost` | Git host to use | `github.com` |
| `-main-template` | Path to custom main template | (Built-in template) |
| `-doc-template` | Path to custom documentation template | (Built-in template) |
| `-style-template` | Path to custom style template | (Built-in template) |

### Using with GitHub Actions

The repository includes a GitHub Actions workflow file that can automatically generate and deploy your documentation to GitHub Pages.

1. Copy the `.github/workflows/page.yml` file to your repository
2. Enable GitHub Pages on your repository (Settings → Pages → Source: gh-pages branch)
3. Customize the workflow as needed

The workflow runs hourly by default and on pushes to the main branch, automatically updating your GitHub Pages.

## Custom Templates

You can provide custom templates for different components of the generated site:

```bash
github-site-gen -repo owner/repo-name -output ./site \
  -main-template path/to/main.html \
  -doc-template path/to/doc.html \
  -style-template path/to/style.css
```

## License

MIT License