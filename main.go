package main

import "fmt"

func main() {
	n := 8
	var ans int
	for n > 0 {
		if n&1 == 1 {
			ans++
		}
		n >>= 1
	}
	fmt.Println(ans)
}
