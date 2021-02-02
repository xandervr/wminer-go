package helpers

import (
	// "crypto/sha256"

	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strings"

	sha256 "github.com/minio/sha256-simd"
)

func SerializeSHA256(txt string) string {
	// h := sha256.New()
	// h.Write([]byte(txt))
	// return hex.EncodeToString(h.Sum(nil))
	return acceleratedSha256(txt)
}

func acceleratedSha256(txt string) string {
	// server := sha256.NewAvx512Server()
	shaWriter := sha256.New()
	// shaWriter = sha256.NewAvx512(server)
	shaWriter.Write([]byte(txt))
	digest := hex.EncodeToString(shaWriter.Sum(nil))
	return digest
}

func CalculateDifficulty(bits float64) float64 {
	return 0xffff * math.Pow(2, 208) / float64(bits)
}

func HexInt(n int64) string {
	diff := new(big.Int)
	diff.SetString(fmt.Sprintf("%d", n), 10)
	res := fmt.Sprintf("0x%x", diff)
	return res
}

func HexFloat64(n float64) string {
	diff := new(big.Int)
	diff.SetString(fmt.Sprintf("%f", n), 10)
	res := fmt.Sprintf("0x%x", diff)
	return res
}

func LittleEndian(txt string) string {
	splitted := make([]string, 0)
	for i := 0; i < len(txt); i += 2 {
		maxRange := i + 2
		if len(txt) < maxRange {
			maxRange = i + 1
		}
		splitted = append([]string{txt[i:maxRange]}, splitted...)
	}
	return strings.Join(splitted, "")
}

func GenerateMerkleRoot(txs []*Transaction) string {
	hashes := make([]string, 0)
	for _, tx := range txs {
		h := SerializeSHA256(
			LittleEndian(HexInt(tx.Timestamp)) +
				LittleEndian(tx.Sender) +
				LittleEndian(tx.Receiver) +
				LittleEndian(HexFloat64(tx.Amount)) +
				LittleEndian(HexFloat64(tx.Fee)))
		hashes = append(hashes, h)
	}

	var innerRecurse func(hsh []string) string

	innerRecurse = func(hsh []string) string {
		parents := make([]string, 0)
		i := 0
		for i < len(hsh) {
			l := hsh[i]
			r := l
			if i+1 < len(hsh) {
				r = hsh[i+1]
			}
			parents = append(parents, SerializeSHA256(l+r))
			i += 2
		}
		if len(parents) > 1 {
			return innerRecurse(parents)
		} else {
			return parents[0]
		}
	}

	if len(hashes) > 0 {
		return innerRecurse(hashes)
	} else {
		return ""
	}
}
