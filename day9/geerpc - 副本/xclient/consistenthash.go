package xclient

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func(data []byte) uint32

type Map struct {
	hash     Hash           // 一致性 hash 所用到的 hash 函数，可替换
	replicas int            // 虚拟结点的倍数
	keys     []int          // 需要排序，保证相对顺序不变
	hashMap  map[int]string // 虚拟结点与真实结点的映射
}

func New(replicas int, fn Hash) *Map {
	m := &Map{
		hash:     fn,
		replicas: replicas,
		keys:     []int{},
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// Add 添加真实结点
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		// 对每一个真实节点 key，对应创建 m.replicas 个虚拟节点，虚拟节点的名称是：strconv.Itoa(i) + key，即通过添加编号的方式区分不同虚拟节点。
		for i := 0; i < m.replicas; i++ { // 给该 key 添加对应的虚拟结点
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash) // 添加虚拟结点 hash 到环上
			m.hashMap[hash] = key         // 虚拟结点对真实结点的映射
		}
	}
	sort.Ints(m.keys) // 将所有虚拟结点的 hash 进行排序
}

func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}
	hash := int(m.hash([]byte(key)))
	// 顺时针寻找第一个虚拟结点
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	return m.hashMap[m.keys[idx % len(m.keys)]]
}