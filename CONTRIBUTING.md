# Contributing to Descry

Thank you for your interest in contributing to Descry! This document provides guidelines for contributing to the Descry rules engine project.

## Development Setup

### Prerequisites

- **Go 1.24.5 or higher** - Descry requires modern Go features
- **Git** - For version control and collaboration
- **Make** - For build automation (optional but recommended)

### Installation

1. **Fork and Clone**
   ```bash
   git clone https://github.com/your-username/descry.git
   cd descry
   ```

2. **Install Dependencies**
   ```bash
   go mod download
   go mod tidy
   ```

3. **Verify Setup**
   ```bash
   # Build everything
   go build ./...
   
   # Run tests
   go test ./...
   
   # Run example application
   go run descry-example/cmd/server/main.go
   ```

### Development Tools

- **Code Formatting**: Use `go fmt ./...` before committing
- **Linting**: Install and run `golangci-lint run` (recommended)
- **Testing**: Use `go test -cover ./...` for coverage reports

## Code Style Guidelines

### Go Standards
- Follow standard Go formatting with `go fmt`
- Use meaningful variable and function names
- Add comments for exported functions and types
- Keep functions focused and concise
- Use Go modules for dependency management

### Project-Specific Guidelines
- **DSL Rules**: Place example rules in `descry-example/rules/`
- **Tests**: Include tests for new functionality
- **Documentation**: Update relevant docs when adding features
- **Error Handling**: Always handle errors explicitly
- **Logging**: Use structured logging for debug information

### Code Organization
```
pkg/descry/           # Core library code
├── engine.go         # Main engine implementation
├── parser/           # DSL parsing logic
├── metrics/          # Metric collection
├── dashboard/        # Web dashboard
└── actions/          # Rule action handlers

descry-example/       # Example application
├── cmd/              # Command-line tools
├── internal/         # Example-specific code
└── rules/            # Sample DSL rules
```

## Testing Guidelines

### Running Tests
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test package
go test ./pkg/descry/parser

# Run tests with verbose output
go test -v ./...
```

### Port Conflict Workarounds
Our integration tests bind to port 9090, which can cause conflicts:

1. **Ensure port 9090 is free** before running tests
2. **Run tests sequentially** if experiencing port conflicts:
   ```bash
   go test -p 1 ./...
   ```
3. **Stop any running Descry instances** before testing

### Writing Tests
- Include unit tests for new functions
- Add integration tests for end-to-end scenarios
- Test error conditions and edge cases
- Use table-driven tests for multiple input scenarios
- Mock external dependencies appropriately

## Pull Request Process

### Before Submitting
1. **Fork** the repository
2. **Create a feature branch** from main: `git checkout -b feature/your-feature`
3. **Make your changes** following the code style guidelines
4. **Add or update tests** for your changes
5. **Update documentation** if needed
6. **Run the full test suite** and ensure it passes
7. **Format your code** with `go fmt ./...`
8. **Commit your changes** with descriptive commit messages

### Commit Message Format
Use clear, descriptive commit messages:
```
Add memory leak detection to DSL engine

- Implement heap trend analysis function
- Add memory threshold rule examples
- Update DSL reference documentation
```

### Pull Request Guidelines
- **Title**: Clearly describe what the PR does
- **Description**: Include context, changes made, and testing done
- **Link Issues**: Reference any related issues with "Fixes #123"
- **Small PRs**: Keep changes focused and reviewable
- **Documentation**: Update relevant documentation
- **Tests**: Ensure all tests pass

### Review Process
1. **Automated checks** must pass (tests, linting)
2. **Code review** by maintainers
3. **Address feedback** promptly and respectfully
4. **Squash and merge** after approval

## Issue Reporting

### Bug Reports
Use the bug report template and include:
- **Go version** (`go version`)
- **Descry version** or commit hash
- **Operating system** and architecture
- **Steps to reproduce** the issue
- **Expected vs actual behavior**
- **Logs or error messages**
- **Sample code** if applicable

### Feature Requests
Use the feature request template and include:
- **Use case description** and motivation
- **Proposed solution** or API design
- **Alternatives considered**
- **Examples** of how it would be used
- **Breaking changes** if any

### Questions and Discussions
For general questions:
- Check existing issues and documentation first
- Use GitHub Discussions for open-ended questions
- Join our community chat (if available)

## Development Workflow

### Typical Development Cycle
1. **Pick an issue** from the issue tracker
2. **Discuss approach** if it's a significant change
3. **Create feature branch**
4. **Implement changes** with tests
5. **Test locally** including example application
6. **Submit pull request**
7. **Address review feedback**
8. **Merge when approved**

### Working with Examples
The `descry-example/` directory demonstrates real-world usage:
- **Test changes** with the example application
- **Update rules** if adding new DSL features
- **Verify dashboard integration** when making UI changes
- **Run load tests** with `descry-example/cmd/fuzz/main.go`

## Documentation

### Updating Documentation
- **README.md**: Update for major changes or new features
- **docs/**: Update relevant documentation files
- **Code comments**: Keep inline documentation current
- **Examples**: Add examples for new features

### Documentation Standards
- Use clear, concise language
- Include code examples where helpful
- Keep documentation up-to-date with code changes
- Test all code samples before committing

## Community Guidelines

### Communication
- Be respectful and inclusive in all interactions
- Use constructive feedback and suggestions
- Help newcomers and answer questions when possible
- Follow our [Code of Conduct](CODE_OF_CONDUCT.md)

### Getting Help
- **Documentation**: Check docs/ directory first
- **Issues**: Search existing issues before creating new ones
- **Discussions**: Use GitHub Discussions for questions
- **Email**: Contact maintainers for security issues

### Recognition
Contributors will be acknowledged in:
- Git commit history
- Release notes for significant contributions
- Contributors section (if added)

## Security

### Reporting Security Issues
- **Do not** create public issues for security vulnerabilities
- **Email** security concerns to maintainers privately
- **Follow** responsible disclosure practices
- **See** SECURITY.md for detailed reporting process

### Security Considerations
- Descry executes user-defined rules in a sandboxed environment
- Be cautious with new DSL functions that access system resources
- Validate all inputs and sanitize outputs
- Consider performance impact of new features

## Release Process

### Version Numbering
Descry follows semantic versioning (semver):
- **Major** (X.0.0): Breaking changes
- **Minor** (0.X.0): New features, backward compatible
- **Patch** (0.0.X): Bug fixes, backward compatible

### Release Preparation
Maintainers handle releases, but contributors can help by:
- Keeping CHANGELOG.md updated
- Testing release candidates
- Updating version numbers consistently
- Verifying documentation accuracy

## License

By contributing to Descry, you agree that your contributions will be licensed under the same license as the project. Make sure you have the right to license your contributions.

## Contact

- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and community discussion
- **Project Maintainers**: Check README.md for current maintainer contacts

Thank you for contributing to Descry! Your contributions help make monitoring and debugging easier for Go developers everywhere.