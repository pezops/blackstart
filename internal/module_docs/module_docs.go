package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/pezops/blackstart"
	_ "github.com/pezops/blackstart/internal/all_modules"
)

const moduleTemplate = `---
title: {{ .Id }}
---

# {{ .Id }}

{{ .Description }}

## Inputs
{{- if .Inputs }}

| Id | Description | Type | Required |
|------|-------------|------|----------|
{{- range $name, $input := .Inputs }}
| {{ $name }} | {{ $input.Description }}{{ if and (not $input.Required) (ne $input.Default nil) }}<br>Default: **{{ $input.Default }}**{{ end }} | {{ $input.Type }} | {{ $input.Required }} |
{{- end }}
{{- else }}

No inputs are supported for this module
{{- end }}

## Outputs
{{- if .Outputs }}

| Id | Description | Type |
|------|-------------|------|
{{- range $name, $output := .Outputs }}
| {{ $name }} | {{ $output.Description }} | {{ $output.Type }} |
{{- end }}
{{- else }}

No outputs are supported for this module
{{- end }}

## Examples
{{- if .Examples }}
{{- range $title, $example := .Examples }}

### {{ $title }}
` + "```" + `yaml
{{ $example }}
` + "```" + `
{{- end }}
{{- else }}

No examples are available for this module.
{{- end }}
`

func main() {
	generateDocs()
}

func generateDocs() {
	caser := cases.Title(language.English)
	modules := blackstart.GetRegisteredModules()
	pathNames := blackstart.GetRegisteredPathNames()

	baseDocsDir := "docs/user-guide/modules"
	modulesDir := "modules"

	generatedFiles := make(map[string]blackstart.ModuleInfo)

	for _, factory := range modules {
		module := factory()
		info := module.Info()

		// Create a path from the module name
		parts := strings.Split(info.Id, "_")
		var docPath, fileName string
		for i := 1; i <= len(parts); i++ {
			p := filepath.Join(parts[:i]...)
			if _, err := os.Stat(filepath.Join(modulesDir, p)); err == nil {
				continue
			}

			docPath = filepath.Join(parts[:i-1]...)
			fileName = strings.Join(parts[i-1:], "_")
			break
		}

		if fileName == "" {
			docPath = filepath.Join(parts...)
			fileName = parts[len(parts)-1]
		}

		// Replace path parts with friendly names if they exist
		pathParts := strings.Split(docPath, string(filepath.Separator))
		for i, part := range pathParts {
			if friendlyName, ok := pathNames[part]; ok {
				pathParts[i] = friendlyName
			} else {
				pathParts[i] = caser.String(part)
			}
		}
		docPath = filepath.Join(pathParts...)

		fullDocPath := filepath.Join(baseDocsDir, docPath)

		// Create the directory for the module documentation
		if err := os.MkdirAll(fullDocPath, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", fullDocPath, err)
		}

		// Prepare the template
		t, err := template.New("doc").Parse(moduleTemplate)
		if err != nil {
			log.Fatalf("Failed to parse template: %v", err)
		}

		// Execute the template with the module information
		var buf bytes.Buffer
		err = t.Execute(&buf, info)
		if err != nil {
			log.Fatalf("Failed to execute template: %v", err)
		}

		// Write the generated documentation to a file
		filePath := filepath.Join(fullDocPath, fmt.Sprintf("%s.md", fileName))
		err = os.WriteFile(filePath, buf.Bytes(), 0644)
		if err != nil {
			log.Fatalf("Failed to write output to file %s: %v", filePath, err)
		}
		fmt.Printf("Generated documentation for module %s at %s\n", info.Id, filePath)
		generatedFiles[filePath] = info
	}

	generateTOCs(baseDocsDir, generatedFiles, pathNames)
}

func generateTOCs(baseDir string, files map[string]blackstart.ModuleInfo, pathNames map[string]string) {
	dirMap := make(map[string][]string)
	allDirs := make(map[string]bool)

	for f := range files {
		dir := filepath.Dir(f)
		dirMap[dir] = append(dirMap[dir], f)
		allDirs[dir] = true

		// Add all parent directories to the set
		p := dir
		for {
			p = filepath.Dir(p)
			if p == "." || p == baseDir || p == filepath.Dir(baseDir) {
				break
			}
			allDirs[p] = true
		}
	}

	var dirs []string
	for d := range allDirs {
		dirs = append(dirs, d)
	}

	// Sort directories to process parents first
	sort.Strings(dirs)
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))

	for _, d := range dirs {
		if d == baseDir {
			continue
		}
		err := createReadme(d, baseDir, files, pathNames)
		if err != nil {
			log.Fatalf("failed to create readme for %s: %v", d, err)
		}
	}

	// Create README for base directory
	err := createReadme(baseDir, baseDir, files, pathNames)
	if err != nil {
		log.Fatalf("failed to create readme for %s: %v", baseDir, err)
	}
}

func createReadme(dir, baseDir string, allFiles map[string]blackstart.ModuleInfo, pathNames map[string]string) error {
	readmePath := filepath.Join(dir, "README.md")
	relDir, err := filepath.Rel(baseDir, dir)
	if err != nil {
		return err
	}

	title := "Modules"
	if relDir != "." {
		pathPart := filepath.Base(relDir)
		if friendlyName, ok := pathNames[strings.ToLower(pathPart)]; ok {
			title = friendlyName
		} else {
			title = cases.Title(language.English).String(pathPart)
		}
	}

	var content bytes.Buffer
	content.WriteString(fmt.Sprintf("# %s\n\n", title))

	// Direct module files in the current directory
	var directFiles []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for f := range allFiles {
		if filepath.Dir(f) == dir {
			directFiles = append(directFiles, f)
		}
	}
	sort.Strings(directFiles)

	if len(directFiles) > 0 {
		content.WriteString("## Modules\n\n")
		for _, f := range directFiles {
			info := allFiles[f]
			relPath, _ := filepath.Rel(dir, f)
			link := fmt.Sprintf("- [%s](./%s)\n", info.Id, relPath)
			content.WriteString(link)
		}
		content.WriteString("\n")
	}

	// Subdirectories
	var subDirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			subDirs = append(subDirs, entry.Name())
		}
	}
	sort.Strings(subDirs)

	if len(subDirs) > 0 {
		for _, subDir := range subDirs {
			// Check if a README.md exists in the subdirectory
			if _, err = os.Stat(filepath.Join(dir, subDir, "README.md")); err == nil {
				caser := cases.Title(language.English)
				title = caser.String(subDir)
				if friendlyName, ok := pathNames[strings.ToLower(subDir)]; ok {
					title = friendlyName
				}
				link := fmt.Sprintf("- [%s](./%s/)\n", title, subDir)
				content.WriteString(link)
			}
		}
	}

	return os.WriteFile(readmePath, content.Bytes(), 0644)
}
