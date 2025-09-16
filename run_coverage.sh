#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

echo "Generating code coverage report for ./pkg, ./cmd"

# Define the target package patterns. The "... " suffix means all packages
# within that directory recursively.
TARGET_PACKAGES="./pkg/... ./cmd/..."

# Generate the coverage profile.
# -covermode=atomic is generally preferred for more accurate results with concurrency.
# If your tests in ./cmd might call code in ./pkg, and you want that pkg code
# to be part of the coverage report, this command will handle it correctly.
# go test will run tests found in TARGET_PACKAGES and measure coverage
# for code within those packages exercised by the tests.
go test -coverprofile=coverage.out -covermode=atomic $TARGET_PACKAGES

echo "Coverage profile generated: coverage.out"

# Display function-level coverage in the terminal (optional, but good for a quick check)
echo ""
echo "Function-level coverage:"
go tool cover -func=coverage.out

# Calculate and display total coverage percentage
# We grep for the 'total:' line and then use awk to print the 3rd field (the percentage)
TOTAL_COVERAGE=$(go tool cover -func=coverage.out | grep '^total:' | awk '{print $3}')

echo ""
echo "----------------------------------------"
echo "Total Code Coverage: $TOTAL_COVERAGE"
echo "----------------------------------------"

# Optional: Generate an HTML report for a more detailed view
echo ""
echo "To view a detailed HTML report, run:"
echo "  go tool cover -html=coverage.out -o coverage.html"
echo "Then open coverage.html in your browser."

# Uncomment the next two lines if you want to automatically generate and open the HTML report
# go tool cover -html=coverage.out -o coverage.html
# echo "HTML report generated: coverage.html (opening...)"
# (Optional) Open the HTML report (macOS example, adapt for Linux/WSL)
# open coverage.html

# Clean up the profile file if you want
# rm coverage.out