package lists

import (
	"container/list"
	"fmt"
	"testing"
	"unsafe"
)

// --- 测试数据类型定义 ---

// BigStruct 模拟较大的自定义结构体 (128 bytes)
// 用于测试内存拷贝和缓存局部性对性能的影响
type BigStruct struct {
	ID   int64
	Data [120]byte
}

// --- 1. 内存与对象大小分析 ---

func TestMemoryAnalysis(t *testing.T) {
	t.Log("=== Memory Footprint Analysis (64-bit Arch) ===")

	// 1. 标准库 container/list
	// Element 结构: next(8) + prev(8) + list(8) + Value(16, interface header) = 40 bytes
	var stdElem list.Element
	stdSize := unsafe.Sizeof(stdElem)
	t.Logf("[StdLib] list.Element size: %d bytes (Fixed overhead)", stdSize)
	t.Logf("         + Heap alloc for value (if not pointer)")

	// 2. Silo LinkedList (Int)
	// node[int] 结构: prev(8) + next(8) + val(8) = 24 bytes
	var intNode node[int]
	siloIntSize := unsafe.Sizeof(intNode)
	t.Logf("[Silo]   node[int] size:    %d bytes (Inline value)", siloIntSize)

	// 3. Silo LinkedList (BigStruct)
	// node[BigStruct] 结构: prev(8) + next(8) + val(128) = 144 bytes
	var structNode node[BigStruct]
	siloStructSize := unsafe.Sizeof(structNode)
	t.Logf("[Silo]   node[BigStruct] size: %d bytes (Inline value)", siloStructSize)

	// 结论输出
	t.Log("------------------------------------------------")
	t.Logf("Memory Efficiency (Int):       Silo saves ~%d%% per node + avoids 1 alloc",
		(int64(stdSize)-int64(siloIntSize))*100/int64(stdSize))
	t.Log("Memory Efficiency (BigStruct): Silo uses 1 contiguous block vs StdLib's 2 blocks (Node + Value)")
}

// --- 2. 统一基准测试 (Benchmarks) ---

// BenchmarkLists 运行所有列表相关的基准测试
// 使用表驱动的方式一次性运行所有场景，避免手动逐个运行
func BenchmarkLists(b *testing.B) {
	// 定义测试规模：小规模用于测试开销，大规模用于测试缓存和遍历
	// 避免使用过大的 Size (如 1M) 进行 Append 测试，否则会导致测试时间过长
	sizes := []int{100, 10_000, 1_000_000}

	// 定义测试场景
	scenarios := []struct {
		name string
		fn   func(b *testing.B, size int, isSilo bool)
	}{
		{"Append_Int", benchmarkAppendInt},
		{"Append_Struct", benchmarkAppendStruct},
		{"Append_StructPtr", benchmarkAppendStructPtr},
		{"Prepend_Int", benchmarkPrependInt},
		{"Modify_Middle", benchmarkModifyMiddle},
		{"Iterate", benchmarkIterate},
		{"Queue_PushPop", benchmarkQueue},
		{"RandomAccess_Get", benchmarkGet},
	}

	for _, sc := range scenarios {
		b.Run(sc.name, func(b *testing.B) {
			for _, size := range sizes {
				// 运行 StdLib 版本
				b.Run(fmt.Sprintf("StdLib/%d", size), func(b *testing.B) {
					sc.fn(b, size, false)
				})
				// 运行 Silo 版本
				b.Run(fmt.Sprintf("Silo/%d", size), func(b *testing.B) {
					sc.fn(b, size, true)
				})
			}
		})
	}
}

// --- 具体场景实现 ---

// benchmarkAppendInt 测试追加 int 的性能 (包含列表创建开销)
func benchmarkAppendInt(b *testing.B, size int, isSilo bool) {
	if isSilo {
		for i := 0; i < b.N; i++ {
			l := NewLinkedList[int]()
			for j := 0; j < size; j++ {
				l.Add(j)
			}
		}
	} else {
		for i := 0; i < b.N; i++ {
			l := list.New()
			for j := 0; j < size; j++ {
				l.PushBack(j)
			}
		}
	}
}

// benchmarkAppendStruct 测试追加大结构体的性能 (测试内存拷贝/分配)
// 注意：Silo 存储的是值(内联)，每次 Add 都会拷贝 128 字节。
// StdLib 存储的是接口，如果复用同一个变量，可能只拷贝指针/接口头。
func benchmarkAppendStruct(b *testing.B, size int, isSilo bool) {
	val := BigStruct{ID: 1}
	if isSilo {
		for i := 0; i < b.N; i++ {
			l := NewLinkedList[BigStruct]()
			for j := 0; j < size; j++ {
				l.Add(val)
			}
		}
	} else {
		for i := 0; i < b.N; i++ {
			l := list.New()
			for j := 0; j < size; j++ {
				l.PushBack(val)
			}
		}
	}
}

