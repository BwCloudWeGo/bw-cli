package snowFlake

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	workerBits  uint8 = 10
	numberBits  uint8 = 12
	workerMax   int64 = -1 ^ (-1 << workerBits) // 最大机器ID 1023
	numberMax   int64 = -1 ^ (-1 << numberBits) // 序列号最大值 4095
	timeShift   uint8 = workerBits + numberBits // 时间戳偏移 22位
	workerShift uint8 = numberBits              // worker偏移 12位
	startTime   int64 = 1525705533000           // 起始时间戳 毫秒

	// 双worker配置：一台机器占用2个workerId
	workerCount = 2
)

// SingleWorker 单个worker单元，独立维护时间戳、序列号
type SingleWorker struct {
	timestamp int64
	workerId  int64
	number    int64
}

// Snowflake 封装两个worker，主备切换
type Snowflake struct {
	mu           sync.Mutex
	workers      [workerCount]*SingleWorker
	currentIndex int // 当前正在使用的worker下标 0主 1备
}

// NewSnowflake 传入起始workerId，自动占用 workerId 和 workerId+1 两个节点
func NewSnowflake(baseWorkerId int64) (*Snowflake, error) {
	// 校验两个workerId都合法
	for i := int64(0); i < workerCount; i++ {
		wid := baseWorkerId + i
		if wid < 0 || wid > workerMax {
			return nil, errors.New("worker id out of range, cannot allocate two workers")
		}
	}

	snow := &Snowflake{
		currentIndex: 0,
	}
	// 初始化两个worker
	for i := 0; i < workerCount; i++ {
		snow.workers[i] = &SingleWorker{
			workerId:  baseWorkerId + int64(i),
			timestamp: 0,
			number:    0,
		}
	}
	return snow, nil
}

// NextId 获取分布式ID，自动处理时钟回拨
func (s *Snowflake) NextId() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixNano() / 1e6 // 当前毫秒时间
	currentWorker := s.workers[s.currentIndex]

	// 场景1：时间正常推进
	if now > currentWorker.timestamp {
		currentWorker.timestamp = now
		currentWorker.number = 0
		return s.generateId(currentWorker, now)
	}

	// 场景2：同一毫秒，序列号自增
	if now == currentWorker.timestamp {
		currentWorker.number++
		if currentWorker.number > numberMax {
			// 本毫秒序列号耗尽，自旋等到下一毫秒
			for now <= currentWorker.timestamp {
				now = time.Now().UnixNano() / 1e6
			}
			currentWorker.timestamp = now
			currentWorker.number = 0
		}
		return s.generateId(currentWorker, now)
	}

	// 场景3：时钟回拨 now < currentWorker.timestamp，切换备用worker
	s.currentIndex = 1 - s.currentIndex // 0<->1互换
	swapWorker := s.workers[s.currentIndex]

	// 使用切换后的worker生成ID
	if now > swapWorker.timestamp {
		swapWorker.timestamp = now
		swapWorker.number = 0
	} else if now == swapWorker.timestamp {
		swapWorker.number++
		if swapWorker.number > numberMax {
			for now <= swapWorker.timestamp {
				now = time.Now().UnixNano() / 1e6
			}
			swapWorker.timestamp = now
			swapWorker.number = 0
		}
	} else {
		// 极端情况：两个worker都被时钟回拨覆盖，自旋阻塞等待时钟追上
		for now <= swapWorker.timestamp {
			now = time.Now().UnixNano() / 1e6
		}
		swapWorker.timestamp = now
		swapWorker.number = 0
	}

	return s.generateId(swapWorker, now)
}

// generateId 拼装雪花ID
func (s *Snowflake) generateId(w *SingleWorker, now int64) int64 {
	return (now-startTime)<<timeShift | (w.workerId << workerShift) | w.number
}

func test() {
	// 机器分配基础workerId=1，自动占用 1、2 两个worker节点
	sf, err := NewSnowflake(1)
	if err != nil {
		panic(err)
	}
	fmt.Println(sf.NextId())

	//// 模拟正常生成ID
	//for i := 0; i < 5; i++ {
	//	fmt.Println(sf.NextId())
	//	time.Sleep(1 * time.Millisecond)
	//}
	//
	//// 模拟时钟回拨场景：人为修改系统时间往前调，再次生成ID会自动切备用worker
	//fmt.Println("=== 模拟时钟回拨后生成ID ===")
	//// 实际测试时手动修改系统时间往前，此处仅演示逻辑
	//for i := 0; i < 3; i++ {
	//	fmt.Println(sf.NextId())
	//}
}
