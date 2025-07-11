// Test script for the logging system
package main

import (
	"fmt"
	"os"
	"time"

	sysutil "github.com/MAHDTech/nixos-installer/pkg/sysutil"
)

func main() {
	// Initialize logger
	if err := sysutil.InitLogger("test.log", sysutil.INFO, sysutil.DEBUG); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := sysutil.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
		}
	}()

	// Test different log levels
	sysutil.Debug("This is a debug message")
	sysutil.Info("This is an info message")
	sysutil.Warn("This is a warning message")
	sysutil.Error("This is an error message")

	// Test progress bar
	progress := sysutil.NewProgress("Test Progress", 10)
	for i := 0; i < 10; i++ {
		progress.Update(i + 1)
		// Simulate some work
		time.Sleep(100 * time.Millisecond)
	}
	progress.Complete()

	// Test success/failure messages
	sysutil.Success("Operation completed successfully!")
	sysutil.Failure("Operation failed!")

	// Test sections
	sysutil.Section("Test Section")
	sysutil.SubSection("Test Subsection")

	fmt.Println("Logging test completed. Check test.log for file output.")
}
