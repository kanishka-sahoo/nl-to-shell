package plugins

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/nl-to-shell/nl-to-shell/internal/interfaces"
	"github.com/nl-to-shell/nl-to-shell/internal/types"
)

// ProjectPlugin identifies project types and gathers language-specific context
type ProjectPlugin struct{}

// NewProjectPlugin creates a new project type identification plugin
func NewProjectPlugin() interfaces.ContextPlugin {
	return &ProjectPlugin{}
}

// Name returns the plugin name
func (p *ProjectPlugin) Name() string {
	return "project"
}

// Priority returns the plugin priority
func (p *ProjectPlugin) Priority() int {
	return 80 // High priority as project type affects command generation
}

// ProjectType represents a detected project type
type ProjectType struct {
	Type       string                 `json:"type"`
	Language   string                 `json:"language"`
	Framework  string                 `json:"framework,omitempty"`
	Confidence float64                `json:"confidence"`
	Indicators []string               `json:"indicators"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// GatherContext identifies project types and gathers language-specific context
func (p *ProjectPlugin) GatherContext(ctx context.Context, baseContext *types.Context) (map[string]interface{}, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	projectTypes := p.detectProjectTypes(ctx)
	primaryType := p.determinePrimaryType(projectTypes)

	// Gather language-specific context
	languageContext := p.gatherLanguageContext(ctx, projectTypes)

	// Analyze project structure
	structure := p.analyzeProjectStructure(ctx)

	result := map[string]interface{}{
		"types":            projectTypes,
		"primary_type":     primaryType,
		"language_context": languageContext,
		"structure":        structure,
	}

	return result, nil
}

// detectProjectTypes detects all possible project types in the current directory
func (p *ProjectPlugin) detectProjectTypes(ctx context.Context) []ProjectType {
	var projectTypes []ProjectType

	// Define project type detectors
	detectors := []func(context.Context) *ProjectType{
		p.detectWebProject,
		p.detectMobileProject,
		p.detectDataScienceProject,
		p.detectDesktopProject,
		p.detectLibraryProject,
		p.detectCLIProject,
		p.detectMicroserviceProject,
		p.detectGameProject,
		p.detectDocumentationProject,
		p.detectInfrastructureProject,
	}

	for _, detector := range detectors {
		select {
		case <-ctx.Done():
			return projectTypes
		default:
		}

		if projectType := detector(ctx); projectType != nil {
			projectTypes = append(projectTypes, *projectType)
		}
	}

	return projectTypes
}

// detectWebProject detects web application projects
func (p *ProjectPlugin) detectWebProject(ctx context.Context) *ProjectType {
	indicators := []string{}
	confidence := 0.0
	framework := ""
	metadata := make(map[string]interface{})

	// Check for web-specific files
	webFiles := map[string]float64{
		"package.json":      0.3,
		"index.html":        0.2,
		"webpack.config.js": 0.3,
		"vite.config.js":    0.3,
		"next.config.js":    0.4,
		"nuxt.config.js":    0.4,
		"angular.json":      0.4,
		"vue.config.js":     0.3,
		"svelte.config.js":  0.3,
		"gatsby-config.js":  0.4,
		"remix.config.js":   0.4,
	}

	for file, weight := range webFiles {
		if _, err := os.Stat(file); err == nil {
			indicators = append(indicators, file)
			confidence += weight

			// Detect specific frameworks
			switch file {
			case "next.config.js":
				framework = "Next.js"
			case "nuxt.config.js":
				framework = "Nuxt.js"
			case "angular.json":
				framework = "Angular"
			case "vue.config.js":
				framework = "Vue.js"
			case "svelte.config.js":
				framework = "Svelte"
			case "gatsby-config.js":
				framework = "Gatsby"
			case "remix.config.js":
				framework = "Remix"
			}
		}
	}

	// Check package.json for web dependencies
	if packageInfo := p.analyzePackageJSON(); packageInfo != nil {
		if deps, ok := packageInfo["dependencies"].(map[string]interface{}); ok {
			webDeps := map[string]string{
				"react":   "React",
				"vue":     "Vue.js",
				"angular": "Angular",
				"svelte":  "Svelte",
				"next":    "Next.js",
				"nuxt":    "Nuxt.js",
				"gatsby":  "Gatsby",
				"express": "Express.js",
				"fastify": "Fastify",
				"koa":     "Koa",
				"nestjs":  "NestJS",
			}

			for dep, fw := range webDeps {
				if _, exists := deps[dep]; exists {
					confidence += 0.2
					if framework == "" {
						framework = fw
					}
					metadata["has_"+dep] = true
				}
			}
		}

		// Check for web-specific scripts
		if scripts, ok := packageInfo["scripts"].(map[string]interface{}); ok {
			webScripts := []string{"start", "build", "dev", "serve"}
			for _, script := range webScripts {
				if _, exists := scripts[script]; exists {
					confidence += 0.1
				}
			}
		}
	}

	// Check for web directories
	webDirs := []string{"public", "static", "assets", "src/components", "src/pages"}
	for _, dir := range webDirs {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			indicators = append(indicators, dir+"/")
			confidence += 0.1
		}
	}

	if confidence > 0.3 {
		return &ProjectType{
			Type:       "web",
			Language:   p.detectPrimaryLanguage(),
			Framework:  framework,
			Confidence: confidence,
			Indicators: indicators,
			Metadata:   metadata,
		}
	}

	return nil
}

// detectMobileProject detects mobile application projects
func (p *ProjectPlugin) detectMobileProject(ctx context.Context) *ProjectType {
	indicators := []string{}
	confidence := 0.0
	framework := ""
	metadata := make(map[string]interface{})

	// Check for mobile-specific files
	mobileFiles := map[string]float64{
		"android/":               0.5,
		"ios/":                   0.5,
		"pubspec.yaml":           0.6, // Flutter
		"flutter.yaml":           0.6, // Flutter
		"react-native.config.js": 0.5, // React Native
		"metro.config.js":        0.4, // React Native
		"app.json":               0.3, // Expo/React Native
		"expo.json":              0.5, // Expo
		"capacitor.config.json":  0.5, // Capacitor
		"cordova.xml":            0.5, // Cordova
		"ionic.config.json":      0.5, // Ionic
		"nativescript.config.ts": 0.5, // NativeScript
		"xamarin.forms":          0.5, // Xamarin
	}

	for file, weight := range mobileFiles {
		if stat, err := os.Stat(file); err == nil {
			if strings.HasSuffix(file, "/") && stat.IsDir() {
				indicators = append(indicators, file)
				confidence += weight
			} else if !stat.IsDir() {
				indicators = append(indicators, file)
				confidence += weight
			}

			// Detect specific frameworks
			switch file {
			case "pubspec.yaml", "flutter.yaml":
				framework = "Flutter"
			case "react-native.config.js", "metro.config.js":
				framework = "React Native"
			case "expo.json":
				framework = "Expo"
			case "capacitor.config.json":
				framework = "Capacitor"
			case "cordova.xml":
				framework = "Cordova"
			case "ionic.config.json":
				framework = "Ionic"
			case "nativescript.config.ts":
				framework = "NativeScript"
			}
		}
	}

	// Check package.json for mobile dependencies
	if packageInfo := p.analyzePackageJSON(); packageInfo != nil {
		if deps, ok := packageInfo["dependencies"].(map[string]interface{}); ok {
			mobileDeps := map[string]string{
				"react-native":       "React Native",
				"@ionic/react":       "Ionic",
				"@capacitor/core":    "Capacitor",
				"expo":               "Expo",
				"@nativescript/core": "NativeScript",
			}

			for dep, fw := range mobileDeps {
				if _, exists := deps[dep]; exists {
					confidence += 0.3
					if framework == "" {
						framework = fw
					}
					metadata["has_"+strings.ReplaceAll(dep, "/", "_")] = true
				}
			}
		}
	}

	if confidence > 0.4 {
		return &ProjectType{
			Type:       "mobile",
			Language:   p.detectPrimaryLanguage(),
			Framework:  framework,
			Confidence: confidence,
			Indicators: indicators,
			Metadata:   metadata,
		}
	}

	return nil
}

// detectDataScienceProject detects data science projects
func (p *ProjectPlugin) detectDataScienceProject(ctx context.Context) *ProjectType {
	indicators := []string{}
	confidence := 0.0
	metadata := make(map[string]interface{})

	// Check for data science files
	dsFiles := map[string]float64{
		"requirements.txt": 0.2,
		"environment.yml":  0.3,
		"conda.yml":        0.3,
		"Pipfile":          0.2,
		"pyproject.toml":   0.2,
		"setup.py":         0.1,
		"notebook.ipynb":   0.4,
		"data/":            0.2,
		"datasets/":        0.3,
		"models/":          0.3,
		"notebooks/":       0.4,
		"experiments/":     0.3,
	}

	for file, weight := range dsFiles {
		if stat, err := os.Stat(file); err == nil {
			if strings.HasSuffix(file, "/") && stat.IsDir() {
				indicators = append(indicators, file)
				confidence += weight
			} else if !stat.IsDir() {
				indicators = append(indicators, file)
				confidence += weight
			}
		}
	}

	// Check for Jupyter notebooks
	matches, _ := filepath.Glob("*.ipynb")
	if len(matches) > 0 {
		confidence += 0.5
		indicators = append(indicators, "*.ipynb")
		metadata["notebook_count"] = len(matches)
	}

	// Check for data science Python packages
	if content, err := os.ReadFile("requirements.txt"); err == nil {
		dsPackages := []string{
			"pandas", "numpy", "scipy", "scikit-learn", "matplotlib",
			"seaborn", "plotly", "jupyter", "ipython", "tensorflow",
			"pytorch", "keras", "xgboost", "lightgbm", "catboost",
		}

		contentStr := string(content)
		foundPackages := []string{}
		for _, pkg := range dsPackages {
			if strings.Contains(contentStr, pkg) {
				foundPackages = append(foundPackages, pkg)
				confidence += 0.1
			}
		}
		if len(foundPackages) > 0 {
			metadata["ds_packages"] = foundPackages
		}
	}

	if confidence > 0.4 {
		return &ProjectType{
			Type:       "data_science",
			Language:   "Python",
			Confidence: confidence,
			Indicators: indicators,
			Metadata:   metadata,
		}
	}

	return nil
}

// detectDesktopProject detects desktop application projects
func (p *ProjectPlugin) detectDesktopProject(ctx context.Context) *ProjectType {
	indicators := []string{}
	confidence := 0.0
	framework := ""
	metadata := make(map[string]interface{})

	// Check for desktop-specific files
	desktopFiles := map[string]float64{
		"main.js":         0.2, // Electron
		"electron.js":     0.4, // Electron
		"forge.config.js": 0.4, // Electron Forge
		"tauri.conf.json": 0.5, // Tauri
		"src-tauri/":      0.5, // Tauri
		"fxmanifest.xml":  0.5, // JavaFX
		"pom.xml":         0.1, // Maven (could be desktop)
		"build.gradle":    0.1, // Gradle (could be desktop)
		"CMakeLists.txt":  0.3, // CMake
		"Makefile":        0.2, // Make
		"*.pro":           0.4, // Qt
		"*.ui":            0.3, // Qt UI files
		"main.cpp":        0.2, // C++
		"main.c":          0.2, // C
		"Program.cs":      0.3, // C# console/desktop
		"*.csproj":        0.3, // C# project
		"*.sln":           0.3, // Visual Studio solution
	}

	for file, weight := range desktopFiles {
		if strings.Contains(file, "*") {
			// Handle glob patterns
			matches, _ := filepath.Glob(file)
			if len(matches) > 0 {
				indicators = append(indicators, file)
				confidence += weight
			}
		} else {
			if stat, err := os.Stat(file); err == nil {
				if strings.HasSuffix(file, "/") && stat.IsDir() {
					indicators = append(indicators, file)
					confidence += weight
					if file == "src-tauri/" {
						framework = "Tauri"
					}
				} else if !stat.IsDir() {
					indicators = append(indicators, file)
					confidence += weight

					// Detect specific frameworks
					switch file {
					case "electron.js", "main.js":
						framework = "Electron"
					case "forge.config.js":
						framework = "Electron Forge"
					case "tauri.conf.json":
						framework = "Tauri"
					}
				}
			}
		}
	}

	// Check package.json for desktop dependencies
	if packageInfo := p.analyzePackageJSON(); packageInfo != nil {
		if deps, ok := packageInfo["dependencies"].(map[string]interface{}); ok {
			desktopDeps := map[string]string{
				"electron":        "Electron",
				"@tauri-apps/api": "Tauri",
				"nw.js":           "NW.js",
			}

			for dep, fw := range desktopDeps {
				if _, exists := deps[dep]; exists {
					confidence += 0.4
					if framework == "" {
						framework = fw
					}
					metadata["has_"+strings.ReplaceAll(dep, "/", "_")] = true
				}
			}
		}
	}

	if confidence > 0.3 {
		return &ProjectType{
			Type:       "desktop",
			Language:   p.detectPrimaryLanguage(),
			Framework:  framework,
			Confidence: confidence,
			Indicators: indicators,
			Metadata:   metadata,
		}
	}

	return nil
}

// detectLibraryProject detects library/package projects
func (p *ProjectPlugin) detectLibraryProject(ctx context.Context) *ProjectType {
	indicators := []string{}
	confidence := 0.0
	metadata := make(map[string]interface{})

	// Check for library-specific files
	libFiles := map[string]float64{
		"setup.py":       0.4, // Python package
		"pyproject.toml": 0.4, // Python package
		"__init__.py":    0.2, // Python package
		"lib/":           0.3, // Library directory
		"src/lib/":       0.3, // Library source
		"dist/":          0.2, // Distribution directory
		"build/":         0.1, // Build directory
		"*.gemspec":      0.5, // Ruby gem
		"Cargo.toml":     0.4, // Rust crate
		"go.mod":         0.3, // Go module
		"composer.json":  0.4, // PHP package
	}

	for file, weight := range libFiles {
		if strings.Contains(file, "*") {
			// Handle glob patterns
			matches, _ := filepath.Glob(file)
			if len(matches) > 0 {
				indicators = append(indicators, file)
				confidence += weight
			}
		} else {
			if stat, err := os.Stat(file); err == nil {
				if strings.HasSuffix(file, "/") && stat.IsDir() {
					indicators = append(indicators, file)
					confidence += weight
				} else if !stat.IsDir() {
					indicators = append(indicators, file)
					confidence += weight
				}
			}
		}
	}

	// Check package.json for library indicators
	if packageInfo := p.analyzePackageJSON(); packageInfo != nil {
		// Check if it's published to npm
		if name, ok := packageInfo["name"].(string); ok && name != "" {
			if main, ok := packageInfo["main"].(string); ok && main != "" {
				confidence += 0.3
				metadata["npm_package"] = true
			}
		}

		// Check for library-specific fields
		if _, ok := packageInfo["exports"]; ok {
			confidence += 0.2
		}
		if _, ok := packageInfo["types"]; ok {
			confidence += 0.2
		}
	}

	if confidence > 0.3 {
		return &ProjectType{
			Type:       "library",
			Language:   p.detectPrimaryLanguage(),
			Confidence: confidence,
			Indicators: indicators,
			Metadata:   metadata,
		}
	}

	return nil
}

// detectCLIProject detects command-line interface projects
func (p *ProjectPlugin) detectCLIProject(ctx context.Context) *ProjectType {
	indicators := []string{}
	confidence := 0.0
	metadata := make(map[string]interface{})

	// Check for CLI-specific files
	cliFiles := map[string]float64{
		"main.go":     0.3, // Go CLI
		"cmd/":        0.4, // Go CLI structure
		"cli.py":      0.4, // Python CLI
		"__main__.py": 0.3, // Python CLI
		"bin/":        0.3, // Binary directory
		"scripts/":    0.2, // Scripts directory
		"Makefile":    0.2, // Build system
		"Dockerfile":  0.1, // Could be CLI tool
	}

	for file, weight := range cliFiles {
		if stat, err := os.Stat(file); err == nil {
			if strings.HasSuffix(file, "/") && stat.IsDir() {
				indicators = append(indicators, file)
				confidence += weight
			} else if !stat.IsDir() {
				indicators = append(indicators, file)
				confidence += weight
			}
		}
	}

	// Check package.json for CLI indicators
	if packageInfo := p.analyzePackageJSON(); packageInfo != nil {
		if bin, ok := packageInfo["bin"]; ok && bin != nil {
			confidence += 0.4
			metadata["has_bin"] = true
		}

		// Check for CLI dependencies
		if deps, ok := packageInfo["dependencies"].(map[string]interface{}); ok {
			cliDeps := []string{"commander", "yargs", "inquirer", "chalk", "ora"}
			foundDeps := []string{}
			for _, dep := range cliDeps {
				if _, exists := deps[dep]; exists {
					foundDeps = append(foundDeps, dep)
					confidence += 0.1
				}
			}
			if len(foundDeps) > 0 {
				metadata["cli_deps"] = foundDeps
			}
		}
	}

	if confidence > 0.3 {
		return &ProjectType{
			Type:       "cli",
			Language:   p.detectPrimaryLanguage(),
			Confidence: confidence,
			Indicators: indicators,
			Metadata:   metadata,
		}
	}

	return nil
}

// detectMicroserviceProject detects microservice projects
func (p *ProjectPlugin) detectMicroserviceProject(ctx context.Context) *ProjectType {
	indicators := []string{}
	confidence := 0.0
	metadata := make(map[string]interface{})

	// Check for microservice-specific files
	microFiles := map[string]float64{
		"Dockerfile":         0.3,
		"docker-compose.yml": 0.4,
		"k8s/":               0.4,
		"kubernetes/":        0.4,
		"helm/":              0.4,
		"service.yaml":       0.3,
		"deployment.yaml":    0.3,
		"api/":               0.2,
		"handlers/":          0.2,
		"routes/":            0.2,
		"middleware/":        0.2,
	}

	for file, weight := range microFiles {
		if stat, err := os.Stat(file); err == nil {
			if strings.HasSuffix(file, "/") && stat.IsDir() {
				indicators = append(indicators, file)
				confidence += weight
			} else if !stat.IsDir() {
				indicators = append(indicators, file)
				confidence += weight
			}
		}
	}

	// Check for microservice patterns in package.json
	if packageInfo := p.analyzePackageJSON(); packageInfo != nil {
		if deps, ok := packageInfo["dependencies"].(map[string]interface{}); ok {
			microDeps := []string{
				"express", "fastify", "koa", "hapi", "@nestjs/core",
				"grpc", "@grpc/grpc-js", "apollo-server", "graphql",
			}
			foundDeps := []string{}
			for _, dep := range microDeps {
				if _, exists := deps[dep]; exists {
					foundDeps = append(foundDeps, dep)
					confidence += 0.1
				}
			}
			if len(foundDeps) > 0 {
				metadata["micro_deps"] = foundDeps
			}
		}
	}

	if confidence > 0.4 {
		return &ProjectType{
			Type:       "microservice",
			Language:   p.detectPrimaryLanguage(),
			Confidence: confidence,
			Indicators: indicators,
			Metadata:   metadata,
		}
	}

	return nil
}

// detectGameProject detects game development projects
func (p *ProjectPlugin) detectGameProject(ctx context.Context) *ProjectType {
	indicators := []string{}
	confidence := 0.0
	framework := ""
	metadata := make(map[string]interface{})

	// Check for game-specific files
	gameFiles := map[string]float64{
		"Assets/":          0.5, // Unity
		"ProjectSettings/": 0.5, // Unity
		"*.unity":          0.6, // Unity scene
		"*.unitypackage":   0.6, // Unity package
		"project.godot":    0.6, // Godot
		"*.tscn":           0.4, // Godot scene
		"*.gd":             0.3, // Godot script
		"game.js":          0.3, // Generic game
		"index.html":       0.1, // Could be web game
		"*.blend":          0.2, // Blender files
		"*.fbx":            0.2, // 3D models
		"*.obj":            0.2, // 3D models
	}

	for file, weight := range gameFiles {
		if strings.Contains(file, "*") {
			// Handle glob patterns
			matches, _ := filepath.Glob(file)
			if len(matches) > 0 {
				indicators = append(indicators, file)
				confidence += weight

				// Detect specific engines
				if strings.Contains(file, "*.unity") {
					framework = "Unity"
				} else if strings.Contains(file, "*.tscn") || strings.Contains(file, "*.gd") {
					framework = "Godot"
				}
			}
		} else {
			if stat, err := os.Stat(file); err == nil {
				if strings.HasSuffix(file, "/") && stat.IsDir() {
					indicators = append(indicators, file)
					confidence += weight

					// Detect specific engines
					if file == "Assets/" || file == "ProjectSettings/" {
						framework = "Unity"
					}
				} else if !stat.IsDir() {
					indicators = append(indicators, file)
					confidence += weight

					if file == "project.godot" {
						framework = "Godot"
					}
				}
			}
		}
	}

	// Check package.json for game dependencies
	if packageInfo := p.analyzePackageJSON(); packageInfo != nil {
		if deps, ok := packageInfo["dependencies"].(map[string]interface{}); ok {
			gameDeps := map[string]string{
				"phaser":    "Phaser",
				"three":     "Three.js",
				"babylon":   "Babylon.js",
				"pixi.js":   "PixiJS",
				"matter-js": "Matter.js",
				"cannon":    "Cannon.js",
			}

			for dep, fw := range gameDeps {
				if _, exists := deps[dep]; exists {
					confidence += 0.3
					if framework == "" {
						framework = fw
					}
					metadata["has_"+strings.ReplaceAll(dep, ".", "_")] = true
				}
			}
		}
	}

	if confidence > 0.4 {
		return &ProjectType{
			Type:       "game",
			Language:   p.detectPrimaryLanguage(),
			Framework:  framework,
			Confidence: confidence,
			Indicators: indicators,
			Metadata:   metadata,
		}
	}

	return nil
}

// detectDocumentationProject detects documentation projects
func (p *ProjectPlugin) detectDocumentationProject(ctx context.Context) *ProjectType {
	indicators := []string{}
	confidence := 0.0
	framework := ""
	metadata := make(map[string]interface{})

	// Check for documentation-specific files
	docFiles := map[string]float64{
		"README.md":            0.1,
		"docs/":                0.4,
		"documentation/":       0.4,
		"_config.yml":          0.4, // Jekyll
		"mkdocs.yml":           0.5, // MkDocs
		"docusaurus.config.js": 0.5, // Docusaurus
		"gitbook.json":         0.5, // GitBook
		"book.json":            0.4, // GitBook
		"_book/":               0.3, // GitBook output
		"site/":                0.2, // Generated site
		"*.md":                 0.2, // Markdown files
		"*.rst":                0.3, // reStructuredText
		"conf.py":              0.4, // Sphinx
		"Makefile":             0.1, // Could be docs build
	}

	for file, weight := range docFiles {
		if strings.Contains(file, "*") {
			// Handle glob patterns
			matches, _ := filepath.Glob(file)
			if len(matches) > 2 { // Multiple markdown/rst files
				indicators = append(indicators, file)
				confidence += weight
				metadata["doc_files_count"] = len(matches)
			}
		} else {
			if stat, err := os.Stat(file); err == nil {
				if strings.HasSuffix(file, "/") && stat.IsDir() {
					indicators = append(indicators, file)
					confidence += weight
				} else if !stat.IsDir() {
					indicators = append(indicators, file)
					confidence += weight

					// Detect specific frameworks
					switch file {
					case "_config.yml":
						framework = "Jekyll"
					case "mkdocs.yml":
						framework = "MkDocs"
					case "docusaurus.config.js":
						framework = "Docusaurus"
					case "gitbook.json", "book.json":
						framework = "GitBook"
					case "conf.py":
						framework = "Sphinx"
					}
				}
			}
		}
	}

	// Check package.json for documentation dependencies
	if packageInfo := p.analyzePackageJSON(); packageInfo != nil {
		if deps, ok := packageInfo["dependencies"].(map[string]interface{}); ok {
			docDeps := map[string]string{
				"@docusaurus/core": "Docusaurus",
				"vuepress":         "VuePress",
				"gitbook-cli":      "GitBook",
				"docsify":          "Docsify",
			}

			for dep, fw := range docDeps {
				if _, exists := deps[dep]; exists {
					confidence += 0.4
					if framework == "" {
						framework = fw
					}
					metadata["has_"+strings.ReplaceAll(dep, "/", "_")] = true
				}
			}
		}
	}

	if confidence > 0.4 {
		return &ProjectType{
			Type:       "documentation",
			Language:   p.detectPrimaryLanguage(),
			Framework:  framework,
			Confidence: confidence,
			Indicators: indicators,
			Metadata:   metadata,
		}
	}

	return nil
}

// detectInfrastructureProject detects infrastructure/DevOps projects
func (p *ProjectPlugin) detectInfrastructureProject(ctx context.Context) *ProjectType {
	indicators := []string{}
	confidence := 0.0
	framework := ""
	metadata := make(map[string]interface{})

	// Check for infrastructure-specific files
	infraFiles := map[string]float64{
		"terraform/":         0.5,
		"*.tf":               0.5, // Terraform
		"*.tfvars":           0.4, // Terraform variables
		"ansible/":           0.5,
		"playbook.yml":       0.5, // Ansible
		"inventory":          0.3, // Ansible
		"Vagrantfile":        0.5, // Vagrant
		"docker-compose.yml": 0.3,
		"Dockerfile":         0.2,
		"k8s/":               0.4,
		"kubernetes/":        0.4,
		"helm/":              0.4,
		"Chart.yaml":         0.5, // Helm
		"values.yaml":        0.3, // Helm
		"cloudformation/":    0.5,
		"*.yaml":             0.1, // Could be k8s/ansible
		"*.yml":              0.1, // Could be k8s/ansible
		"Pulumi.yaml":        0.5, // Pulumi
		"serverless.yml":     0.5, // Serverless
	}

	for file, weight := range infraFiles {
		if strings.Contains(file, "*") {
			// Handle glob patterns
			matches, _ := filepath.Glob(file)
			if len(matches) > 0 {
				indicators = append(indicators, file)
				confidence += weight

				// Detect specific tools
				if strings.Contains(file, "*.tf") {
					framework = "Terraform"
				}
			}
		} else {
			if stat, err := os.Stat(file); err == nil {
				if strings.HasSuffix(file, "/") && stat.IsDir() {
					indicators = append(indicators, file)
					confidence += weight

					// Detect specific tools
					switch file {
					case "terraform/":
						framework = "Terraform"
					case "ansible/":
						framework = "Ansible"
					case "k8s/", "kubernetes/":
						framework = "Kubernetes"
					case "helm/":
						framework = "Helm"
					case "cloudformation/":
						framework = "CloudFormation"
					}
				} else if !stat.IsDir() {
					indicators = append(indicators, file)
					confidence += weight

					// Detect specific tools
					switch file {
					case "playbook.yml":
						framework = "Ansible"
					case "Vagrantfile":
						framework = "Vagrant"
					case "Chart.yaml":
						framework = "Helm"
					case "Pulumi.yaml":
						framework = "Pulumi"
					case "serverless.yml":
						framework = "Serverless"
					}
				}
			}
		}
	}

	if confidence > 0.4 {
		return &ProjectType{
			Type:       "infrastructure",
			Language:   p.detectPrimaryLanguage(),
			Framework:  framework,
			Confidence: confidence,
			Indicators: indicators,
			Metadata:   metadata,
		}
	}

	return nil
}

// determinePrimaryType determines the primary project type from detected types
func (p *ProjectPlugin) determinePrimaryType(projectTypes []ProjectType) *ProjectType {
	if len(projectTypes) == 0 {
		return nil
	}

	// Find the type with highest confidence
	var primaryType *ProjectType
	maxConfidence := 0.0

	for i := range projectTypes {
		if projectTypes[i].Confidence > maxConfidence {
			maxConfidence = projectTypes[i].Confidence
			primaryType = &projectTypes[i]
		}
	}

	return primaryType
}

// detectPrimaryLanguage detects the primary programming language
func (p *ProjectPlugin) detectPrimaryLanguage() string {
	// Language detection based on file extensions
	languageFiles := map[string][]string{
		"JavaScript": {"*.js", "*.jsx", "*.mjs", "*.ts", "*.tsx"},
		"Python":     {"*.py", "*.pyw", "*.pyi"},
		"Go":         {"*.go"},
		"Java":       {"*.java"},
		"C++":        {"*.cpp", "*.cxx", "*.cc", "*.c++"},
		"C":          {"*.c", "*.h"},
		"C#":         {"*.cs"},
		"Ruby":       {"*.rb"},
		"PHP":        {"*.php"},
		"Rust":       {"*.rs"},
		"Swift":      {"*.swift"},
		"Kotlin":     {"*.kt", "*.kts"},
		"Dart":       {"*.dart"},
		"Scala":      {"*.scala"},
		"R":          {"*.r", "*.R"},
		"Shell":      {"*.sh", "*.bash", "*.zsh"},
	}

	languageCounts := make(map[string]int)

	for language, patterns := range languageFiles {
		totalCount := 0
		for _, pattern := range patterns {
			matches, _ := filepath.Glob(pattern)
			totalCount += len(matches)
		}
		if totalCount > 0 {
			languageCounts[language] = totalCount
		}
	}

	// Find language with most files
	maxCount := 0
	primaryLanguage := "Unknown"
	for language, count := range languageCounts {
		if count > maxCount {
			maxCount = count
			primaryLanguage = language
		}
	}

	return primaryLanguage
}

// analyzePackageJSON analyzes package.json if it exists
func (p *ProjectPlugin) analyzePackageJSON() map[string]interface{} {
	content, err := os.ReadFile("package.json")
	if err != nil {
		return nil
	}

	var packageInfo map[string]interface{}
	if err := json.Unmarshal(content, &packageInfo); err != nil {
		return nil
	}

	return packageInfo
}

// gatherLanguageContext gathers language-specific context
func (p *ProjectPlugin) gatherLanguageContext(ctx context.Context, projectTypes []ProjectType) map[string]interface{} {
	languageContext := make(map[string]interface{})

	// Gather context for each detected language
	languages := make(map[string]bool)
	for _, pt := range projectTypes {
		if pt.Language != "" && pt.Language != "Unknown" {
			languages[pt.Language] = true
		}
	}

	for language := range languages {
		select {
		case <-ctx.Done():
			return languageContext
		default:
		}

		switch language {
		case "JavaScript", "TypeScript":
			languageContext[language] = p.gatherJavaScriptContext()
		case "Python":
			languageContext[language] = p.gatherPythonContext()
		case "Go":
			languageContext[language] = p.gatherGoContext()
		case "Java":
			languageContext[language] = p.gatherJavaContext()
		}
	}

	return languageContext
}

// gatherJavaScriptContext gathers JavaScript/TypeScript specific context
func (p *ProjectPlugin) gatherJavaScriptContext() map[string]interface{} {
	context := make(map[string]interface{})

	// Check for TypeScript
	if _, err := os.Stat("tsconfig.json"); err == nil {
		context["typescript"] = true
	}

	// Check for common config files
	configFiles := []string{
		"webpack.config.js", "vite.config.js", "rollup.config.js",
		"babel.config.js", ".babelrc", "jest.config.js", ".eslintrc.js",
		"prettier.config.js", ".prettierrc",
	}

	foundConfigs := []string{}
	for _, config := range configFiles {
		if _, err := os.Stat(config); err == nil {
			foundConfigs = append(foundConfigs, config)
		}
	}
	if len(foundConfigs) > 0 {
		context["config_files"] = foundConfigs
	}

	return context
}

// gatherPythonContext gathers Python specific context
func (p *ProjectPlugin) gatherPythonContext() map[string]interface{} {
	context := make(map[string]interface{})

	// Check Python version files
	versionFiles := []string{".python-version", "runtime.txt"}
	for _, file := range versionFiles {
		if content, err := os.ReadFile(file); err == nil {
			context["version_file"] = file
			context["version"] = strings.TrimSpace(string(content))
			break
		}
	}

	// Check for virtual environment
	venvDirs := []string{"venv", ".venv", "env", ".env"}
	for _, dir := range venvDirs {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			context["virtual_env"] = dir
			break
		}
	}

	return context
}

// gatherGoContext gathers Go specific context
func (p *ProjectPlugin) gatherGoContext() map[string]interface{} {
	context := make(map[string]interface{})

	// Check go.mod for module info
	if content, err := os.ReadFile("go.mod"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				context["module"] = strings.TrimPrefix(line, "module ")
				break
			}
		}
	}

	return context
}

// gatherJavaContext gathers Java specific context
func (p *ProjectPlugin) gatherJavaContext() map[string]interface{} {
	context := make(map[string]interface{})

	// Check for build tools
	if _, err := os.Stat("pom.xml"); err == nil {
		context["build_tool"] = "Maven"
	} else if _, err := os.Stat("build.gradle"); err == nil {
		context["build_tool"] = "Gradle"
	}

	return context
}

// analyzeProjectStructure analyzes the overall project structure
func (p *ProjectPlugin) analyzeProjectStructure(ctx context.Context) map[string]interface{} {
	structure := make(map[string]interface{})

	// Count different types of files
	fileCounts := make(map[string]int)
	dirCounts := make(map[string]int)

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden files and directories
		if strings.HasPrefix(filepath.Base(path), ".") && path != "." {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip common build/dependency directories
		skipDirs := []string{"node_modules", "vendor", "target", "build", "dist", "__pycache__"}
		for _, skipDir := range skipDirs {
			if strings.Contains(path, skipDir) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if info.IsDir() {
			dirCounts["total"]++
		} else {
			fileCounts["total"]++
			ext := strings.ToLower(filepath.Ext(path))
			if ext != "" {
				fileCounts[ext]++
			}
		}

		return nil
	})

	if err == nil {
		structure["file_counts"] = fileCounts
		structure["dir_counts"] = dirCounts
	}

	return structure
}
