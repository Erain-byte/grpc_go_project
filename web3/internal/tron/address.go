// Package tron contains the TRON-specific read-only integration used by the
// public USDT approval checker. It never signs or broadcasts transactions.
package tron

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math/big"
	"strings"
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var base58Radix = big.NewInt(58)

// DecodeMainnetAddress validates a TRON Base58Check address and returns its
// 21-byte payload (0x41 network prefix plus the 20-byte account address).
func DecodeMainnetAddress(address string) ([]byte, error) {
	address = strings.TrimSpace(address)
	if len(address) != 34 || !strings.HasPrefix(address, "T") {
		return nil, errors.New("TRON address must be a 34-character Base58Check string starting with T")
	}

	value := new(big.Int)
	for _, char := range address {
		index := strings.IndexRune(base58Alphabet, char)
		if index < 0 {
			return nil, errors.New("TRON address contains invalid Base58 characters")
		}
		value.Mul(value, base58Radix)
		value.Add(value, big.NewInt(int64(index)))
	}
	decoded := value.Bytes()
	leadingZeros := 0
	for leadingZeros < len(address) && address[leadingZeros] == '1' {
		leadingZeros++
	}
	if leadingZeros > 0 {
		decoded = append(make([]byte, leadingZeros), decoded...)
	}
	if len(decoded) != 25 {
		return nil, errors.New("TRON address has an invalid decoded length")
	}

	payload, checksum := decoded[:21], decoded[21:]
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])
	if !bytes.Equal(checksum, second[:4]) {
		return nil, errors.New("TRON address checksum is invalid")
	}
	if payload[0] != 0x41 {
		return nil, errors.New("TRON address is not a mainnet address")
	}
	return append([]byte(nil), payload...), nil
}

// ABIAddressParameter converts a Base58Check address into the 32-byte ABI
// address slot expected by TRON's triggerconstantcontract endpoint.
func ABIAddressParameter(address string) (string, error) {
	payload, err := DecodeMainnetAddress(address)
	if err != nil {
		return "", err
	}
	// TVM ABI addresses use the final 20 bytes without the TRON 0x41 prefix.
	return strings.Repeat("0", 24) + bytesToUpperHex(payload[1:]), nil
}

func bytesToUpperHex(value []byte) string {
	const digits = "0123456789ABCDEF"
	encoded := make([]byte, len(value)*2)
	for i, current := range value {
		encoded[i*2] = digits[current>>4]
		encoded[i*2+1] = digits[current&0x0f]
	}
	return string(encoded)
}
