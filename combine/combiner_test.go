package combine

import (
    "errors"
    "fmt"
    "sync"
    "sync/atomic"
    "testing"
    "time"
)

type mockRequest struct {
    c uint32
}

func (r *mockRequest) GetInfo(i interface{}) (interface{}, error) {
    time.Sleep(100 * time.Millisecond)
    for {
        if atomic.CompareAndSwapUint32(&r.c, 0, 1) {
            return nil, errors.New("error")
        }
        if atomic.CompareAndSwapUint32(&r.c, 1, 0) {
            return 1, nil
        }
    }
}

type mockOneRequest struct {
    c uint32
}

func (r *mockOneRequest) GetInfo(i interface{}) (interface{}, error) {
    for {
        if atomic.AddUint32(&r.c, 1) == 2 {
            return 1, nil
        }
        return nil, errors.New("error")
    }
}

func TestNewCombine(t *testing.T) {
    cb := NewCombineWithOption(&mockRequest{})
    sw := sync.WaitGroup{}
    for i := 0; i < 100; i++ {
        sw.Add(1)
        go func() {
            defer sw.Done()
            fmt.Println(cb.GetInfo(1))
        }()
    }
    sw.Wait()
}

func TestNewCombine1(t *testing.T) {
    cb := NewCombineWithOption(&mockOneRequest{})
    sw := sync.WaitGroup{}
    for i := 0; i < 100; i++ {
        sw.Add(1)
        go func() {
            defer sw.Done()
            r, e := cb.GetInfo(1)
            fmt.Println("return:", r, e)
        }()
    }
    sw.Wait()
}
