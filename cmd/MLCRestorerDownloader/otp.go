package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

func getCommonKey() ([]byte, error) {
	if fileExists("otp.bin") {
		otp, err := os.ReadFile("otp.bin")
		if err != nil {
			return nil, err
		}
		commonKey := otp[0x0E0 : 0x0E0+0x10]
		commonKeyHash := sha1.Sum(commonKey)
		if hex.EncodeToString(commonKeyHash[:]) != "6a0b87fc98b306ae3366f0e0a88d0b06a2813313" {
			return nil, fmt.Errorf("invalid common key from otp")
		}
		return commonKey, nil
	}
	fmt.Print("Common key not found. Enter it here: ")
	var inputKey string
	fmt.Scanln(&inputKey)
	commonKey, err := hex.DecodeString(strings.TrimSpace(inputKey))
	if err != nil {
		return nil, err
	}
	commonKeyHash := sha1.Sum(commonKey)
	if hex.EncodeToString(commonKeyHash[:]) != "6a0b87fc98b306ae3366f0e0a88d0b06a2813313" {
		return nil, fmt.Errorf("invalid common key")
	}
	return commonKey, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
