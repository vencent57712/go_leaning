package safeList

import (
	"sync"
	"sync/atomic"
	"unsafe"
	_ "unsafe"
)

type IntList struct {
	head   *intNode
	length int64
}

type intNode struct {
	value int
	Deleted uint32 // 0：未删除 1：已删除
	next  *intNode
	mu sync.Mutex
}

func newIntNode(value int) *intNode {
	return &intNode{value: value}
}

func NewInt() *IntList {
	return &IntList{head: newIntNode(0)}
}


// Insert 插入一个元素，如果此操作成功插入一个元素，则返回 true，否则返回 false
func (l *IntList) Insert(value int) bool {

	var a *intNode
	var b *intNode

	for {
		// 1、找到节点A和B，不存在直接返回
		a = l.head
		b = (*intNode)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&a.next))))

		for b != nil && b.value < value {
			a = b
			b = (*intNode)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&a.next))))
		}
		if b != nil && b.value == value {
			return false
		}

		// 2、锁定节点A，检查A.next==B，如果为假，则解锁A回到Step1
		a.mu.Lock()
		if a.next != b {
			a.mu.Unlock()
			continue
		}
		break
	}

	// 3、创建新节点
	x := newIntNode(value)

	// 4、X.next=B A.next=X
	x.next = b
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&a.next)), unsafe.Pointer(x))
	atomic.AddInt64(&l.length, 1)

	// 5、解锁节点A
	a.mu.Unlock()

	return true
}

func (l *IntList) Delete(value int) bool {

	var a *intNode
	var b *intNode

	for {
		// 1、找到节点A和B，不存在直接返回
		a = l.head
		b = (*intNode)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&a.next))))

		for b != nil && b.value < value {
			a = b
			b = (*intNode)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&a.next))))
		}
		if b == nil || b.value != value {
			return false
		}

		// 2、锁定节点B，若已经逻辑删除，则解锁A回到Step1
		b.mu.Lock()
		if atomic.LoadUint32(&b.Deleted) == 1 {
			b.mu.Unlock()
			continue
		}

		// 3、锁定节点A，检查A.next!=B or a.marked，如果为真，则解锁A和B回到Step1
		a.mu.Lock()
		if a.next != b || atomic.LoadUint32(&a.Deleted) == 1 {
			a.mu.Unlock()
			b.mu.Unlock()
			continue
		}

		break
	}

	// 4、逻辑删除B，A.next=B.next
	atomic.StoreUint32(&b.Deleted , 1)
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&a.next)), unsafe.Pointer(b.next))
	atomic.AddInt64(&l.length, -1)

	// 5、解锁A和B
	a.mu.Unlock()
	b.mu.Unlock()

	return true
}

func (l *IntList) Contains(value int) bool {

	a := l.head
	b := (*intNode)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&a.next))))

	for b != nil && b.value < value {
		a = b
		b = (*intNode)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&a.next))))
	}
	if b == nil {
		return false
	}
	if b.value != value {
		return false
	}
	if atomic.LoadUint32(&b.Deleted) == 1 {
		return false
	}
	return true
}

func (l *IntList) Range(f func(value int) bool) {
	a := l.head
	b := (*intNode)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&a.next))))
	for b != nil {
		if !f(b.value) {
			break
		}
		a = b
		b = (*intNode)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&a.next))))
	}
}

func (l *IntList) Len() int {
	return int(atomic.LoadInt64(&l.length))
}
