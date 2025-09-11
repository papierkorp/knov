# Troubleshooting Guide

This guide helps resolve common issues you might encounter.

## Common Issues

### 1. Search Not Working

**Symptoms**: No results returned from search
**Solution**:

1. Check if metadata is properly indexed
2. Verify search syntax in [[technical-documentation.md#api-endpoints]]
3. See [[meeting-notes.md]] for recent search improvements

### 2. Links Not Resolving

**Symptoms**: `[[links]]` appear as plain text
**Solutions**:

- Ensure target file exists
- Check file path syntax
- Verify in [[getting-started.md#quick-links]] for examples

### 3. Performance Issues

| Issue          | Cause           | Solution              |
| -------------- | --------------- | --------------------- |
| Slow loading   | Large files     | Optimize file size    |
| Memory usage   | Too many files  | Implement pagination  |
| Search timeout | Complex queries | Simplify search terms |

### Debug Commands

    # Check system status
    curl http://localhost:1324/api/health

    # Test search functionality
    curl -X POST http://localhost:1324/api/files/filter \
      -d "metadata[]=tags&operator[]=contains&value[]=important"

    # Verify git status
    git status
    git log --oneline -5

### Getting Help

1. Check this troubleshooting guide first
2. Review [[technical-documentation.md]] for API details
3. Look at recent [[meeting-notes.md]] for known issues
4. Contact the team if issue persists

> **Tip**: Most issues are documented in our [[project-overview.md|Project Overview]] or recent meeting notes.

### Related Documentation

- [[getting-started.md]] - Basic setup
- [[technical-documentation.md]] - Technical details
- [[meeting-notes.md]] - Recent changes and decisions
- [[guides/user-manual.md]] - User guide
