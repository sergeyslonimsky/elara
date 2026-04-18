# Elara UI - Configuration Manager Web Interface

React-based web interface for managing configurations with Ant Design components.

## Features

- 📋 **Configuration List**: View, search, and manage all configurations
- 📝 **Configuration Editor**: Create and edit JSON/YAML configurations
- 🌳 **Path Tree**: Hierarchical view of configuration paths
- ✅ **Content Validation**: Real-time validation of JSON/YAML content
- 🏷️ **Namespace Support**: Organize configurations by namespaces
- 📱 **Responsive Design**: Works on desktop and mobile devices

## Getting Started

### Prerequisites

- Node.js 18+
- npm or yarn
- Elara backend service running on port 8080

### Installation

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build
```

### Development

The development server runs on `http://localhost:3000` and proxies API requests to the backend service on `http://localhost:8080`.

### Configuration

The app connects to the Elara backend via the following endpoints:

- `GET /api/v1/configs` - List configurations
- `POST /api/v1/configs` - Create configuration
- `GET /api/v1/configs/{path}` - Get configuration
- `PUT /api/v1/configs/{path}` - Update configuration
- `DELETE /api/v1/configs/{path}` - Delete configuration
- `GET /api/v1/tree` - Get path tree
- `POST /api/v1/validate` - Validate configuration content

## Usage

### Managing Configurations

1. **List View**: Browse all configurations with search and filtering
2. **Create New**: Click "New Configuration" to create a new config
3. **Edit Existing**: Click the edit icon on any configuration
4. **Tree View**: Navigate the "Path Tree" to see hierarchical organization
5. **Validation**: Use the "Validate Content" button to check JSON/YAML syntax

### Features

- **Search**: Filter configurations by path prefix
- **Namespaces**: Filter and organize by namespace
- **Formats**: Support for JSON, YAML, and YML formats
- **Metadata**: Add custom key-value metadata to configurations
- **Validation**: Real-time content validation with error reporting

## Build

```bash
npm run build
```

The built files will be in the `dist/` directory and can be served by any static file server or integrated with the Go backend.
