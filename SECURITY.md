# Security Policy

## Supported Versions

We release patches for security vulnerabilities in the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 0.2.x   | :white_check_mark: |
| < 0.2   | :x:                |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them responsibly by emailing **chosenoffset@gmail.com** with the subject line "Descry Security Issue".

### What to Include

Please include the following information in your report:

- **Description**: Clear description of the issue
- **Impact**: What could happen if this issue is exploited
- **Steps to Reproduce**: How to reproduce the issue
- **Affected Components**: Which parts of Descry are affected
- **Environment**: Go version, OS, Descry version
- **Suggested Fix**: If you have ideas for resolution

## Response Timeline

- **Acknowledgment**: We will respond within **48 hours**
- **Initial Assessment**: We will provide assessment within **5 business days**
- **Status Updates**: Regular updates every **7 days**
- **Resolution**: We aim to resolve issues within **30 days**

## Security Best Practices

### For Users
- Only load DSL rules from trusted sources
- Restrict dashboard access to trusted networks
- Use HTTPS in production environments
- Keep Descry updated to the latest version

### Built-in Protections
- DSL rules execute in a sandboxed environment
- Input validation on all user-provided data
- Resource limits prevent excessive resource usage
- No network or filesystem access from rules

## Contact Information

- **Security Email**: chosenoffset@gmail.com
- **General Issues**: [GitHub Issues](https://github.com/chosenoffset/descry/issues)
- **Discussion**: [GitHub Discussions](https://github.com/chosenoffset/descry/discussions)

Thank you for helping keep Descry secure!