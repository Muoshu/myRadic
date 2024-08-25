package test

import (
	"fmt"
	"github.com/Muoshu/myRadic/demo"
	"testing"
)

func TestGetClassBits(t *testing.T) {
	fmt.Printf("%064b\n", demo.GetClassBits([]string{"五月天", "北京", "资讯", "热点"}))
}
