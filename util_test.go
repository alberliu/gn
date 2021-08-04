package gn

import (
	"fmt"
	"testing"
)

func Test_getIPPort(t *testing.T) {
	fmt.Println(getIPPort(":1111"))
	fmt.Println(getIPPort("111.00.00.00:1111"))
	fmt.Println(getIPPort("1:1111"))
}
