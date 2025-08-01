package plugins

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kanishka-sahoo/nl-to-shell/internal/types"
)

func TestProjectPlugin_Name(t *testing.T) {
	plugin := NewProjectPlugin()
	if plugin.Name() != "project" {
		t.Errorf("Expected plugin name 'project', got '%s'", plugin.Name())
	}
}

func TestProjectPlugin_Priority(t *testing.T) {
	plugin := NewProjectPlugin()
	if plugin.Priority() != 80 {
		t.Errorf("Expected plugin priority 80, got %d", plugin.Priority())
	}
}

func TestProjectPlugin_GatherContext(t *testing.T) {
	plugin := NewProjectPlugin()

	ctx := context.Background()
	baseContext := &types.Context{
		WorkingDirectory: "/test/dir",
	}

	result, err := plugin.GatherContext(ctx, baseContext)
	if err != nil {
		t.Fatalf("GatherContext failed: %v", err)
	}

	// Check that result has expected structure
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Check that all expected sections are present
	expectedSections := []string{"types", "primary_type", "language_context", "structure"}
	for _, section := range expectedSections {
		if _, exists := result[section]; !exists {
			t.Errorf("Expected section '%s' to be present in result", section)
		}
	}

	// Check types section
	types, ok := result["types"].([]ProjectType)
	if !ok {
		t.Fatal("Expected types to be []ProjectType")
	}

	// Types can be empty if no project files are detected
	_ = types
}

func TestProjectPlugin_GatherContext_WithCancellation(t *testing.T) {
	plugin := NewProjectPlugin()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	baseContext := &types.Context{
		WorkingDirectory: "/test/dir",
	}

	result, err := plugin.GatherContext(ctx, baseContext)
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}

	if result != nil {
		t.Error("Expected nil result due to cancelled context")
	}
}

