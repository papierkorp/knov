# KNOV - Knowledge Management System

KNOV is a file-based knowledge management system that organizes markdown files with metadata and provides flexible viewing through boards (dashboards).

## Core Concepts

### Files

- **All file types** stored in the filesystem
- **Git-backed** for version control and history
- **Metadata** automatically generated for all files
- **Links** between markdown files are tracked and indexed

### Collections

- **Folder-based organization** - root folders become collections
- **Automatic detection** - files inherit collection from their location
- **Default collection** for root-level files
- Examples: `personal/`, `work/`, `projects/`

### Metadata

Each file has associated metadata:

- **Basic info**: name, path, size, dates
- **Organization**: collection, folders, tags, boards
- **Relationships**: parents, kids, ancestors, links
- **Classification**: type (note/todo/journal), status, priority

### Boards (Dashboards)

- **Filtered views** of files based on metadata criteria
- **Multiple display types**: list, cards, content preview
- **Flexible layouts**: single column, multi-column, grid
- **Saved configurations** - users can create and switch between boards

### Search

- **Full-text search** across all files
- **Multiple engines**: SQLite (persistent), in-memory (fast), grep (simple)
- **Metadata filtering** combined with content search

## Architecture

### Data Storage

```
data/                   # Git repository with markdown files
├── collection1/        # Collection folders
│   ├── file1.md
│   └── subfolder/
│       └── file2.md
├── collection2/
│   └── file3.md
└── root-file.md        # Default collection

config/
├── .metadata/          # Metadata storage (JSON/SQLite)
├── users/             # User settings
└── custom.css         # Customizations
```

### Core Components

- **File Management** - read, parse, convert markdown to HTML
- **Metadata System** - auto-generate and maintain file relationships
- **Search Engine** - index and query files
- **Theme System** - pluggable UI themes
- **API Layer** - RESTful endpoints for all operations

## User Workflow

1. **Create/Edit** markdown files in collections
2. **Organize** with tags, status, priority via metadata
3. **Create boards** to view filtered sets of files
4. **Search** across content and metadata
5. **Navigate** through file relationships (links, parents, kids)

## Key Features

- **Git Integration** - automatic versioning and history
- **Flexible Organization** - collections + boards + metadata
- **Powerful Search** - content + metadata filtering
- **Link Management** - automatic detection and tracking
- **Multi-theme Support** - customizable UI
- **API-First** - all features accessible via REST API
