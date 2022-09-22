package combine

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/legenove/keeper"
)

var groupId uint64 = 0

type group struct {
	id     string
	mu     sync.RWMutex
	mapper map[string]*combine
	conf   *Config
	req    Requester
}

func NewGroupWithOption(req Requester, options ...Option) *group {
	g := &group{
		id:     strconv.FormatUint(atomic.AddUint64(&groupId, 1), 10),
		req:    req,
		conf:   &Config{},
		mapper: make(map[string]*combine, 256),
	}
	for i := range options {
		options[i](g.conf)
	}
	keeper.SetKeeper("legenove.guardian."+g.id+".combiner.group", g.clearNoAlive, time.Second*20, false)
	return g
}

func (g *group) GetCombine(key string) *combine {
	if c, ok := g.getCombine(key); ok {
		// r.Alive() inline
		if atomic.LoadUint32(&c.status) == CbStatusWait || time.Now().UnixNano() < c.doneTime+c.conf.aliveTime {
			return c
		}
	}
	return g.newCombine(key)
}

func (g *group) getCombine(key string) (*combine, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	c, ok := g.mapper[key]
	return c, ok
}

func (g *group) newCombine(key string) *combine {
	g.mu.Lock()
	defer g.mu.Unlock()
	if c, ok := g.mapper[key]; ok {
		// r.Alive() inline
		if atomic.LoadUint32(&c.status) == CbStatusWait || time.Now().UnixNano() < c.doneTime+c.conf.aliveTime {
			return c
		}
	}
	c := NewCombineWithConfig(g.req, g.conf)
	g.mapper[key] = c
	return c
}

func (g *group) clearNoAlive() error {
	var times = 3
	g.mu.Lock()
	defer g.mu.Unlock()
	count := 1000
	for i := 0; i < times; i++ {
		ctu := true
		if len(g.mapper) <= count {
			ctu = false
		}
		for k, c := range g.mapper {
			if count == 0 {
				break
			}
			count--
			// "Inlining" of expired
			if atomic.LoadUint32(&c.status) == CbStatusWait || time.Now().UnixNano() < c.doneTime+c.conf.aliveTime {
				continue
			}
			delete(g.mapper, k)
		}
		if !ctu {
			break
		}
	}
	return nil
}