func TestProjectPlugin_detectWebProject(t *testing.T) {
	plugin := &ProjectPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "project-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	ctx := context.Background()

	// Test without web files
	webProject := plugin.detectWebProject(ctx)
	if webProject != nil {
		t.Error("Expected nil webProject when no web files exist")
	}

	// Create package.json with web dependencies
	packageJSON := `{
		"name": "test-web-app",
		"version": "1.0.0",
		"dependencies": {
			"react": "^18.0.0",
			"react-dom": "^18.0.0"
		},
		"scripts": {
			"start": "react-scripts start",
			"build": "react-scripts build"
		}
	}`
	err = os.WriteFile("package.json", []byte(packageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	webProject = plugin.detectWebProject(ctx)
	if webProject == nil {
		t.Fatal("Expected non-nil webProject when package.json with React exists")
	}

	if webProject.Type != "web" {
		t.Errorf("Expected project type 'web', got '%s'", webProject.Type)
	}

	if webProject.Framework != "React" {
		t.Errorf("Expected framework 'React', got '%s'", webProject.Framework)
	}

	if webProject.Confidence <= 0.3 {
		t.Errorf("Expected confidence > 0.3, got %f", webProject.Confidence)
	}

	// Create Next.js config
	nextConfig := `module.exports = { reactStrictMode: true }`
	err = os.WriteFile("next.config.js", []byte(nextConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create next.config.js: %v", err)
	}

	webProject = plugin.detectWebProject(ctx)
	if webProject.Framework != "Next.js" {
		t.Errorf("Expected framework 'Next.js', got '%s'", webProject.Framework)
	}
}

func TestProjectPlugin_detectMobileProject(t *testing.T) {
	plugin := &ProjectPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "project-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	ctx := context.Background()

	// Test without mobile files
	mobileProject := plugin.detectMobileProject(ctx)
	if mobileProject != nil {
		t.Error("Expected nil mobileProject when no mobile files exist")
	}

	// Create Flutter pubspec.yaml
	pubspec := `name: test_flutter_app
description: A test Flutter application
version: 1.0.0+1

environment:
  sdk: ">=2.17.0 <4.0.0"

dependencies:
  flutter:
    sdk: flutter
  cupertino_icons: ^1.0.2`

	err = os.WriteFile("pubspec.yaml", []byte(pubspec), 0644)
	if err != nil {
		t.Fatalf("Failed to create pubspec.yaml: %v", err)
	}

	mobileProject = plugin.detectMobileProject(ctx)
	if mobileProject == nil {
		t.Fatal("Expected non-nil mobileProject when pubspec.yaml exists")
	}

	if mobileProject.Type != "mobile" {
		t.Errorf("Expected project type 'mobile', got '%s'", mobileProject.Type)
	}

	if mobileProject.Framework != "Flutter" {
		t.Errorf("Expected framework 'Flutter', got '%s'", mobileProject.Framework)
	}

	if mobileProject.Confidence <= 0.4 {
		t.Errorf("Expected confidence > 0.4, got %f", mobileProject.Confidence)
	}
}

func TestProjectPlugin_detectDataScienceProject(t *testing.T) {
	plugin := &ProjectPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "project-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	ctx := context.Background()

	// Test without data science files
	dsProject := plugin.detectDataScienceProject(ctx)
	if dsProject != nil {
		t.Error("Expected nil dsProject when no data science files exist")
	}

	// Create requirements.txt with data science packages
	requirements := `pandas==1.5.0
numpy==1.23.0
scikit-learn==1.1.0
matplotlib==3.6.0
jupyter==1.0.0`

	err = os.WriteFile("requirements.txt", []byte(requirements), 0644)
	if err != nil {
		t.Fatalf("Failed to create requirements.txt: %v", err)
	}

	// Create a Jupyter notebook
	notebook := `{
 "cells": [],
 "metadata": {
  "kernelspec": {
   "display_name": "Python 3",
   "language": "python",
   "name": "python3"
  }
 },
 "nbformat": 4,
 "nbformat_minor": 4
}`

	err = os.WriteFile("analysis.ipynb", []byte(notebook), 0644)
	if err != nil {
		t.Fatalf("Failed to create analysis.ipynb: %v", err)
	}

	dsProject = plugin.detectDataScienceProject(ctx)
	if dsProject == nil {
		t.Fatal("Expected non-nil dsProject when data science files exist")
	}

	if dsProject.Type != "data_science" {
		t.Errorf("Expected project type 'data_science', got '%s'", dsProject.Type)
	}

	if dsProject.Language != "Python" {
		t.Errorf("Expected language 'Python', got '%s'", dsProject.Language)
	}

	if dsProject.Confidence <= 0.4 {
		t.Errorf("Expected confidence > 0.4, got %f", dsProject.Confidence)
	}

	// Check metadata
	if dsProject.Metadata["notebook_count"] != 1 {
		t.Errorf("Expected notebook_count to be 1, got %v", dsProject.Metadata["notebook_count"])
	}

	dsPackages, ok := dsProject.Metadata["ds_packages"].([]string)
	if !ok {
		t.Fatal("Expected ds_packages to be []string")
	}

	expectedPackages := []string{"pandas", "numpy", "scikit-learn", "matplotlib", "jupyter"}
	for _, expectedPkg := range expectedPackages {
		found := false
		for _, pkg := range dsPackages {
			if pkg == expectedPkg {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected package '%s' to be detected", expectedPkg)
		}
	}
}

func TestProjectPlugin_detectDesktopProject(t *testing.T) {
	plugin := &ProjectPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "project-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	ctx := context.Background()

	// Test without desktop files
	desktopProject := plugin.detectDesktopProject(ctx)
	if desktopProject != nil {
		t.Error("Expected nil desktopProject when no desktop files exist")
	}

	// Create Electron main.js
	mainJS := `const { app, BrowserWindow } = require('electron')

function createWindow () {
  const mainWindow = new BrowserWindow({
    width: 800,
    height: 600
  })
  mainWindow.loadFile('index.html')
}

app.whenReady().then(createWindow)`

	err = os.WriteFile("main.js", []byte(mainJS), 0644)
	if err != nil {
		t.Fatalf("Failed to create main.js: %v", err)
	}

	// Create package.json with Electron
	packageJSON := `{
		"name": "test-electron-app",
		"version": "1.0.0",
		"main": "main.js",
		"dependencies": {
			"electron": "^20.0.0"
		}
	}`
	err = os.WriteFile("package.json", []byte(packageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	desktopProject = plugin.detectDesktopProject(ctx)
	if desktopProject == nil {
		t.Fatal("Expected non-nil desktopProject when Electron files exist")
	}

	if desktopProject.Type != "desktop" {
		t.Errorf("Expected project type 'desktop', got '%s'", desktopProject.Type)
	}

	if desktopProject.Framework != "Electron" {
		t.Errorf("Expected framework 'Electron', got '%s'", desktopProject.Framework)
	}

	if desktopProject.Confidence <= 0.3 {
		t.Errorf("Expected confidence > 0.3, got %f", desktopProject.Confidence)
	}
}

func TestProjectPlugin_detectLibraryProject(t *testing.T) {
	plugin := &ProjectPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "project-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	ctx := context.Background()

	// Test without library files
	libProject := plugin.detectLibraryProject(ctx)
	if libProject != nil {
		t.Error("Expected nil libProject when no library files exist")
	}

	// Create setup.py
	setupPy := `from setuptools import setup, find_packages

setup(
    name="test-library",
    version="1.0.0",
    packages=find_packages(),
    install_requires=[
        "requests",
    ],
)`

	err = os.WriteFile("setup.py", []byte(setupPy), 0644)
	if err != nil {
		t.Fatalf("Failed to create setup.py: %v", err)
	}

	// Create lib directory
	err = os.Mkdir("lib", 0755)
	if err != nil {
		t.Fatalf("Failed to create lib directory: %v", err)
	}

	libProject = plugin.detectLibraryProject(ctx)
	if libProject == nil {
		t.Fatal("Expected non-nil libProject when library files exist")
	}

	if libProject.Type != "library" {
		t.Errorf("Expected project type 'library', got '%s'", libProject.Type)
	}

	if libProject.Confidence <= 0.3 {
		t.Errorf("Expected confidence > 0.3, got %f", libProject.Confidence)
	}
}

func TestProjectPlugin_detectCLIProject(t *testing.T) {
	plugin := &ProjectPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "project-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	ctx := context.Background()

	// Test without CLI files
	cliProject := plugin.detectCLIProject(ctx)
	if cliProject != nil {
		t.Error("Expected nil cliProject when no CLI files exist")
	}

	// Create package.json with bin field
	packageJSON := `{
		"name": "test-cli",
		"version": "1.0.0",
		"bin": {
			"test-cli": "./bin/cli.js"
		},
		"dependencies": {
			"commander": "^9.0.0",
			"chalk": "^4.0.0"
		}
	}`
	err = os.WriteFile("package.json", []byte(packageJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	// Create bin directory
	err = os.Mkdir("bin", 0755)
	if err != nil {
		t.Fatalf("Failed to create bin directory: %v", err)
	}

	cliProject = plugin.detectCLIProject(ctx)
	if cliProject == nil {
		t.Fatal("Expected non-nil cliProject when CLI files exist")
	}

	if cliProject.Type != "cli" {
		t.Errorf("Expected project type 'cli', got '%s'", cliProject.Type)
	}

	if cliProject.Confidence <= 0.3 {
		t.Errorf("Expected confidence > 0.3, got %f", cliProject.Confidence)
	}

	// Check metadata
	if !cliProject.Metadata["has_bin"].(bool) {
		t.Error("Expected has_bin to be true")
	}

	cliDeps, ok := cliProject.Metadata["cli_deps"].([]string)
	if !ok {
		t.Fatal("Expected cli_deps to be []string")
	}

	expectedDeps := []string{"commander", "chalk"}
	for _, expectedDep := range expectedDeps {
		found := false
		for _, dep := range cliDeps {
			if dep == expectedDep {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected CLI dependency '%s' to be detected", expectedDep)
		}
	}
}

func TestProjectPlugin_detectGameProject(t *testing.T) {
	plugin := &ProjectPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "project-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	ctx := context.Background()

	// Test without game files
	gameProject := plugin.detectGameProject(ctx)
	if gameProject != nil {
		t.Error("Expected nil gameProject when no game files exist")
	}

	// Create Godot project file
	godotProject := `[application]
config/name="Test Game"
run/main_scene="res://Main.tscn"`

	err = os.WriteFile("project.godot", []byte(godotProject), 0644)
	if err != nil {
		t.Fatalf("Failed to create project.godot: %v", err)
	}

	gameProject = plugin.detectGameProject(ctx)
	if gameProject == nil {
		t.Fatal("Expected non-nil gameProject when Godot files exist")
	}

	if gameProject.Type != "game" {
		t.Errorf("Expected project type 'game', got '%s'", gameProject.Type)
	}

	if gameProject.Framework != "Godot" {
		t.Errorf("Expected framework 'Godot', got '%s'", gameProject.Framework)
	}

	if gameProject.Confidence <= 0.4 {
		t.Errorf("Expected confidence > 0.4, got %f", gameProject.Confidence)
	}
}

func TestProjectPlugin_detectPrimaryLanguage(t *testing.T) {
	plugin := &ProjectPlugin{}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "project-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Test with no files
	language := plugin.detectPrimaryLanguage()
	if language != "Unknown" {
		t.Errorf("Expected 'Unknown' language, got '%s'", language)
	}

	// Create JavaScript files
	jsFiles := []string{"index.js", "app.js", "utils.js"}
	for _, file := range jsFiles {
		err = os.WriteFile(file, []byte("console.log('test');"), 0644)
		if err != nil {
			t.Fatalf("Failed to create %s: %v", file, err)
		}
	}

	language = plugin.detectPrimaryLanguage()
	if language != "JavaScript" {
		t.Errorf("Expected 'JavaScript' language, got '%s'", language)
	}

	// Create more Python files
	pyFiles := []string{"main.py", "utils.py", "models.py", "views.py"}
	for _, file := range pyFiles {
		err = os.WriteFile(file, []byte("print('test')"), 0644)
		if err != nil {
			t.Fatalf("Failed to create %s: %v", file, err)
		}
	}

	language = plugin.detectPrimaryLanguage()
	if language != "Python" {
		t.Errorf("Expected 'Python' language, got '%s'", language)
	}
}

func TestProjectPlugin_determinePrimaryType(t *testing.T) {
	plugin := &ProjectPlugin{}

	// Test with empty types
	primaryType := plugin.determinePrimaryType([]ProjectType{})
	if primaryType != nil {
		t.Error("Expected nil primary type for empty types")
	}

	// Test with single type
	types := []ProjectType{
		{Type: "web", Confidence: 0.8},
	}
	primaryType = plugin.determinePrimaryType(types)
	if primaryType == nil {
		t.Fatal("Expected non-nil primary type")
	}
	if primaryType.Type != "web" {
		t.Errorf("Expected primary type 'web', got '%s'", primaryType.Type)
	}

	// Test with multiple types
	types = []ProjectType{
		{Type: "web", Confidence: 0.6},
		{Type: "library", Confidence: 0.8},
		{Type: "cli", Confidence: 0.4},
	}
	primaryType = plugin.determinePrimaryType(types)
	if primaryType == nil {
		t.Fatal("Expected non-nil primary type")
	}
	if primaryType.Type != "library" {
		t.Errorf("Expected primary type 'library', got '%s'", primaryType.Type)
	}
}

func TestProjectPlugin_Integration(t *testing.T) {
	plugin := NewProjectPlugin()

	// Test that the plugin implements the ContextPlugin interface correctly
	if plugin.Name() == "" {
		t.Error("Plugin name should not be empty")
	}

	if plugin.Priority() < 0 {
		t.Error("Plugin priority should be non-negative")
	}

	ctx := context.Background()
	baseContext := &types.Context{
		WorkingDirectory: "/test/dir",
	}

	result, err := plugin.GatherContext(ctx, baseContext)
	if err != nil {
		t.Fatalf("GatherContext should not fail: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// Check that all expected sections are present
	expectedSections := []string{"types", "primary_type", "language_context", "structure"}
	for _, section := range expectedSections {
		if _, exists := result[section]; !exists {
			t.Errorf("Expected section '%s' to be present in result", section)
		}
	}
}

func TestProjectPlugin_GatherContext_WithTimeout(t *testing.T) {
	plugin := NewProjectPlugin()

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Sleep to ensure timeout
	time.Sleep(1 * time.Millisecond)

	baseContext := &types.Context{
		WorkingDirectory: "/test/dir",
	}

	result, err := plugin.GatherContext(ctx, baseContext)
	if err == nil {
		t.Error("Expected error due to timeout")
	}

	if result != nil {
		t.Error("Expected nil result due to timeout")
	}
}
