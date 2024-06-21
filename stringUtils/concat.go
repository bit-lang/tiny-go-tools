package stringUtils

import (
	"bytes"
  "fmt"
)

// concat input strings into a single string
func ConcatStrs(names ...string) string {
    var buffer bytes.Buffer
    for _, name := range names {
        buffer.WriteString(name)
    }
    return buffer.String()
}

// concat input strings into a single byte slice
func ConcatStrsAsBytes(names ...string) []byte {
    var buffer bytes.Buffer
    for _, name := range names {
        buffer.WriteString(name)
    }
    return buffer.Bytes()
}

func MergeByteSlices(inputs [][]byte) ([]byte, error) {
	output_buffer := bytes.NewBuffer(nil)
	for ndex, buffs := range inputs {
		_, err_buff := output_buffer.Write(buffs)
		if err_buff != nil {
			return nil, fmt.Errorf("merge byte slices() writing buffer #%d error: %v", ndex, err_buff)
		}
	}
	return output_buffer.Bytes(), nil
}
