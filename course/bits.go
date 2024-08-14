package course

// 判断第i位是否为1，i从1开始
func IsBit1(n uint64, i int) bool {
	if i > 64 {
		panic(i)
	}

	c := uint64(1 << (i - 1))
	if c&n == c {
		return true
	}
	return false
}

func SetBit1(n uint64, i int) uint64 {
	if i > 64 {
		panic(i)
	}
	c := uint64(1 << (i - 1))

	return c | n
}

func CountBit1(n uint64) int {
	var ans int
	for n > 0 {
		if n&1 == 1 {
			ans++
		}
		n = n >> 1
	}
	return ans
}

const (
	MALE = 1 << iota
	VIP
	WEEK_ACTIVE
)

type Candidate struct {
	Id     int
	Gender string
	Vip    bool
	Active int //几天内活跃
	Bits   uint64
}

func (c *Candidate) SetMale() {
	c.Gender = "男"
	c.Bits |= MALE
}

func (c *Candidate) SetVip() {
	c.Vip = true
	c.Bits |= VIP
}

func (c *Candidate) SetActive(day int) {
	c.Active = day
	if day <= 7 {
		c.Bits |= WEEK_ACTIVE
	}
}

// 判断3个条件是否同时满足
func (c *Candidate) Filter1(male, vip, weekActive bool) bool {
	if male && c.Gender != "男" {
		return false
	}
	if vip && !c.Vip {
		return false
	}
	if weekActive && c.Active > 7 {
		return false
	}
	return true
}

func (c *Candidate) Filter2(on uint64) bool {
	return c.Bits&on == on
}

type BitMap struct {
	Table uint64
}

func CreateBitMap(min int, arr []int) *BitMap {
	bitMap := new(BitMap)
	for _, ele := range arr {
		index := ele - min
		bitMap.Table = SetBit1(bitMap.Table, index)
	}
	return bitMap
}

// 位图求交集
func IntersectionOfBitMap(bm1, bm2 *BitMap, min int) []int {
	res := make([]int, 0, 100)
	s := bm1.Table & bm2.Table
	for i := 1; i <= 64; i++ {
		if IsBit1(s, i) {
			res = append(res, i+min)
		}
	}
	return res
}

// 有序链表求交际
func IntersectionOfOrderedList(arr1, arr2 []int) []int {
	m, n := len(arr1), len(arr2)
	if m == 0 || n == 0 {
		return nil
	}
	res := make([]int, 0, 100)
	var i, j int
	for i < m && j < n {
		if arr1[i] == arr2[j] {
			res = append(res, arr1[i])
			i++
			j++
		} else if arr1[i] < arr2[j] {
			i++
		} else {
			j++
		}
	}
	return res
}
