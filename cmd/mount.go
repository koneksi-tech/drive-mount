package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/koneksi/koneksi-drive/internal/config"
	"github.com/koneksi/koneksi-drive/internal/fs"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var mountCmd = &cobra.Command{
	Use:   "mount [mountpoint]",
	Short: "Mount Koneksi storage to a local directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mountpoint := args[0]

		// Ensure mountpoint exists
		if err := os.MkdirAll(mountpoint, 0755); err != nil {
			return fmt.Errorf("failed to create mountpoint: %w", err)
		}

		// Convert to absolute path
		absMount, err := filepath.Abs(mountpoint)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Create and mount filesystem
		kfs, err := fs.NewKoneksiFS(cfg)
		if err != nil {
			return fmt.Errorf("failed to create filesystem: %w", err)
		}

		fmt.Printf("Mounting Koneksi storage at %s...\n", absMount)
		
		if err := kfs.Mount(absMount); err != nil {
			return fmt.Errorf("failed to mount filesystem: %w", err)
		}

		fmt.Println("Filesystem mounted successfully. Press Ctrl+C to unmount.")

		// Wait for interrupt signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		fmt.Println("\nUnmounting filesystem...")
		if err := kfs.Unmount(); err != nil {
			return fmt.Errorf("failed to unmount: %w", err)
		}

		fmt.Println("Filesystem unmounted successfully.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mountCmd)
	
	mountCmd.Flags().Bool("readonly", false, "Mount filesystem as read-only")
	mountCmd.Flags().Bool("allow-other", false, "Allow other users to access the filesystem")
	mountCmd.Flags().String("cache-dir", "", "Directory for caching files (default: temp dir)")
	mountCmd.Flags().Duration("cache-ttl", 0, "Cache time-to-live (0 to disable caching)")
	
	viper.BindPFlag("mount.readonly", mountCmd.Flags().Lookup("readonly"))
	viper.BindPFlag("mount.allow_other", mountCmd.Flags().Lookup("allow-other"))
	viper.BindPFlag("mount.cache_dir", mountCmd.Flags().Lookup("cache-dir"))
	viper.BindPFlag("mount.cache_ttl", mountCmd.Flags().Lookup("cache-ttl"))
}