package qqwry

import (
	"encoding/binary"
	"fmt"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
)

/**
+----------+
|  文件头  |  (8字节)
+----------+
|  记录区  | （不定长）
+----------+
|  索引区  | （大小由文件头决定）
+----------+

文件头：4字节开始索引偏移值+4字节结尾索引偏移值

记录区： 每条IP记录格式 ==> IP地址[国家信息][地区信息]
   对于国家记录，可以有三种表示方式：
       字符串形式(IP记录第5字节不等于0x01和0x02的情况)，
       重定向模式1(第5字节为0x01),则接下来3字节为国家信息存储地的偏移值
       重定向模式(第5字节为0x02),
   
   对于地区记录，可以有两种表示方式： 字符串形式和重定向
   最后一条规则：重定向模式1的国家记录后不能跟地区记录

索引区： 每条索引记录格式 ==> 4字节起始IP地址 + 3字节指向IP记录的偏移值
   索引区的IP和它指向的记录区一条记录中的IP构成一个IP范围。查询信息是这个
   范围内IP的信息
*/

const (
	// IndexLen 索引长度
	IndexLen = 7
	// 用于二分查找折半算中间值
	IndexLenTwo = IndexLen << 1
	// RedirectMode1 国家的类型, 指向另一个指向
	RedirectMode1 = 0x01
	// RedirectMode2 国家的类型, 指向另一个指向
	RedirectMode2 = 0x02
)

// 定义信息结构
type IPinfo struct {
	Loc      string   // IP 所在的地址
	Owner    string   // IP 的所有者
}

// IP 归属地信息
type ResultQQwry struct {
	IP string
	IPinfo
}

// QQwry 纯真ip库
type QQwry struct {
	IPNum int64    // ip 的个数
	DataLen int64  // 数据的长度
	Offset int64   // 偏移量
	Data []byte    // IP的数据
}

var IpData QQwry

func init(){
	_ = IpData.initIPData("./data/qqwry.dat")
}

// InitIPData 初始化ip库数据到内存中
func (q *QQwry) initIPData(datapath string) (rs interface{}) {
	datFile := flag.String("qqwry", datapath, "纯真 IP 库的地址")
	flag.Parse()
 
	// 判断文件是否存在
	_, err := os.Stat(*datFile)
	if err != nil && os.IsNotExist(err) {
		log.Println("IP库文件不存在，请下载")
		return
	} else {
		// 打开文件句柄
		log.Printf("从本地数据库文件 %s 打开\n", *datFile)
		Path, err := os.OpenFile(*datFile, os.O_RDONLY, 0400)
		if err != nil {
			return
		}
		defer Path.Close()
		q.Data, err = ioutil.ReadAll(Path)
		if err != nil {
			log.Println(err)
			return
		}
	}

	q.DataLen = int64(len(q.Data))
	start := binary.LittleEndian.Uint32(q.Data[:4])
	end := binary.LittleEndian.Uint32(q.Data[4:8])
	// 计算 ip 收录的个数
	q.IPNum = int64((end-start)/IndexLen + 1)
	return
}

// ReadData 从文件中读取数据  num 读取的长度  offset 偏移的位置
func (q *QQwry) readData(num int, offset ...int64) (rs []byte) {
	if len(offset) > 0 {
		q.Offset = offset[0]
	}
	if q.Offset > q.DataLen {
		return nil
	}

	end := q.Offset + int64(num)
	if end > q.DataLen {
		end = q.DataLen
	}
	rs = q.Data[q.Offset:end]
	q.Offset = end
	return
}

// Find ip地址查询对应归属地信息
func (q *QQwry) Find(ip string) (res ResultQQwry) {
	res = ResultQQwry{}
	res.IP = ip
	if strings.Count(ip, ".") != 3 {
		return res
	}
	offset := q.searchIndex(binary.BigEndian.Uint32(net.ParseIP(ip).To4()))
	res.IPinfo = q.findCity(offset)
	return
}

