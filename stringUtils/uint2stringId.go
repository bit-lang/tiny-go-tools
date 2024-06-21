package stringUtils

// converts base 10 uint64 to base 32, and base 58 string
// notes:
// 1. base 32: 10 digits, and lower case letters, but removed 'l', 'o', 'y', and 'z'
// 2. base 58: 10 digits, upper case letters, lower case letters,
// removed 'I', 'O', and 'l', 'o', for identification convenience

import (
	"fmt"
)

const (
	Base32 = uint64(32)
	Base58 = uint64(58)
)

var (
	base32s = []byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f',
		'g', 'h', 'i', 'j', 'k', 'm', 'n', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x'}
	base58s = []byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
		'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'J', 'K', 'L', 'M', 'N', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
		'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'm', 'n', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z'}
)

func Uint64ToBase32Str(value uint64) string {
	return val2Str(value, Base32, base32s)
}

func Uint64ToBase58Str(value uint64) string {
	return val2Str(value, Base58, base58s)
}

func val2Str(value, conversionBase uint64, usingBase []byte) string {
	var res [16]byte // len 16 is enough to hold max uint64 value,
	// max uint64 (18446744073709551615) converted to base32s is fxxxxxxxxxxxx (len is 13)
	var i int
	for i = len(res) - 1; ; i-- {
		res[i] = usingBase[value%conversionBase]
		value /= conversionBase
		if value == 0 {
			break
		}
	}
	return string(res[i:])
}

func Uint64ToIdStr(value, conversionBase uint64) (string, error) {
	var usingBase []byte
	switch conversionBase { // guard against invalid conversion Base
	case Base32:
		usingBase = base32s
	case Base58:
		usingBase = base58s
	default:
		return "", fmt.Errorf("conversion base incorrect: %d, must be 32 or 58", conversionBase)
	}
	return val2Str(value, conversionBase, usingBase), nil
}
