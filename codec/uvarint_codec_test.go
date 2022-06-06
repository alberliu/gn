package codec

import (
	"fmt"
	"testing"
)

func Test_getUvarintLen(t *testing.T) {
	fmt.Println(getUvarintLen(100000000))
}
