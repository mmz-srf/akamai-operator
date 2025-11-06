# Testing Documentation

This document provides comprehensive information about testing the Akamai Operator, including all available Go tests, how to run them, and what they validate.

## Table of Contents

- [Overview](#overview)
- [Running Tests](#running-tests)
- [Test Suites](#test-suites)
  - [Hostname Management Tests](#hostname-management-tests)
  - [Rules Comparison Tests](#rules-comparison-tests)
  - [Real World Tests](#real-world-tests)
- [Test Coverage](#test-coverage)
- [Writing New Tests](#writing-new-tests)
- [Continuous Integration](#continuous-integration)

## Overview

The Akamai Operator includes comprehensive unit tests covering critical functionality:

- **Hostname management** - Validation of hostname comparison and change detection
- **Rules comparison** - Property rules normalization and difference detection
- **Real-world scenarios** - Tests based on actual Akamai API responses

All tests are written using Go's standard `testing` package and can be run with the `go test` command.

## Running Tests

### Run All Tests

```bash
# Run all tests in the project
go test ./...

# Run with verbose output
go test ./... -v

# Run with coverage
go test ./... -cover

# Run with detailed coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Run Specific Package Tests

```bash
# Run hostname tests only
go test ./pkg/akamai/... -v

# Run controller tests only
go test ./controllers/... -v
```

### Run Specific Test Functions

```bash
# Run a specific test by name
go test ./pkg/akamai -run TestCompareHostnames -v

# Run tests matching a pattern
go test ./controllers -run TestRules -v
```

### Run Tests with Race Detection

```bash
# Detect race conditions
go test ./... -race
```

## Test Suites

### Hostname Management Tests

**Location:** `pkg/akamai/hostnames_test.go`

These tests validate the hostname comparison logic used to detect when property hostnames need to be updated.

#### Test: `TestCompareHostnames`

Validates hostname comparison in various scenarios:

| Subtest | Description | Expected Result |
|---------|-------------|-----------------|
| `identical_hostnames` | Same hostnames in same order | No difference |
| `different_count` | Different number of hostnames | Difference detected |
| `different_cnameTo` | Different edge hostname targets | Difference detected |
| `different_cnameFrom` | Different source hostnames | Difference detected |
| `different_certProvisioningType` | Different certificate provisioning | Difference detected |
| `empty_desired_certProvisioningType_matches_any` | Empty cert type in desired state | No difference (any match) |
| `multiple_hostnames_in_different_order` | Same hostnames, different order | No difference |
| `both_empty` | Both lists empty | No difference |
| `desired_empty_current_has_hostnames` | Removing all hostnames | Difference detected |
| `current_empty_desired_has_hostnames` | Adding hostnames to empty property | Difference detected |

**Run this test:**
```bash
go test ./pkg/akamai -run TestCompareHostnames -v
```

**Example output:**
```
=== RUN   TestCompareHostnames
=== RUN   TestCompareHostnames/identical_hostnames
=== RUN   TestCompareHostnames/different_count
=== RUN   TestCompareHostnames/different_cnameTo
=== RUN   TestCompareHostnames/different_cnameFrom
=== RUN   TestCompareHostnames/different_certProvisioningType
=== RUN   TestCompareHostnames/empty_desired_certProvisioningType_matches_any
=== RUN   TestCompareHostnames/multiple_hostnames_in_different_order
=== RUN   TestCompareHostnames/both_empty
=== RUN   TestCompareHostnames/desired_empty_current_has_hostnames
=== RUN   TestCompareHostnames/current_empty_desired_has_hostnames
--- PASS: TestCompareHostnames (0.00s)
```

#### Test: `TestCompareHostnamesWithMultipleHostnames`

Validates complex scenarios with multiple hostnames and modifications.

**Run this test:**
```bash
go test ./pkg/akamai -run TestCompareHostnamesWithMultipleHostnames -v
```

### Rules Comparison Tests

**Location:** `controllers/rules_comparison_test.go`

These tests validate the property rules comparison logic, which is critical for detecting when property configurations need to be updated.

#### Test: `TestRulesNeedUpdate`

Validates rules comparison in various scenarios:

| Subtest | Description | Expected Result |
|---------|-------------|-----------------|
| `nil_desired_rules` | No rules specified | No update needed |
| `identical_rules` | Exact same rules | No update needed |
| `different_behavior_options` | Changed behavior options | Update needed |
| `ignore_auto-generated_UUID_differences` | UUID added by Akamai | No update needed (ignored) |
| `ignore_empty_string_values` | Empty string vs null | No update needed (normalized) |
| `different_criteria` | Changed matching criteria | Update needed |
| `null_options_vs_empty_object_options` | Null vs empty object | No update needed (normalized) |
| `missing_criteriaMustSatisfy_vs_all` | Default "all" value | No update needed (normalized) |
| `customOverride_null_handling` | Null custom overrides | No update needed (normalized) |

**Run this test:**
```bash
go test ./controllers -run TestRulesNeedUpdate -v
```

**What it validates:**
- Rules are normalized before comparison
- Auto-generated fields (UUIDs) are ignored
- Null vs empty values are handled correctly
- Behavior and criteria changes are detected
- Default values are properly handled

#### Test: `TestNormalizeCurrentRules`

Validates the normalization of rules received from the Akamai API.

**Run this test:**
```bash
go test ./controllers -run TestNormalizeCurrentRules -v
```

**What it validates:**
- UUIDs are removed from current state
- Empty arrays are normalized
- Default values are set consistently
- Null vs empty object handling

#### Test: `TestEmptyArraysAndObjects`

Validates handling of empty arrays and objects in rules.

**Subtests:**
- `empty_criteria_array_vs_no_criteria` - Empty array vs missing field
- `empty_children_array_vs_no_children` - Empty children vs missing field

**Run this test:**
```bash
go test ./controllers -run TestEmptyArraysAndObjects -v
```

### Real World Tests

**Location:** `controllers/real_world_comparison_test.go`

These tests use actual examples from real Akamai properties to ensure the operator handles production scenarios correctly.

#### Test: `TestRealWorldComparison`

Tests comparison using real-world YAML configuration and Akamai API responses.

**Run this test:**
```bash
go test ./controllers -run TestRealWorldComparison -v
```

**What it validates:**
- Actual property configurations from production
- Real Akamai API response handling
- Edge cases found in real deployments
- Complex rule structures with nested children

## Test Coverage

### View Coverage Report

```bash
# Generate coverage report
go test ./... -coverprofile=coverage.out

# View in terminal
go tool cover -func=coverage.out

# View in browser (HTML)
go tool cover -html=coverage.out
```

### Current Coverage

To see current test coverage by package:

```bash
go test ./... -cover
```

Example output:
```
ok      github.com/mmz-srf/akamai-operator/controllers  0.400s  coverage: 65.2% of statements
ok      github.com/mmz-srf/akamai-operator/pkg/akamai   0.252s  coverage: 72.8% of statements
```

## Writing New Tests

### Test Structure

Follow Go testing conventions:

```go
package mypackage

import (
    "testing"
)

func TestMyFunction(t *testing.T) {
    // Arrange
    input := "test input"
    expected := "expected output"
    
    // Act
    result := MyFunction(input)
    
    // Assert
    if result != expected {
        t.Errorf("MyFunction(%q) = %q, want %q", input, result, expected)
    }
}
```

### Table-Driven Tests

For multiple test cases:

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "normal case",
            input:    "hello",
            expected: "HELLO",
        },
        {
            name:     "empty string",
            input:    "",
            expected: "",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := MyFunction(tt.input)
            if result != tt.expected {
                t.Errorf("got %q, want %q", result, tt.expected)
            }
        })
    }
}
```

### Best Practices

1. **Use descriptive test names**: Test names should clearly describe what is being tested
2. **Test edge cases**: Include tests for empty inputs, nil values, boundary conditions
3. **Keep tests isolated**: Each test should be independent and not rely on others
4. **Use subtests**: Group related test cases using `t.Run()`
5. **Add comments**: Explain complex test scenarios
6. **Mock external dependencies**: Don't make actual API calls in unit tests

### Adding New Test Files

1. Create a file named `*_test.go` in the same package
2. Import the `testing` package
3. Write test functions starting with `Test`
4. Run with `go test ./...`

Example:
```bash
# Create new test file
touch pkg/akamai/myfeature_test.go

# Write tests
# ...

# Run all tests
go test ./...
```

## Continuous Integration

### GitHub Actions

Add a workflow file (`.github/workflows/test.yml`):

```yaml
name: Tests

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Run tests
      run: go test ./... -v -race -coverprofile=coverage.out
    
    - name: Upload coverage
      uses: codecov/codecov-action@v3
      with:
        files: ./coverage.out
```

### Pre-commit Hooks

Add to `.git/hooks/pre-commit`:

```bash
#!/bin/bash
echo "Running tests..."
go test ./... || exit 1
echo "All tests passed!"
```

Make it executable:
```bash
chmod +x .git/hooks/pre-commit
```

## Debugging Tests

### Verbose Output

```bash
# See detailed test output
go test ./... -v
```

### Run Specific Test with Debugging

```bash
# Add debug prints in your test
func TestMyFunction(t *testing.T) {
    t.Logf("Debug: input value = %v", input)
    // ... test code
}
```

### Test Timeout

```bash
# Set custom timeout
go test ./... -timeout 30s
```

### Fail Fast

```bash
# Stop on first failure
go test ./... -failfast
```

## Common Test Commands

| Command | Description |
|---------|-------------|
| `go test ./...` | Run all tests |
| `go test ./... -v` | Run with verbose output |
| `go test ./... -cover` | Run with coverage |
| `go test ./... -race` | Run with race detection |
| `go test ./pkg/akamai` | Run tests in specific package |
| `go test -run TestName` | Run specific test |
| `go test -short` | Skip long-running tests |
| `go test -count=1 ./...` | Disable test caching |
| `go test -bench=.` | Run benchmarks |

## Test Results Summary

Current test status (as of last run):

```
Package                                          Tests  Status
────────────────────────────────────────────────────────────────
github.com/mmz-srf/akamai-operator/controllers      6  PASS
github.com/mmz-srf/akamai-operator/pkg/akamai       2  PASS
────────────────────────────────────────────────────────────────
Total                                               8  PASS
```

### Test Breakdown

- **Hostname Tests**: 2 test functions, 12 subtests
- **Rules Tests**: 4 test functions, 11 subtests
- **Total**: 6 test functions, 23 subtests

All tests passing ✓

## Troubleshooting

### Tests Fail with "cannot find package"

```bash
# Download dependencies
go mod download
go mod tidy
```

### Tests Fail with Import Errors

```bash
# Update dependencies
go get -u ./...
go mod tidy
```

### Tests Pass Locally but Fail in CI

- Check Go version consistency
- Ensure all dependencies are committed
- Review environment-specific code

## Related Documentation

- [Development Guide](../README.md#development)
- [Hostname Management](HOSTNAME_MANAGEMENT.md)
- [Rules Management](RULESET_MANAGEMENT.md)
- [Contributing Guidelines](../CONTRIBUTING.md)

## Support

If you encounter issues with tests:

1. Check test output for specific error messages
2. Review test code to understand what's being validated
3. Run tests with `-v` flag for detailed output
4. Check [GitHub Issues](https://github.com/mmz-srf/akamai-operator/issues) for known problems
5. Open a new issue if the problem persists

---

**Last Updated:** November 6, 2025
