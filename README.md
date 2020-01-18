# QQWRY-go

QQWRY 纯真 IP 库 golang 版

## 使用

- 下载

```bash
go get github.com/kasonpasser/qqwry-go
```

- 在项目中引入

```go
import (
	"github.com/kasonpasser/qqwry-go"
	"fmt"
)

func main() {

    ipinfo := qqwry.IpData.Find("223.247.9.0")
	fmt.Printf("IP:%v, Localtion:%v, Owner:%v", ipinfo.IP, ipinfo.Loc, ipinfo.Owner)
    //  IP:223.247.9.0, Localtion:安徽省池州市青阳县, Owner:电信
}
```
