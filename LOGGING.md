# NixOS Installer Logging System

The NixOS Installer now features a comprehensive logging system that provides both file logging and user-friendly console output.

## Features

### Dual Output System
- **File Logging**: All log messages are written to `nixos-installer.log` with timestamps and log levels
- **Console Output**: User-friendly colored output with progress indicators and summary information

### Log Levels
- **DEBUG**: Detailed debug information (file only by default)
- **INFO**: General information messages (default console level)
- **WARN**: Warning messages
- **ERROR**: Error messages only

### Progress Tracking
- Real-time progress bars with ETA
- Phase-by-phase progress tracking
- Success/failure indicators

### Colored Output
- Green: Success messages and INFO level
- Yellow: Warning messages
- Red: Error messages and failures
- Blue: Section headers and progress bars
- Cyan: Subsection headers and DEBUG level

## Command Line Options

```bash
# Basic usage
./nixos-installer -config=my-config.yaml

# With custom logging
./nixos-installer -config=my-config.yaml -log-file=/var/log/nixos-installer.log

# Control log levels
./nixos-installer -console-level=WARN -file-level=DEBUG

# Show help
./nixos-installer -help
```

### Log Options
- `-log-file`: Path to log file (default: `nixos-installer.log`)
- `-console-level`: Console output level (DEBUG, INFO, WARN, ERROR)
- `-file-level`: File logging level (DEBUG, INFO, WARN, ERROR)

## Example Output

### Console Output (INFO level)
```
=== NixOS Installation Process ===
[2024-01-15 10:30:00] INFO: Starting NixOS Installer...
[2024-01-15 10:30:00] INFO: Reading and validating configuration from config.yaml
✓ Configuration loaded successfully

--- Tool Availability Check ---
Installation Progress [██████████████████████████████] 6/6 (100.0%) 2m30s/0s
✓ All required tools are available

--- Preparation Phase ---
[2024-01-15 10:30:05] INFO: Checking current mountpoints
[2024-01-15 10:30:06] INFO: Creating necessary directories
✓ Directories created successfully
[2024-01-15 10:30:07] INFO: Unmounting existing partitions
✓ Partitions unmounted successfully
✓ Preparation phase completed

✓ NixOS installation process completed successfully
```

### File Log Output (DEBUG level)
```
[2024-01-15 10:30:00] INFO: Starting NixOS Installer...
[2024-01-15 10:30:00] DEBUG: Running in dry run mode, see '-help' for more information
[2024-01-15 10:30:00] INFO: Reading and validating configuration from config.yaml
[2024-01-15 10:30:01] DEBUG: Found tool: zfs
[2024-01-15 10:30:01] DEBUG: Found tool: zpool
[2024-01-15 10:30:01] DEBUG: Found tool: sgdisk
[2024-01-15 10:30:01] DEBUG: Found tool: wipefs
[2024-01-15 10:30:01] DEBUG: Found tool: mount
[2024-01-15 10:30:01] DEBUG: Found tool: umount
[2024-01-15 10:30:01] DEBUG: Found tool: lsblk
[2024-01-15 10:30:01] DEBUG: Found tool: readlink
[2024-01-15 10:30:01] DEBUG: Found tool: partprobe
[2024-01-15 10:30:01] DEBUG: Found tool: udevadm
[2024-01-15 10:30:01] DEBUG: Found tool: dd
[2024-01-15 10:30:01] DEBUG: Found tool: chmod
[2024-01-15 10:30:01] INFO: All required tools are available
[2024-01-15 10:30:02] INFO: Starting installation phases...
[2024-01-15 10:30:02] INFO: Checking current mountpoints
[2024-01-15 10:30:02] DEBUG: EXECUTING: /usr/bin/lsblk --noheadings --json --output ID,MOUNTPOINTS
[2024-01-15 10:30:03] INFO: Creating necessary directories
[2024-01-15 10:30:03] DEBUG: EXECUTING: /usr/bin/mkdir -p /mnt/nixos
[2024-01-15 10:30:03] INFO: Unmounting existing partitions
[2024-01-15 10:30:03] DEBUG: EXECUTING: /usr/bin/zpool list -H -o name
[2024-01-15 10:30:04] INFO: Directories created successfully
[2024-01-15 10:30:04] INFO: Partitions unmounted successfully
[2024-01-15 10:30:04] INFO: Preparation phase completed
```

## Implementation Details

### Logger Structure
The logging system is implemented in `pkg/sysutil/log.go` with the following components:

- `Logger`: Main logger struct with file and console output
- `LogLevel`: Enum for log levels (DEBUG, INFO, WARN, ERROR)
- `Progress`: Progress bar implementation with ETA calculation
- Global convenience functions for easy access

### Integration
The logging system is integrated throughout the installer:

1. **Main entry point**: Initializes logger with command-line options
2. **Installation phases**: Each phase uses appropriate log levels
3. **Command execution**: All command outputs are logged at DEBUG level
4. **Error handling**: Errors are logged at ERROR level with context
5. **Progress tracking**: Real-time progress bars for long operations

### Best Practices
- Use `DEBUG` for detailed technical information
- Use `INFO` for general progress and status updates
- Use `WARN` for non-fatal issues that should be noted
- Use `ERROR` for actual errors that may affect the installation
- Use `Success()` and `Failure()` for clear user feedback
- Use `Section()` and `SubSection()` for organizing output
- Use `Progress` for long-running operations

## Testing

You can test the logging system using the provided test script:

```bash
go run test_logging.go
```

This will demonstrate all logging features and create a `test.log` file with detailed output. 