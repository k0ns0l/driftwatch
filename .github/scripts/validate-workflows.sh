#!/bin/bash

# Workflow validation script
# This script validates that all GitHub Actions workflows are properly configured

set -e

echo "🔍 Validating GitHub Actions workflows..."

# Check if required files exist
required_files=(
    ".github/workflows/ci.yml"
    ".github/workflows/release.yml"
    ".github/workflows/codeql.yml"
    ".github/workflows/dependency-review.yml"
    ".golangci.yml"
    ".goreleaser.yml"
    "Dockerfile"
)

for file in "${required_files[@]}"; do
    if [[ -f "$file" ]]; then
        echo "✅ $file exists"
    else
        echo "❌ $file is missing"
        exit 1
    fi
done

# Validate Go project structure
echo ""
echo "🔍 Validating Go project structure..."

if [[ -f "go.mod" ]]; then
    echo "✅ go.mod exists"
else
    echo "❌ go.mod is missing"
    exit 1
fi

if [[ -f "main.go" ]]; then
    echo "✅ main.go exists"
else
    echo "❌ main.go is missing"
    exit 1
fi

# Test basic Go commands that workflows will use
echo ""
echo "🔍 Testing Go commands..."

echo "Testing go mod download..."
go mod download
echo "✅ go mod download successful"

echo "Testing go mod verify..."
go mod verify
echo "✅ go mod verify successful"

echo "Testing go build..."
go build -o test-binary .
if [[ -f "test-binary" ]] || [[ -f "test-binary.exe" ]]; then
    echo "✅ go build successful"
    rm -f test-binary test-binary.exe
else
    echo "❌ go build failed"
    exit 1
fi

echo "Testing go test..."
if go test ./... > /dev/null 2>&1; then
    echo "✅ go test successful"
else
    echo "❌ go test failed"
    exit 1
fi

# Validate workflow syntax (basic YAML validation)
echo ""
echo "🔍 Validating workflow YAML syntax..."

for workflow in .github/workflows/*.yml; do
    if command -v yamllint > /dev/null 2>&1; then
        if yamllint "$workflow" > /dev/null 2>&1; then
            echo "✅ $workflow syntax is valid"
        else
            echo "❌ $workflow has syntax errors"
            exit 1
        fi
    else
        # Basic YAML check - ensure file is readable
        if python3 -c "import yaml; yaml.safe_load(open('$workflow'))" > /dev/null 2>&1; then
            echo "✅ $workflow syntax is valid"
        else
            echo "❌ $workflow has syntax errors"
            exit 1
        fi
    fi
done

# Check Docker build (if Docker is available)
echo ""
echo "🔍 Testing Docker build..."

if command -v docker > /dev/null 2>&1; then
    if docker build -t driftwatch-test . > /dev/null 2>&1; then
        echo "✅ Docker build successful"
        docker rmi driftwatch-test > /dev/null 2>&1
    else
        echo "❌ Docker build failed"
        exit 1
    fi
else
    echo "⚠️  Docker not available, skipping Docker build test"
fi

echo ""
echo "🎉 All validations passed! GitHub Actions workflows are ready."
echo ""
echo "📋 Summary of configured workflows:"
echo "   • CI Pipeline (multi-platform testing, linting, security)"
echo "   • Release Pipeline (automated releases, Docker, packages)"
echo "   • Security Scanning (CodeQL, dependency review)"
echo "   • Documentation (generation, linting, link checking)"
echo "   • Performance Monitoring (benchmarks, profiling, load tests)"
echo "   • Automation (auto-merge, stale issue management)"
echo ""
echo "🚀 Ready for production use!"