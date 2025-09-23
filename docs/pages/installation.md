---
title: Installation
---

# {% $markdoc.frontmatter.title %} {% #overview %}

Dewy is distributed as a single Go binary and operates without external dependencies. You can install it using the following methods.

## Prerequisites

Before using Dewy, please ensure the following requirements are met:

### System Requirements

- **Operating System**: Linux, macOS, Windows
- **Architecture**: amd64, arm64
- **Shell**: `/bin/sh` must be available (for hook execution)
- **Network**: Outbound connections to registry and artifact store

### File System Requirements

- **Write permissions**: Write access to working directory
- **Symbolic link support**: For creating `current` pointers
- **Temporary directory**: Access permissions for cache storage
- **Disk space**: Sufficient capacity for 7 generations of releases + cache (typically several hundred MB)

## Installation Methods

### 1. Download Pre-built Binary (Recommended)

Latest releases can be downloaded from [GitHub Releases](https://github.com/linyows/dewy/releases).

#### Manual Download

```bash
# Check latest release URL
LATEST_VERSION=$(curl -s https://api.github.com/repos/linyows/dewy/releases/latest | grep '"tag_name"' | cut -d '"' -f 4)

# Download binary for your architecture (Linux amd64 example)
wget https://github.com/linyows/dewy/releases/download/${LATEST_VERSION}/dewy_linux_amd64.tar.gz

# Extract and install
tar -xzf dewy_linux_amd64.tar.gz
sudo mv dewy /usr/local/bin/
chmod +x /usr/local/bin/dewy
```

#### Platform-Specific Binaries

| OS | Architecture | Filename |
|---|---|---|
| Linux | amd64 | `dewy_linux_amd64.tar.gz` |
| Linux | arm64 | `dewy_linux_arm64.tar.gz` |
| macOS | amd64 | `dewy_darwin_amd64.tar.gz` |
| macOS | arm64 (Apple Silicon) | `dewy_darwin_arm64.tar.gz` |
| Windows | amd64 | `dewy_windows_amd64.zip` |

### 2. Build from Source

Go 1.21 or later is required.

```bash
# Clone repository
git clone https://github.com/linyows/dewy.git
cd dewy

# Get dependencies
go mod download

# Build
go build -o dewy

# Install to system directory
sudo mv dewy /usr/local/bin/
```

#### Development Build

```bash
# Install directly from latest main branch
go install github.com/linyows/dewy@latest
```

### 3. Provisioning Tools

For large-scale production deployment, you can use the following provisioning tools.

#### Chef

Installation using Chef Cookbook:

```bash
# Get Cookbook
# See https://github.com/linyows/dewy-cookbook
```

Chef Recipe example:

```ruby
# Install dewy
dewy 'myapp' do
  registry 'ghr://myorg/myapp'
  notifier 'slack://deployments?title=myapp'
  ports ['8000']
  log_level 'info'
  action :install
end

# Configure as systemd service
systemd_unit 'dewy-myapp.service' do
  content <<~EOS
    [Unit]
    Description=Dewy - myapp deployment manager
    After=network.target

    [Service]
    Type=simple
    User=deploy
    ExecStart=/usr/local/bin/dewy server --registry ghr://myorg/myapp \\
              --notifier slack://deployments?title=myapp \\
              --port 8000 --log-level info \\
              -- /opt/myapp/current/myapp
    Restart=always
    RestartSec=5

    [Install]
    WantedBy=multi-user.target
  EOS
  action [:create, :enable, :start]
end
```

#### Puppet

Installation using Puppet Module:

```bash
# Get Puppet Module
# See https://github.com/takumakume/puppet-dewy
```

Puppet manifest example:

```puppet
# Install dewy
class { 'dewy':
  version => '1.2.3',
  install_method => 'binary',
}

# Application configuration
dewy::app { 'myapp':
  registry => 'ghr://myorg/myapp',
  notifier => 'slack://deployments?title=myapp',
  ports    => ['8000'],
  log_level => 'info',
  command  => '/opt/myapp/current/myapp',
  user     => 'deploy',
  group    => 'deploy',
}
```

## Installation Verification

Verify that installation completed successfully:

```bash
# Check version
dewy --version

# Display help
dewy --help

# Basic operation check
dewy server --registry ghr://linyows/dewy --help
```

## Next Steps

After installation is complete, refer to the following documentation to begin configuration:

- [Getting Started](./getting-started.md)
- [Architecture](./architecture.md)
- [FAQ](./faq.md)

## Troubleshooting

### Common Issues

#### Permission Errors

```bash
# When there's no write permission to /usr/local/bin
sudo chmod 755 /usr/local/bin
sudo chown root:root /usr/local/bin/dewy

# Or install to user directory
mkdir -p ~/bin
mv dewy ~/bin/
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

#### Symbolic Link Errors

```bash
# Check if filesystem supports symbolic links
ln -s /tmp/test /tmp/testlink && rm /tmp/testlink || echo "Symlinks not supported"
```

#### Network Connection Issues

```bash
# Check connection to GitHub API
curl -s https://api.github.com/repos/linyows/dewy/releases/latest

# For proxy environments
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
```

### Logging and Debug

```bash
# Run with debug logging enabled
dewy server --registry ghr://owner/repo --log-level debug --log-format json

# Check system logs (when using systemd)
journalctl -u dewy-myapp.service -f
```

If issues persist, please seek support at [GitHub Issues](https://github.com/linyows/dewy/issues).