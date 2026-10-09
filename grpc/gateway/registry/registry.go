package registry

import (
	"sync"
	"unsafe"

	"github.com/ez4bk/ezboot/grpc/gateway"
)

func FunctionPointer(r gateway.EndpointRegister) uintptr {
	return *(*uintptr)(unsafe.Pointer(&r))
}

type gwEndpointRegisterSet struct {
	lazy sync.Once
	set  map[uintptr]struct{}
}

func (s *gwEndpointRegisterSet) Add(fp uintptr) {
	s.lazy.Do(func() {
		s.set = make(map[uintptr]struct{})
	})

	s.set[fp] = struct{}{}
}

var gwEndpointRegisterGraph struct {
	initOnce sync.Once

	regs     map[uintptr]gateway.EndpointRegister
	edges    map[uintptr]*gwEndpointRegisterSet
	indegree map[uintptr]int
}

// AddRegister 新增一个网关注册器, 要求注册器需要在 prerequisites 之后注册
func AddRegister(register gateway.EndpointRegister, prerequisites ...gateway.EndpointRegister) {
	gwEndpointRegisterGraph.initOnce.Do(func() {
		gwEndpointRegisterGraph.regs = make(map[uintptr]gateway.EndpointRegister)
		gwEndpointRegisterGraph.edges = make(map[uintptr]*gwEndpointRegisterSet)
		gwEndpointRegisterGraph.indegree = make(map[uintptr]int)
	})

	fp := FunctionPointer(register)
	gwEndpointRegisterGraph.regs[fp] = register
	gwEndpointRegisterGraph.indegree[fp] += len(prerequisites)
	for _, prerequisite := range prerequisites {
		pfp := FunctionPointer(prerequisite)
		if _, ok := gwEndpointRegisterGraph.edges[pfp]; !ok {
			gwEndpointRegisterGraph.edges[pfp] = new(gwEndpointRegisterSet)
		}
		gwEndpointRegisterGraph.edges[pfp].Add(fp)
	}
}

func GatewayEndpointRegister() []gateway.HandlerOption {
	var queue []uintptr
	for fp := range gwEndpointRegisterGraph.regs {
		if gwEndpointRegisterGraph.indegree[fp] == 0 {
			queue = append(queue, fp)
		}
	}

	var curr uintptr
	var gwOptions []gateway.HandlerOption
	for len(queue) != 0 {
		curr, queue = queue[0], queue[1:]
		gwOptions = append(gwOptions, gateway.WithHandlerEndpointRegister(
			gwEndpointRegisterGraph.regs[curr]))

		if neighbor := gwEndpointRegisterGraph.edges[curr]; neighbor != nil {
			for next := range neighbor.set {
				gwEndpointRegisterGraph.indegree[next]--
				if gwEndpointRegisterGraph.indegree[next] == 0 {
					queue = append(queue, next)
				}
			}
		}
	}

	if len(gwOptions) != len(gwEndpointRegisterGraph.regs) {
		panic("due to a circular dependencies")
	}

	// 由于 gRPC-Gateway 使用头插法注册, 所以这里需要倒序一下
	for l, r := 0, len(gwOptions)-1; l < r; l, r = l+1, r-1 {
		gwOptions[l], gwOptions[r] = gwOptions[r], gwOptions[l]
	}
	return gwOptions
}
