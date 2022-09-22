package combine

import "time"

type Config struct {
	allowN    int8          // 允许通过数量
	maxWaitN  int32         // 最大等待数量
	timeout   time.Duration // 超时时间
	aliveTime int64         // 有效时间
	newTimer  timeAfter
}

var defaultTimeAfter = time.After

func (c *Config) TimeAfter() <-chan time.Time {
	if c.newTimer == nil {
		return defaultTimeAfter(c.timeout)
	}
	return c.newTimer(c.timeout)
}

type Option func(c *Config)
type timeAfter func(duration time.Duration) <-chan time.Time

func AllowNOption(n int8) Option {
	return func(c *Config) {
		c.allowN = n
	}
}

func AliveTimeOption(t time.Duration) Option {
	return func(c *Config) {
		c.aliveTime = int64(t)
	}
}

func MaxWaitNOption(n int32) Option {
	return func(c *Config) {
		c.maxWaitN = n
	}
}

func TimeoutOption(t time.Duration) Option {
	return func(c *Config) {
		c.timeout = t
	}
}

func newTimeAfterOption(f timeAfter) Option {
	return func(c *Config) {
		c.newTimer = f
	}
}
