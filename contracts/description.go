package contracts

import (
	"fmt"

	"github.com/qoryai/integrations/internal/rawjson"
)

// The limits of a description that description.schema.json states in words, since a
// schema cannot express them.
const (
	// DescriptionMaxBytes is the most bytes a description's raw document takes, 256 KiB.
	DescriptionMaxBytes = 262144
	// DescriptionMaxStringBytes is the most bytes of UTF-8 a string of a description,
	// a member name included, holds.
	DescriptionMaxStringBytes = 16384
	// DescriptionMaxValues is the most JSON values a description holds in all: the
	// document itself, each array entry and each member's value count one each, and
	// member names none.
	DescriptionMaxValues = 4096
	// DescriptionMaxDepth is the most levels arrays and objects nest in a description,
	// the top-level object being level 1.
	DescriptionMaxDepth = 64
)

// CheckDescription reports the first rule of description.schema.json that a
// description's raw bytes break and that no schema can express: the bytes are at most
// [DescriptionMaxBytes] and are UTF-8; no string holds a \u escape of a lone surrogate;
// no object repeats a member, at any depth; no string, a member name included, holds
// more than [DescriptionMaxStringBytes] bytes of UTF-8; the document holds at most
// [DescriptionMaxValues] JSON values; and it nests arrays and objects at most
// [DescriptionMaxDepth] levels deep. It checks b before any decoder reads it leniently,
// and leaves the schema's rules to the schema.
func CheckDescription(b []byte) error {
	if len(b) > DescriptionMaxBytes {
		return fmt.Errorf("the document is %d bytes, more than %d (256 KiB)", len(b), DescriptionMaxBytes)
	}
	if err := rawjson.Text(b); err != nil {
		return err
	}
	return rawjson.Check(b, rawjson.Rules{
		MaxStringBytes: DescriptionMaxStringBytes,
		MaxValues:      DescriptionMaxValues,
		MaxDepth:       DescriptionMaxDepth,
	})
}