// benchmarkAppendStructPtr 测试追加大结构体指针的性能
// 这种场景下，Silo 和 StdLib 都只存储指针 (8 bytes)，主要对比节点分配开销
func benchmarkAppendStructPtr(b *testing.B, size int, isSilo bool) {
	val := &BigStruct{ID: 1}
	if isSilo {
		for i := 0; i < b.N; i++ {
			l := NewLinkedList[*BigStruct]()
			for j := 0; j < size; j++ {
				l.Add(val)
			}
		}
	} else {
		for i := 0; i < b.N; i++ {
			l := list.New()
			for j := 0; j < size; j++ {
				l.PushBack(val)
			}
		}
	}
}

// benchmarkPrependInt 测试头部插入 (Stack Push / Prepend)
// 这是链表的强项 (O(1))，用于对比 Array (O(N)) 或验证分配性能
func benchmarkPrependInt(b *testing.B, size int, isSilo bool) {
	if isSilo {
		for i := 0; i < b.N; i++ {
			l := NewLinkedList[int]()
			for j := 0; j < size; j++ {
				l.AddFirst(j)
			}
		}
	} else {
		for i := 0; i < b.N; i++ {
			l := list.New()
			for j := 0; j < size; j++ {
				l.PushFront(j)
			}
		}
	}
}

// benchmarkModifyMiddle 测试中间插入和删除
// 包含 O(N) 的查找成本 + O(1) 的链接成本
// 通过 Insert + Remove 保持列表大小不变
func benchmarkModifyMiddle(b *testing.B, size int, isSilo bool) {
	mid := size / 2
	if isSilo {
		l := NewLinkedList[int]()
		for j := 0; j < size; j++ {
			l.Add(j)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Insert(mid, i)
			l.Remove(mid)
		}
	} else {
		l := list.New()
		for j := 0; j < size; j++ {
			l.PushBack(j)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// StdLib 需要手动遍历到中间
			e := l.Front()
			for k := 0; k < mid; k++ {
				e = e.Next()
			}
			// InsertBefore 返回新插入的元素
			newElem := l.InsertBefore(i, e)
			l.Remove(newElem)
		}
	}
}

// benchmarkIterate 测试遍历性能
func benchmarkIterate(b *testing.B, size int, isSilo bool) {
	// Setup: 构建一次列表
	if isSilo {
		l := NewLinkedList[int]()
		for j := 0; j < size; j++ {
			l.Add(j)
		}
		b.ResetTimer() // 重置计时器，只测量遍历时间
		for i := 0; i < b.N; i++ {
			sum := 0
			for v := range l.Values() {
				sum += v
			}
		}
	} else {
		l := list.New()
		for j := 0; j < size; j++ {
			l.PushBack(j)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sum := 0
			for e := l.Front(); e != nil; e = e.Next() {
				sum += e.Value.(int)
			}
		}
	}
}

// benchmarkQueue 测试队列操作 (Push Back + Pop Front)
// 这种模式下列表大小保持恒定，测试节点分配和回收的开销
func benchmarkQueue(b *testing.B, size int, isSilo bool) {
	if isSilo {
		l := NewLinkedList[int]()
		for j := 0; j < size; j++ {
			l.Add(j)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Add(i)
			l.RemoveFirst()
		}
	} else {
		l := list.New()
		for j := 0; j < size; j++ {
			l.PushBack(j)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.PushBack(i)
			l.Remove(l.Front())
		}
	}
}

// benchmarkGet 测试随机访问 (Get Middle)
// 注意：LinkedList 的随机访问是 O(N) 的，这里主要对比实现差异
func benchmarkGet(b *testing.B, size int, isSilo bool) {
	mid := size / 2
	if isSilo {
		l := NewLinkedList[int]()
		for j := 0; j < size; j++ {
			l.Add(j)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Get(mid)
		}
	} else {
		l := list.New()
		for j := 0; j < size; j++ {
			l.PushBack(j)
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// StdLib 没有 Get(index)，必须手动遍历
			e := l.Front()
			for k := 0; k < mid; k++ {
				e = e.Next()
			}
			_ = e.Value
		}
	}
}
