package mlcrestorerdownloader

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	WiiUCommonKeySHA1 = "6a0b87fc98b306ae3366f0e0a88d0b06a2813313"
	h3TotalBlock      = 0x10000
	h3HeaderSize      = 0x400
	h3BatchSize       = h3TotalBlock * 32
	linearChunkSize   = 4 * 1024 * 1024
)

func normalizeTID(titleID string) string {
	r := strings.NewReplacer("-", "", "/", "", "\\", "")
	return strings.ToLower(strings.TrimSpace(r.Replace(titleID)))
}

func VerifyCommonKey(commonKey []byte) error {
	if len(commonKey) != 16 {
		return fmt.Errorf("invalid common key length: %d", len(commonKey))
	}
	sum := sha1.Sum(commonKey)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), WiiUCommonKeySHA1) {
		return errors.New("invalid common key SHA-1")
	}
	return nil
}

func ReadTMDAndTitleKey(tmdPath, tikPath string, commonKey []byte, expectedTID string) (*TMD, []byte, error) {
	if err := VerifyCommonKey(commonKey); err != nil {
		return nil, nil, err
	}

	rawTMD, err := os.ReadFile(tmdPath)
	if err != nil {
		return nil, nil, err
	}
	if len(rawTMD) < 0x200 {
		return nil, nil, fmt.Errorf("invalid TMD size: %d", len(rawTMD))
	}

	tmd, err := ParseTMD(rawTMD)
	if err != nil {
		return nil, nil, err
	}
	if tmd.ContentCount == 0 || len(tmd.Contents) != int(tmd.ContentCount) {
		return nil, nil, errors.New("invalid TMD content count")
	}

	if expectedTID != "" {
		normExpected := normalizeTID(expectedTID)
		expectedInt, err := strconv.ParseUint(normExpected, 16, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid expected title ID %s: %w", expectedTID, err)
		}
		if tmd.TitleID != expectedInt {
			return nil, nil, fmt.Errorf("TMD title ID mismatch: got %016x, expected %016x", tmd.TitleID, expectedInt)
		}
	}

	rawTik, err := os.ReadFile(tikPath)
	if err != nil {
		return nil, nil, err
	}
	if len(rawTik) < 0x1D0 {
		return nil, nil, fmt.Errorf("invalid ticket size: %d", len(rawTik))
	}
	if len(rawTik) >= 0x1E4 {
		tikTID := binary.BigEndian.Uint64(rawTik[0x1DC:0x1E4])
		if tikTID != tmd.TitleID {
			return nil, nil, fmt.Errorf("manifest mismatch between TMD (%016x) and ticket (%016x)", tmd.TitleID, tikTID)
		}
	}

	commonBlock, err := aes.NewCipher(commonKey)
	if err != nil {
		return nil, nil, err
	}

	var ivTKey [16]byte
	copy(ivTKey[:8], rawTMD[0x18C:0x194])

	titleKey := make([]byte, 16)
	cipher.NewCBCDecrypter(commonBlock, ivTKey[:]).CryptBlocks(titleKey, rawTik[0x1BF:0x1CF])

	return tmd, titleKey, nil
}

func VerifyTitle(titleID, outputDirectory string, commonKey []byte, progressReporter ProgressReporter) error {
	outputDir := strings.TrimRight(outputDirectory, "/\\")
	tmdPath := filepath.Join(outputDir, "title.tmd")
	tikPath := filepath.Join(outputDir, "title.tik")
	certPath := filepath.Join(outputDir, "title.cert")

	certInfo, err := os.Stat(certPath)
	if err != nil || certInfo.IsDir() || certInfo.Size() == 0 {
		return fmt.Errorf("missing or empty title.cert for %s", titleID)
	}

	tmd, titleKey, err := ReadTMDAndTitleKey(tmdPath, tikPath, commonKey, titleID)
	if err != nil {
		return fmt.Errorf("integrity check failed for %s: %w", titleID, err)
	}

	titleBlock, err := aes.NewCipher(titleKey)
	if err != nil {
		return err
	}

	var totalSize uint64
	for i := range tmd.Contents {
		totalSize += tmd.Contents[i].Size
	}

	var verifiedBytes uint64
	onProgress := func(n uint64) {
		verifiedBytes += n
		if progressReporter != nil && totalSize > 0 {
			progress := float64(verifiedBytes) / float64(totalSize)
			if progress > 1.0 {
				progress = 1.0
			}
			progressReporter.UpdateDecryptionProgress(progress)
		}
	}

	for _, c := range tmd.Contents {
		if err := verifySingleContent(outputDir, c, titleBlock, onProgress); err != nil {
			return fmt.Errorf("title %s content %08X verification failed: %w", titleID, c.ID, err)
		}
	}

	return nil
}

func resolveContentFile(workDir string, cid uint32, ext string) (string, error) {
	candidates := []string{
		filepath.Join(workDir, fmt.Sprintf("%08X%s", cid, ext)),
		filepath.Join(workDir, fmt.Sprintf("%08x%s", cid, ext)),
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}
	return candidates[0], fmt.Errorf("missing file %s", filepath.Base(candidates[0]))
}

