package codec

import (
	"encoding/binary"
	"testing"
)

func Test_getUvarintLen(t *testing.T) {
	buf := make([]byte, 100)

	compara := func(x uint64) {
		if binary.PutUvarint(buf, x) != getUvarintLen(x) {
			t.Fatal(x)
		}
	}

	compara(1)
	compara(100)
	compara(100000000)
	compara(1000000000000)
}
