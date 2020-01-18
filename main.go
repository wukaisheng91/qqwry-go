package main

import (
	"fmt"
	"qqwry-ip/qqwry"
)

func main() {

	ipinfo := qqwry.IpData.Find("223.247.9.0")
	fmt.Printf("IP:%v, Localtion:%v, Owner:%v", ipinfo.IP, ipinfo.Loc, ipinfo.Owner)

	// allIp := qqwry.FeachAll()
	// for val := range allIp {
	// 	fmt.Println(val)
	// }
}