func verifySingleContent(workDir string, c Content, titleBlock cipher.Block, onProgress func(uint64)) error {
	appPath, err := resolveContentFile(workDir, c.ID, ".app")
	if err != nil {
		return err
	}
	appName := filepath.Base(appPath)

	if len(c.Hash) < 20 {
		return fmt.Errorf("invalid content hash length in TMD for %s", appName)
	}
	expectedHash := c.Hash[:20]

	var baseIV [16]byte
	if len(c.Index) >= 2 {
		baseIV[0] = c.Index[0]
		baseIV[1] = c.Index[1]
	}

	if (c.Type & 0x02) != 0 {
		h3Path, err := resolveContentFile(workDir, c.ID, ".h3")
		if err != nil {
			return err
		}
		h3Name := filepath.Base(h3Path)

		h3Raw, err := os.ReadFile(h3Path)
		if err != nil {
			return err
		}

		h3Sum := sha1.Sum(h3Raw)
		if !bytes.Equal(h3Sum[:], expectedHash) {
			return fmt.Errorf("H3 SHA-1 mismatch for %s", h3Name)
		}

		fApp, err := os.Open(appPath)
		if err != nil {
			return err
		}
		defer fApp.Close()

		idxHi := baseIV[0]
		idxLo := baseIV[1]
		chunkIdx := 0

		buf := make([]byte, h3BatchSize)
		hashes := make([]byte, h3HeaderSize)
		decPayload := make([]byte, h3TotalBlock-h3HeaderSize)
		var cIV [16]byte
		var expectedTrueHash [20]byte

		for {
			nRead, readErr := io.ReadFull(fApp, buf)
			if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
				return readErr
			}
			if nRead == 0 {
				break
			}

			pos := 0
			for pos < nRead {
				blockEnd := pos + h3TotalBlock
				if blockEnd > nRead {
					blockEnd = nRead
				}
				blockLen := blockEnd - pos
				if blockLen <= h3HeaderSize {
					return fmt.Errorf("short H3 block in %s at chunk %d", appName, chunkIdx)
				}

				cipher.NewCBCDecrypter(titleBlock, baseIV[:]).CryptBlocks(hashes, buf[pos:pos+h3HeaderSize])

				blockNum := chunkIdx & 0xF
				h0Off := blockNum * 20
				copy(expectedTrueHash[:], hashes[h0Off:h0Off+20])
				copy(cIV[:], expectedTrueHash[:16])

				if blockNum == 0 {
					cIV[0] ^= idxHi
					cIV[1] ^= idxLo
					expectedTrueHash[0] ^= idxHi
					expectedTrueHash[1] ^= idxLo
				}

				payloadLen := blockLen - h3HeaderSize
				encSlice := buf[pos+h3HeaderSize : blockEnd]
				rem := payloadLen & 0xF

				bodyDec := cipher.NewCBCDecrypter(titleBlock, cIV[:])
				var validDec []byte

				if rem != 0 {
					paddedLen := payloadLen + (16 - rem)
					paddedEnc := make([]byte, paddedLen)
					copy(paddedEnc, encSlice)
					paddedDec := make([]byte, paddedLen)
					bodyDec.CryptBlocks(paddedDec, paddedEnc)
					validDec = paddedDec[:payloadLen]
				} else {
					bodyDec.CryptBlocks(decPayload[:payloadLen], encSlice)
					validDec = decPayload[:payloadLen]
				}

				payloadSum := sha1.Sum(validDec)
				if payloadSum != expectedTrueHash {
					return fmt.Errorf("H0 hash mismatch in %s at chunk %d", appName, chunkIdx)
				}

				if onProgress != nil {
					onProgress(uint64(blockLen))
				}

				chunkIdx++
				pos += h3TotalBlock
			}

			if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
				break
			}
		}

		return nil
	}

	stat, err := os.Stat(appPath)
	if err != nil {
		return err
	}
	actualSize := uint64(stat.Size())
	alignedSize := (c.Size + 15) & ^uint64(15)
	if actualSize != c.Size && actualSize != alignedSize {
		return fmt.Errorf("size mismatch for %s: got %d, expected %d", appName, actualSize, c.Size)
	}

	fApp, err := os.Open(appPath)
	if err != nil {
		return err
	}
	defer fApp.Close()

	decMode := cipher.NewCBCDecrypter(titleBlock, baseIV[:])
	hasher := sha1.New()

	var processedSize uint64
	buf := make([]byte, linearChunkSize)
	decBuf := make([]byte, linearChunkSize)

	for processedSize < c.Size {
		nRead, readErr := fApp.Read(buf)
		if nRead > 0 {
			rem := nRead & 0xF
			var chunkLen uint64
			var decSlice []byte

			if rem != 0 {
				paddedLen := nRead + (16 - rem)
				paddedEnc := make([]byte, paddedLen)
				copy(paddedEnc, buf[:nRead])
				paddedDec := make([]byte, paddedLen)
				decMode.CryptBlocks(paddedDec, paddedEnc)
				decSlice = paddedDec
				chunkLen = uint64(paddedLen)
			} else {
				decMode.CryptBlocks(decBuf[:nRead], buf[:nRead])
				decSlice = decBuf[:nRead]
				chunkLen = uint64(nRead)
			}

			if processedSize+uint64(len(decSlice)) >= c.Size {
				validLen := c.Size - processedSize
				hasher.Write(decSlice[:validLen])
				if onProgress != nil {
					onProgress(validLen)
				}
				processedSize += validLen
				break
			}

			hasher.Write(decSlice)
			if onProgress != nil {
				onProgress(chunkLen)
			}
			processedSize += chunkLen
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return readErr
		}
	}

	if processedSize < c.Size {
		return fmt.Errorf("unexpected EOF in %s (%d/%d bytes)", appName, processedSize, c.Size)
	}

	computed := hasher.Sum(nil)
	if !bytes.Equal(computed, expectedHash) {
		return fmt.Errorf("linear SHA-1 mismatch for %s", appName)
	}

	return nil
}
