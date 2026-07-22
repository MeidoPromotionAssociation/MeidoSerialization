package application

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const (
	archivePageTokenVersion = byte(1)
	archivePageTokenMACSize = sha256.Size
)

// ArchivePager signs opaque cursors with a process-local random key. A pager
// belongs to one transport server; cursors from another server are rejected.
type ArchivePager struct {
	key [sha256.Size]byte
}

func NewArchivePager() (*ArchivePager, error) {
	pager := &ArchivePager{}
	if _, err := rand.Read(pager.key[:]); err != nil {
		return nil, fmt.Errorf("generate archive page-token key: %w", err)
	}
	return pager, nil
}

// ArchiveListing is one sorted, budget-checked archive view. fingerprint also
// binds cursors to the exact source bytes, not merely a matching entry count.
type ArchiveListing struct {
	FormatID    string
	Entries     []ArchiveEntry
	fingerprint [sha256.Size]byte
}

func (p *ArchivePager) Decode(listing ArchiveListing, token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	if len(token) > 512 {
		return 0, archivePageTokenError("token is too long")
	}
	if p == nil {
		return 0, archivePageTokenError("pager is unavailable")
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return 0, archivePageTokenError("token encoding is invalid")
	}
	const fixedPayload = 1 + 8 + 2 + sha256.Size
	if len(raw) < fixedPayload+archivePageTokenMACSize {
		return 0, archivePageTokenError("token is truncated")
	}
	formatLength := int(binary.BigEndian.Uint16(raw[9:11]))
	payloadLength := fixedPayload + formatLength
	if len(raw) != payloadLength+archivePageTokenMACSize {
		return 0, archivePageTokenError("token length is invalid")
	}
	payload, suppliedMAC := raw[:payloadLength], raw[payloadLength:]
	mac := hmac.New(sha256.New, p.key[:])
	_, _ = mac.Write(payload)
	if !hmac.Equal(suppliedMAC, mac.Sum(nil)) {
		return 0, archivePageTokenError("token signature is invalid")
	}
	if payload[0] != archivePageTokenVersion {
		return 0, archivePageTokenError("token version is unsupported")
	}
	formatID := string(payload[11 : 11+formatLength])
	if formatID != normalizeArchiveFormatID(listing.FormatID) {
		return 0, archivePageTokenError("token belongs to another format")
	}
	fingerprint := payload[11+formatLength : payloadLength]
	if !hmac.Equal(fingerprint, listing.fingerprint[:]) {
		return 0, archivePageTokenError("archive listing changed")
	}
	offset64 := binary.BigEndian.Uint64(payload[1:9])
	if offset64 > uint64(math.MaxInt) || int(offset64) >= len(listing.Entries) {
		return 0, archivePageTokenError("token offset is outside the current listing")
	}
	return int(offset64), nil
}

func (p *ArchivePager) Encode(listing ArchiveListing, offset int) (string, error) {
	if p == nil {
		return "", archivePageTokenError("pager is unavailable")
	}
	if offset <= 0 || offset >= len(listing.Entries) {
		return "", archivePageTokenError("next offset is outside the current listing")
	}
	formatID := normalizeArchiveFormatID(listing.FormatID)
	if formatID == "" || len(formatID) > math.MaxUint16 {
		return "", archivePageTokenError("format ID is invalid")
	}
	payload := make([]byte, 1+8+2+len(formatID)+sha256.Size)
	payload[0] = archivePageTokenVersion
	binary.BigEndian.PutUint64(payload[1:9], uint64(offset))
	binary.BigEndian.PutUint16(payload[9:11], uint16(len(formatID)))
	copy(payload[11:], formatID)
	copy(payload[11+len(formatID):], listing.fingerprint[:])
	mac := hmac.New(sha256.New, p.key[:])
	_, _ = mac.Write(payload)
	raw := append(payload, mac.Sum(nil)...)
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func archiveListingFingerprint(formatID string, sourceDigest [sha256.Size]byte, entries []ArchiveEntry) [sha256.Size]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("meido-archive-listing-v1\x00"))
	writeArchiveFingerprintString(hash, normalizeArchiveFormatID(formatID))
	_, _ = hash.Write(sourceDigest[:])
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], uint64(len(entries)))
	_, _ = hash.Write(count[:])
	for _, entry := range entries {
		writeArchiveFingerprintString(hash, entry.Name)
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(entry.Size))
		_, _ = hash.Write(size[:])
		writeArchiveFingerprintString(hash, entry.Kind)
	}
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result
}

type archiveFingerprintWriter interface {
	Write([]byte) (int, error)
}

func writeArchiveFingerprintString(writer archiveFingerprintWriter, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write([]byte(value))
}

func normalizeArchiveFormatID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func archivePageTokenError(reason string) error {
	return opError("archive page token", CodeInvalidArgument, fmt.Errorf("invalid archive page_token: %s", reason))
}
