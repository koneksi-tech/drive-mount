# Koneksi Drive

A FUSE filesystem driver that allows you to mount Koneksi storage as a local filesystem on Linux and macOS.

## Features

- Mount Koneksi storage as a local filesystem
- Read and write files directly from/to Koneksi storage
- Directory operations (create, delete)
- File caching for improved performance
- Cross-platform support (Linux and macOS)
- Read-only mode option
- Configurable cache settings

## Requirements

### Linux
- FUSE 2.9+ installed
- Go 1.21+ (for building from source)

### macOS
- macFUSE installed (https://osxfuse.github.io/)
- Go 1.21+ (for building from source)

## Installation

### From Binary Release

1. Download the latest release for your platform from the [releases page](https://github.com/koneksi/koneksi-drive/releases)
2. Extract the archive: `tar -xzf koneksi-drive-<platform>-<arch>.tar.gz`
3. Move the binary to your PATH: `sudo mv koneksi-drive /usr/local/bin/`
4. Make it executable: `chmod +x /usr/local/bin/koneksi-drive`

### From Source

```bash
git clone https://github.com/koneksi/koneksi-drive.git
cd koneksi-drive
go build -o koneksi-drive .
sudo mv koneksi-drive /usr/local/bin/
```

## Configuration

Create a configuration file at `~/.koneksi-drive.yaml`:

```yaml
api:
  base_url: "https://your-koneksi-instance.com"
  client_id: "your-client-id"
  client_secret: "your-client-secret"
  directory_id: "your-directory-id"
  timeout: 30s
  retry_count: 3

mount:
  readonly: false      # Mount as read-only
  allow_other: false   # Allow other users to access the mount
  uid: 1000           # User ID for file ownership
  gid: 1000           # Group ID for file ownership
  umask: 0022         # Default umask for new files

cache:
  enabled: true
  directory: ""        # Cache directory (empty for temp dir)
  ttl: 5m             # Cache time-to-live
  max_size: 1073741824  # Max cache size in bytes (1GB)
```

## Usage

### Basic Mount

```bash
# Create a mount point
mkdir ~/koneksi-storage

# Mount Koneksi storage
koneksi-drive mount ~/koneksi-storage

# The filesystem is now mounted and accessible
ls ~/koneksi-storage
```

### Mount Options

```bash
# Mount as read-only
koneksi-drive mount --readonly ~/koneksi-storage

# Mount with custom config file
koneksi-drive mount --config /path/to/config.yaml ~/koneksi-storage

# Mount with caching disabled
koneksi-drive mount --cache-ttl 0 ~/koneksi-storage

# Mount allowing other users to access
koneksi-drive mount --allow-other ~/koneksi-storage
```

### Unmounting

To unmount the filesystem, press `Ctrl+C` in the terminal where koneksi-drive is running, or use:

```bash
# Linux
fusermount -u ~/koneksi-storage

# macOS
umount ~/koneksi-storage
```

## Performance Considerations

1. **Caching**: Enable caching for better performance with frequently accessed files
2. **Network Latency**: Performance depends on your network connection to the Koneksi server
3. **Large Files**: Streaming large files may be slower than local storage
4. **Concurrent Access**: Multiple processes can read/write simultaneously

## Troubleshooting

### Linux: "Transport endpoint is not connected"

This usually means the filesystem was not properly unmounted. Fix with:

```bash
fusermount -u ~/koneksi-storage
```

### macOS: "mount_macfuse: the file system is not available"

Ensure macFUSE is properly installed:

```bash
brew install --cask macfuse
# Restart your computer after installation
```

### Permission Denied

1. Check your Koneksi API credentials in the config file
2. Ensure the mount point directory exists and you have write permissions
3. Try running with `--debug` flag for more information

### Debug Mode

Run with debug output to troubleshoot issues:

```bash
koneksi-drive mount --debug ~/koneksi-storage
```

## Security Considerations

1. **Config File**: Keep your config file secure (chmod 600 ~/.koneksi-drive.yaml)
2. **API Credentials**: Never commit credentials to version control
3. **Mount Permissions**: Use appropriate uid/gid and umask settings
4. **Network**: Use HTTPS for API connections

## Building from Source

### Prerequisites

- Go 1.21 or later
- FUSE development headers
  - Linux: `sudo apt-get install libfuse-dev` (Debian/Ubuntu)
  - macOS: Install macFUSE

### Build Commands

```bash
# Clone the repository
git clone https://github.com/koneksi/koneksi-drive.git
cd koneksi-drive

# Download dependencies
go mod download

# Build for current platform
go build -o koneksi-drive .

# Build for specific platform
GOOS=linux GOARCH=amd64 go build -o koneksi-drive-linux-amd64 .
GOOS=darwin GOARCH=amd64 go build -o koneksi-drive-darwin-amd64 .
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [go-fuse](https://github.com/hanwen/go-fuse) - FUSE bindings for Go
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management