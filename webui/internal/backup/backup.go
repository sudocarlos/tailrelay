package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
)

// Manager handles backup and restore operations
type Manager struct {
	cfg *config.Config
}

// NewManager creates a new backup manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg: cfg,
	}
}

// Create creates a full backup of configurations and certificates
func (m *Manager) Create(backupType string) (string, error) {
	// Ensure backup directory exists
	if err := os.MkdirAll(m.cfg.Paths.BackupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	filename := fmt.Sprintf("tailrelay-backup-%s-%s.tar.gz", hostname, timestamp)
	backupPath := filepath.Join(m.cfg.Paths.BackupDir, filename)

	// Create tar.gz file
	file, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add metadata file
	metadata := config.BackupMetadata{
		Timestamp:  time.Now(),
		Version:    "0.2.0",
		Hostname:   hostname,
		BackupType: backupType,
	}

	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := addFileToTar(tarWriter, "metadata.json", metadataJSON); err != nil {
		return "", fmt.Errorf("failed to add metadata: %w", err)
	}

	// Add configuration files
	filesToBackup := []string{
		m.cfg.Paths.CaddyConfig,
		m.cfg.Paths.SocatRelayConfig,
	}

	for _, filePath := range filesToBackup {
		if filePath == "" {
			continue
		}

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue // Skip non-existent files
		}

		if err := addFilePathToTar(tarWriter, filePath, filepath.Base(filePath)); err != nil {
			return "", fmt.Errorf("failed to add file %s: %w", filePath, err)
		}
	}

	// Add certificates directory if it exists
	if m.cfg.Paths.CertificatesDir != "" {
		if info, err := os.Stat(m.cfg.Paths.CertificatesDir); err == nil && info.IsDir() {
			if err := addDirectoryToTar(tarWriter, m.cfg.Paths.CertificatesDir, "certificates"); err != nil {
				return "", fmt.Errorf("failed to add certificates: %w", err)
			}
		}
	}

	return backupPath, nil
}

// Restore restores a backup from a tar.gz file
func (m *Manager) Restore(backupPath string) error {
	// Open backup file
	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		// Determine target path
		var targetPath string
		switch {
		case header.Name == "metadata.json":
			// Skip metadata, just for info
			continue
		case strings.HasPrefix(header.Name, "certificates/"):
			// Restore to certificates directory
			relPath := strings.TrimPrefix(header.Name, "certificates/")
			targetPath = filepath.Join(m.cfg.Paths.CertificatesDir, relPath)
		case strings.HasSuffix(header.Name, "Caddyfile"):
			targetPath = m.cfg.Paths.CaddyConfig
		case strings.HasSuffix(header.Name, "relays.json"):
			targetPath = m.cfg.Paths.SocatRelayConfig
		default:
			// Unknown file, skip
			continue
		}

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
		}

		// Extract file or directory
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		} else {
			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			outFile.Close()
		}
	}

	return nil
}

// List returns a list of available backups
func (m *Manager) List() ([]config.BackupInfo, error) {
	// Check if backup directory exists
	if _, err := os.Stat(m.cfg.Paths.BackupDir); os.IsNotExist(err) {
		return []config.BackupInfo{}, nil
	}

	files, err := os.ReadDir(m.cfg.Paths.BackupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	backups := []config.BackupInfo{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if !strings.HasSuffix(file.Name(), ".tar.gz") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		// Try to read metadata
		metadata, _ := m.ReadMetadata(filepath.Join(m.cfg.Paths.BackupDir, file.Name()))

		backups = append(backups, config.BackupInfo{
			Filename:  file.Name(),
			Size:      info.Size(),
			Timestamp: info.ModTime(),
			Metadata:  metadata,
		})
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// Delete deletes a backup file
func (m *Manager) Delete(filename string) error {
	backupPath := filepath.Join(m.cfg.Paths.BackupDir, filename)

	// Security check: ensure the file is in the backup directory
	absBackupPath, err := filepath.Abs(backupPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	absBackupDir, err := filepath.Abs(m.cfg.Paths.BackupDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute backup dir: %w", err)
	}

	if !strings.HasPrefix(absBackupPath, absBackupDir) {
		return fmt.Errorf("invalid backup path")
	}

	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	return nil
}

// ReadMetadata reads metadata from a backup file
func (m *Manager) ReadMetadata(backupPath string) (config.BackupMetadata, error) {
	file, err := os.Open(backupPath)
	if err != nil {
		return config.BackupMetadata{}, err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return config.BackupMetadata{}, err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	// Look for metadata.json
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return config.BackupMetadata{}, err
		}

		if header.Name == "metadata.json" {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return config.BackupMetadata{}, err
			}

			var metadata config.BackupMetadata
			if err := json.Unmarshal(data, &metadata); err != nil {
				return config.BackupMetadata{}, err
			}

			return metadata, nil
		}
	}

	return config.BackupMetadata{}, fmt.Errorf("metadata not found")
}

// CleanupOldBackups removes old backups keeping only the specified number
func (m *Manager) CleanupOldBackups(keepCount int) error {
	backups, err := m.List()
	if err != nil {
		return err
	}

	if len(backups) <= keepCount {
		return nil // Nothing to cleanup
	}

	// Delete oldest backups
	for i := keepCount; i < len(backups); i++ {
		if err := m.Delete(backups[i].Filename); err != nil {
			return fmt.Errorf("failed to delete old backup %s: %w", backups[i].Filename, err)
		}
	}

	return nil
}

// Helper functions

func addFileToTar(tw *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name:    name,
		Mode:    0644,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := tw.Write(data); err != nil {
		return err
	}

	return nil
}

func addFilePathToTar(tw *tar.Writer, filePath, nameInTar string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    nameInTar,
		Mode:    0644,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := io.Copy(tw, file); err != nil {
		return err
	}

	return nil
}

func addDirectoryToTar(tw *tar.Writer, dirPath, nameInTar string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		// Name in tar
		tarPath := filepath.Join(nameInTar, relPath)

		if info.IsDir() {
			header := &tar.Header{
				Name:     tarPath + "/",
				Mode:     0755,
				ModTime:  info.ModTime(),
				Typeflag: tar.TypeDir,
			}
			return tw.WriteHeader(header)
		}

		return addFilePathToTar(tw, path, tarPath)
	})
}
