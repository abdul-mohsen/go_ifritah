package handlers

import (
	"crypto/sha1" //nolint:gosec // G505: UUID v5 is defined over SHA-1 (RFC 4122). It names a bill and is not a security control.
	"fmt"
	"regexp"
	"strconv"
)

// importPosID is the pos_id of every sale that the server imports. An import runs on the server, not on a till.
const importPosID = "import"

// importSessionField is the form field, and the JSON key, that carries the session of a sales import.
const importSessionField = "import_session"

// importSessionMessage answers a sales import whose session is missing or is not a UUID v4.
const importSessionMessage = "The import session is missing or invalid. Reload the page and try again."

// importNamespace is the UUID v5 namespace of the import bill_request_ids, 7b0f7f4e-3c1a-4e5b-9d2a-8f6e1c0b9a47.
// Changing it changes the identity that a retry sends.
var importNamespace = [16]byte{0x7b, 0x0f, 0x7f, 0x4e, 0x3c, 0x1a, 0x4e, 0x5b, 0x9d, 0x2a, 0x8f, 0x6e, 0x1c, 0x0b, 0x9a, 0x47}

// importSessionPattern matches a lowercase UUID v4, the form in which the import page creates a session.
var importSessionPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// validImportSession reports whether session is a lowercase UUID v4.
func validImportSession(session string) bool {
	return importSessionPattern.MatchString(session)
}

// importSaleIdentity returns the pos_id and bill_request_id of bill number row (counted from 1) in a sales import. The
// bill_request_id is the UUID v5 of "<session>:<row>" under importNamespace. A retry of the same session sends the same
// identity, so the backend answers with the bill that the first attempt created.
func importSaleIdentity(session string, row int) (string, string) {
	return importPosID, uuidV5(importNamespace, session+":"+strconv.Itoa(row))
}

// uuidV5 returns the RFC 4122 version 5 UUID of name in namespace, in lowercase.
func uuidV5(namespace [16]byte, name string) string {
	sum := sha1.Sum(append(namespace[:], name...)) //nolint:gosec // G401: the SHA-1 of a UUID v5, see the import.
	var b [16]byte
	copy(b[:], sum[:16])
	b[6] = b[6]&0x0f | 0x50
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
