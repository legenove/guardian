package combine

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// 聚合限流器

const (
	CbStatusWait  uint32 = iota // 等待
	CbStatusDone                // 完成-成功
	CbStatusError               // 完成-错误
)

const (
	defaultAllowN   = 3
	defaultMaxWaitN = 1000
	defaultTimeout  = 1 * time.Second
)

type Requester interface {
	GetInfo(interface{}) (interface{}, error)
}

type cInfo struct {
	result interface{}
	err    error
}

type combine struct {
	conf     *Config
	req      Requester
	mu       sync.RWMutex
	doneN    int32         // 完成数量
	status   uint32        // 状态
	ch       chan struct{} // 完成信号
	resch    chan cInfo    // 返回结果队列
	result   cInfo         // 结果数据
	waitCnt  int32         // 等待的数量
	doneTime int64         // 完成时间
}

func NewCombineWithConfig(req Requester, config *Config) *combine {
	c := &combine{
		req:  req,
		ch:   make(chan struct{}),
		conf: config,
	}
	c.init()
	return c
}

func NewCombineWithOption(req Requester, options ...Option) *combine {
	c := &combine{
		req:  req,
		ch:   make(chan struct{}),
		conf: &Config{},
	}
	for i := range options {
		options[i](c.conf)
	}
	c.init()
	return c
}

func (c *combine) init() {
	if c.conf.allowN <= 0 {
		c.conf.allowN = defaultAllowN
	}
	if c.conf.maxWaitN <= 0 {
		c.conf.maxWaitN = defaultMaxWaitN
	}
	if c.conf.timeout <= 0 {
		c.conf.timeout = defaultTimeout
	}
	c.resch = make(chan cInfo, c.conf.allowN)
}

func (c *combine) Alive() bool {
	if atomic.LoadUint32(&c.status) == CbStatusWait || time.Now().UnixNano() < c.doneTime+c.conf.aliveTime {
		return true
	}
	return false
}

func (c *combine) GetInfo(data interface{}) (interface{}, error) {
	if atomic.LoadUint32(&c.status) > CbStatusWait {
		return c.result.result, c.result.err
	}
	cnt := atomic.AddInt32(&c.waitCnt, 1)
	if cnt <= 127 && int8(cnt) <= c.conf.allowN {
		if cnt == 1 {
			go func() {
				// 处理结果
			loop:
				for {
					select {
					case ci := <-c.resch:
						d := atomic.AddInt32(&c.doneN, 1)
						if int8(d) < c.conf.allowN && ci.err != nil {
							continue
						}
						_st := CbStatusDone
						if ci.err != nil {
							_st = CbStatusError
						}
						c.mu.Lock()
						c.doneTime = time.Now().UnixNano()
						if !atomic.CompareAndSwapUint32(&c.status, CbStatusWait, _st) {
							c.mu.Unlock()
							break loop
						}
						c.result = ci
						close(c.ch)
						c.mu.Unlock()
						break loop
					}
				}
				c.mu.Lock()
				close(c.resch)
				c.mu.Unlock()
			}()
		}
		// 处理请求 # 阻塞
		res, err := c.req.GetInfo(data)
		c.mu.RLock()
		if atomic.LoadUint32(&c.status) == CbStatusWait {
			c.resch <- cInfo{result: res, err: err}
		}
		c.mu.RUnlock()
		goto wait
	}
	if cnt > c.conf.maxWaitN {
		c.mu.RLock()
		if atomic.LoadUint32(&c.status) > CbStatusWait {
			c.mu.RUnlock()
			return c.result.result, c.result.err
		}
		c.mu.RUnlock()
		// 超限制
		return nil, errors.New("combine QPS Limit")
	}
wait:
	// 等待回调
	select {
	case <-c.ch:
		c.mu.RLock()
		defer c.mu.RUnlock()
		// 成功or失败
		return c.result.result, c.result.err
	case <-c.conf.TimeAfter():
		return nil, errors.New("combine timeout")
	}
}
