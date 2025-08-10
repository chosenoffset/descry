---
name: Bug report
about: Create a report to help us improve Descry
title: ''
labels: 'bug'
assignees: ''
---

**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. Go to '...'
2. Click on '....'
3. Scroll down to '....'
4. See error

**Expected behavior**
A clear and concise description of what you expected to happen.

**Rule/Code Sample**
If applicable, include the DSL rule or Go code that's causing the issue:

```dscr
when heap.alloc > 100MB {
    alert("High memory usage")
}
```

**Error Messages/Logs**
If applicable, add error messages or log output:

```
ERROR: rule parsing failed at line 2: unexpected token
```

**Environment (please complete the following information):**
- **Go version**: [e.g. 1.24.5] (`go version`)
- **Descry version**: [e.g. v0.2.0 or commit hash]
- **OS**: [e.g. Ubuntu 22.04, macOS 14.1, Windows 11]
- **Architecture**: [e.g. amd64, arm64]

**Descry Configuration**
- Are you using the dashboard? [Yes/No]
- Custom metrics? [Yes/No] 
- HTTP middleware? [Yes/No]
- Any custom alert handlers? [Yes/No]

**Additional context**
Add any other context about the problem here. Screenshots of dashboard issues are helpful if applicable.

**Workaround**
If you found a temporary workaround, please share it to help others with the same issue.