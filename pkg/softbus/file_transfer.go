package softbus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	FileChunkSize = 64 * 1024 // 64KB chunks
)

// FileTransferState represents the state of a file transfer
type FileTransferState int

const (
	TransferPending FileTransferState = iota
	TransferActive
	TransferComplete
	TransferFailed
	TransferCancelled
)

// FileTransfer represents a file transfer operation
type FileTransfer struct {
	ID          string
	FileName    string
	FileSize    int64
	Checksum    string
	SourceID    string
	DestID      string
	State       FileTransferState
	Progress    float64 // 0.0 - 1.0
	BytesSent   int64
	TotalChunks int
	SentChunks  int
}

// FileTransferManager manages file transfers
type FileTransferManager struct {
	mu         sync.RWMutex
	transfers  map[string]*FileTransfer
	transport  *Transport
	deviceID   string
	downloadDir string
}

// NewFileTransferManager creates a new file transfer manager
func NewFileTransferManager(transport *Transport, deviceID, downloadDir string) *FileTransferManager {
	return &FileTransferManager{
		transfers:   make(map[string]*FileTransfer),
		transport:   transport,
		deviceID:    deviceID,
		downloadDir: downloadDir,
	}
}

// SendFile initiates a file transfer to a device
func (ftm *FileTransferManager) SendFile(filePath, destDevice string) (*FileTransfer, error) {
	// Check file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	// Calculate checksum
	checksum, err := calculateFileChecksum(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Generate transfer ID
	transferID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	totalChunks := int(fileInfo.Size()/FileChunkSize) + 1

	transfer := &FileTransfer{
		ID:          transferID,
		FileName:    filepath.Base(filePath),
		FileSize:    fileInfo.Size(),
		Checksum:    checksum,
		SourceID:    ftm.deviceID,
		DestID:      destDevice,
		State:       TransferPending,
		TotalChunks: totalChunks,
	}

	ftm.mu.Lock()
	ftm.transfers[transferID] = transfer
	ftm.mu.Unlock()

	// Send file transfer request
	msg := NewMessage(MsgFileTransfer, ftm.deviceID, destDevice)
	msg.Metadata["transfer_id"] = transferID
	msg.Metadata["file_name"] = transfer.FileName
	msg.Metadata["file_size"] = fmt.Sprintf("%d", transfer.FileSize)
	msg.Metadata["checksum"] = checksum
	msg.Metadata["total_chunks"] = fmt.Sprintf("%d", totalChunks)

	if err := ftm.transport.Send(destDevice, msg); err != nil {
		transfer.State = TransferFailed
		return nil, fmt.Errorf("failed to send transfer request: %w", err)
	}

	transfer.State = TransferActive

	// In production: send file chunks
	go ftm.sendFileChunks(transfer, filePath)

	return transfer, nil
}

func (ftm *FileTransferManager) sendFileChunks(transfer *FileTransfer, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		transfer.State = TransferFailed
		return
	}
	defer file.Close()

	buffer := make([]byte, FileChunkSize)
	chunkIndex := 0

	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			transfer.State = TransferFailed
			return
		}

		// Send chunk
		msg := NewMessage(MsgData, ftm.deviceID, transfer.DestID)
		msg.Metadata["transfer_id"] = transfer.ID
		msg.Metadata["chunk_index"] = fmt.Sprintf("%d", chunkIndex)
		msg.Payload = buffer[:n]

		if err := ftm.transport.Send(transfer.DestID, msg); err != nil {
			transfer.State = TransferFailed
			return
		}

		chunkIndex++
		transfer.SentChunks = chunkIndex
		transfer.BytesSent += int64(n)
		transfer.Progress = float64(transfer.BytesSent) / float64(transfer.FileSize)
	}

	transfer.State = TransferComplete
	transfer.Progress = 1.0
}

// GetTransfer returns a file transfer by ID
func (ftm *FileTransferManager) GetTransfer(transferID string) *FileTransfer {
	ftm.mu.RLock()
	defer ftm.mu.RUnlock()
	return ftm.transfers[transferID]
}

// GetActiveTransfers returns all active transfers
func (ftm *FileTransferManager) GetActiveTransfers() []*FileTransfer {
	ftm.mu.RLock()
	defer ftm.mu.RUnlock()

	transfers := make([]*FileTransfer, 0)
	for _, t := range ftm.transfers {
		if t.State == TransferActive || t.State == TransferPending {
			transfers = append(transfers, t)
		}
	}
	return transfers
}

// CancelTransfer cancels a file transfer
func (ftm *FileTransferManager) CancelTransfer(transferID string) error {
	ftm.mu.Lock()
	defer ftm.mu.Unlock()

	transfer, exists := ftm.transfers[transferID]
	if !exists {
		return fmt.Errorf("transfer not found: %s", transferID)
	}

	transfer.State = TransferCancelled
	return nil
}

func calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}