// 通过偏移量来找城市
func (q *QQwry) findCity(offset uint32) (res IPinfo){
	res = IPinfo{}
	if offset <= 0 {
		return
	}

	var loc []byte
	var owner []byte
	// 这里是读取 ip 段后面的字节
	mode := q.readMode(offset + 4)
	if mode == RedirectMode1 {
		locOffset := q.readUInt24()
		mode = q.readMode(locOffset)
		if mode == RedirectMode2 {
			c := q.readUInt24()
			loc = q.readString(c)
			locOffset += 4
		} else {
			loc = q.readString(locOffset)
			locOffset += uint32(len(loc) + 1)
		}
		owner = q.readOwner(locOffset)
	} else if mode == RedirectMode2 {
		locOffset := q.readUInt24()
		loc = q.readString(locOffset)
		owner = q.readOwner(offset + 8)
	} else {
		loc = q.readString(offset + 4)
		owner = q.readOwner(offset + uint32(5+len(loc)))
	}

	enc := simplifiedchinese.GBK.NewDecoder()
	res.Loc, _ = enc.String(string(loc))
	res.Owner, _ = enc.String(string(owner))
	return
}

// 把所有的 IP 都输出来
func (q *QQwry) FeachAll() <- chan ResultQQwry{
	channel := make(chan ResultQQwry)
	res := ResultQQwry{}
	header := q.readData(8, 0)
	start := binary.LittleEndian.Uint32(header[:4])
	end := binary.LittleEndian.Uint32(header[4:])
	buf := make([]byte, IndexLen)
	_ip := uint32(0)
	go func(){
		for start <= end{
			buf = q.readData(IndexLen, int64(start))
			// ip 转 int32
			_ip = binary.LittleEndian.Uint32(buf[:4])
			// 偏移位置
			offset := byteToUInt32(buf[4:])
			res.IPinfo = q.findCity(offset)
			res.IP = fmt.Sprintf("%d.%d.%d.%d",
			byte(_ip>>24), byte(_ip>>16), byte(_ip>>8), byte(_ip))
			// 下一个 ip 的索引位置
			start += IndexLen
			channel <- res
		}
		close(channel)
	}()
	return channel
}

// readMode 获取偏移值类型
func (q *QQwry) readMode(offset uint32) byte {
	mode := q.readData(1, int64(offset))
	return mode[0]
}

// readArea 读取所属者
func (q *QQwry) readOwner(offset uint32) []byte {
	mode := q.readMode(offset)
	if mode == RedirectMode1 || mode == RedirectMode2 {
		ownerOffset := q.readUInt24()
		if ownerOffset == 0 {
			return []byte("")
		}
		return q.readString(ownerOffset)
	}
	return q.readString(offset)
}

// readString 获取字符串 从偏移位置开始读取字符串  读取到0为结束
func (q *QQwry) readString(offset uint32) []byte {
	q.Offset = int64(offset)
	data := make([]byte, 0, 30)
	buf := make([]byte, 1)
	for {
		buf = q.readData(1)
		if buf[0] == 0 {
			break
		}
		data = append(data, buf[0])
	}
	return data
}

// searchIndex 查找索引位置
func (q *QQwry) searchIndex(ip uint32) uint32 {
	header := q.readData(8, 0)
	start := binary.LittleEndian.Uint32(header[:4])
	end := binary.LittleEndian.Uint32(header[4:])

	buf := make([]byte, IndexLen)
	mid := uint32(0)
	_ip := uint32(0)

	for {
		if end-start == IndexLen {
			offset := byteToUInt32(buf[4:])
			buf = q.readData(IndexLen)
			if ip < binary.LittleEndian.Uint32(buf[:4]) {
				return offset
			}
			return 0
		}

		// 找出中间的 ip
		// 找出ip的位置
		mid = q.getMiddleOffset(start, end)
		// 读取ip的二进值
		buf = q.readData(IndexLen, int64(mid))
		// ip 转 int32
		_ip = binary.LittleEndian.Uint32(buf[:4])
		// 找到的比较大，向前移
		if _ip > ip {
			end = mid
		} else if _ip < ip { // 找到的比较小，向后移
			start = mid
		} else if _ip == ip {
			return byteToUInt32(buf[4:])
		}
	}
}

// readUInt24  读取三个字节
func (q *QQwry) readUInt24() uint32 {
	buf := q.readData(3)
	return byteToUInt32(buf)
}

// 二分查找，获取中间位置
func (q *QQwry) getMiddleOffset(start uint32, end uint32) uint32 {
	// 按索引的长度折半
	records := ((end - start) / IndexLenTwo)
	return start + records*IndexLen
}

// byteToUInt32 将 byte 转换为uint32
func byteToUInt32(data []byte) uint32 {
	i := uint32(data[0]) & 0xff
	i |= (uint32(data[1]) << 8) & 0xff00
	i |= (uint32(data[2]) << 16) & 0xff0000
	return i
}
