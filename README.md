# APM (Alloy Package Manager)

A simple CLI tool for managing packages on Alloy Linux and other NixOS-based systems. 

### Installation
```bash
# install it imperatively as the current user (discouraged)
nix profile install github:Alloy-Linux/apm_go
```

### Declarative Installation (Recommended)


```nix
# flake.nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";
    unstable.url = "github:NixOS/nixpkgs/nixos-unstable";

    # add the apm repo
    apm.url = "github:Alloy-Linux/apm_go";
    apm.packages.${system}.default
  };


}
```

# Basic Usage

### install using flakes (recommended)



```nix
inputs = {
   nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";
   unstable.url = "github:NixOS/nixpkgs/nixos-unstable";

   # add the apm repo
   apm.url = "github:Alloy-Linux/apm_go";

};
```

### Add the package your preferred way
```nix
# install using home-manager
home.packages = [ 
   apm.packages.{system}.default
];
```
### or

```nix
# install system-wide
environment.systemPackages = [
   apm.packages.{system}.default
];

```

### Basic Usage

After adding APM to your flake and rebuilding, you can start using it:

```bash
# Set up your flake location (point to your Nix config directory)
apm set-flake-location /path/to/your/nix-config

# Build the package cache (first-time setup - downloads ~100k packages)
apm makecache

# Add a package (automatically uses Home Manager)
apm add firefox

# List installed packages
apm list --home-manager

# Rebuild your system with the new packages
apm rebuild
```

## Commands

### Package Management
# Add a package to your configuration
- **`add [package]`** - Add a package to your configuration
  - `--home-manager` - Add to Home Manager packages (default)
  - `--nix-env` - Add to Nix environment packages
  - `--flatpak` - Add Flatpak application
  - `--unstable` - Install from unstable channel
  - `--exact` - Install exact package name (skip search, useful for known packages or scripts)

- **`list`** - Show installed packages
  - `--home-manager` - List Home Manager packages
  - `--nix-env` - List Nix environment packages
  - `--flatpak` - List Flatpak applications

### Configuration Management
- **`set-flake-location [path]`** - Set the path to your Nix flake configuration directory

- **`add-input [name] [url]`** - Add a new input to your flake.nix (like adding repositories)

- **`list-inputs`** - Show all inputs defined in your flake configuration

- **`list-modules`** - Show available modules from your flake inputs

### Cache Management
- **`makecache`** - Build/update the local package database (contains 100k+ packages)

- **`removecache`** - Clear the package cache

### Environment Setup
- **`makenixenv`** - Create Nix environment structure and packages file

- **`makehomeenv`** - Create Home Manager packages file

- **`setupflatpak`** - Add Flatpak module to your flake configuration

### Information
- **`show-nixpkgs-version`** - Display the current nixpkgs version in your flake


## How It Works

### Package Installation Methods

1. **Home Manager** (`--home-manager` or default)
   - Manages user-specific packages
   - Packages are defined in `packages/home-packages.nix`
   - **This is the default method when no flag is specified**
   - Example: `apm add firefox` (automatically uses Home Manager)

2. **Nix Environment** (`--nix-env`)
   - Manages system-wide packages
   - Packages are defined in system configuration
   - Example: `apm add htop --nix-env`

3. **Flatpak** (`--flatpak`)
   - Manages sandboxed applications
   - Uses Flatpak repositories
   - Example: `apm add org.mozilla.firefox --flatpak`

### Package Cache

- Stores package information in a local SQLite database
- Contains metadata for 100k+ packages from Nixpkgs
- Enables fast package searching and validation
- Located at `~/.config/apm/cache.db`


## Examples

### Adding Packages
```bash
# Add Firefox to Home Manager (default method)
apm add firefox

# Or explicitly specify Home Manager
apm add firefox --home-manager

# Add from unstable channel
apm add neovim --unstable

# Install exact package name (skip search)
apm add git --exact

# Add development tools
apm add vscode
apm add git --home-manager

# Rebuild your system
apm rebuild
```

### Managing Configuration
```bash
# Set your flake location
apm set-flake-location ~/projects/nix-config

# Add a new flake input
apm add-input home-manager github:nix-community/home-manager

# Check current nixpkgs version
apm show-nixpkgs-version
```

### Listing and Managing
```bash
# See all Home Manager packages
apm list --home-manager

# See all system packages
apm list --nix-env

# Update package cache
apm makecache
```


## License

This project is licensed under the GPLv3 license

## Troubleshooting

### Common Issues

**"Package not found in Nixpkgs"**
- Run `apm makecache` to update the package database
- Check package name spelling

**"Flake location not set"**
- Run `apm set-flake-location /path/to/your/flake`

**"Permission denied"**
- Ensure you have write access to your Nix configuration directory

### Getting Help

- Check `apm --help` for command usage
- Run `apm [command] --help` for specific command help
- Verify your Nix flake configuration is valid

---

