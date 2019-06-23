package mq

import (
	"errors"
	"sort"
	"strconv"

	"gopkg.in/ini.v1"
)

var (
	Dper              *Dispatcher
	ErrBucketNum      = errors.New("The number of buckets must be greater then 0")
	ErrDispacherNoRun = errors.New("Dispacher is not running")
)

// 添加Job到Job Pool
// 调度Job分配到bucket
// 管理bucket
type Dispatcher struct {
	conf        *ini.File
	addToBucket chan *JobCard
	bucket      []*Bucket
}

func init() {
	Dper = &Dispatcher{
		addToBucket: make(chan *JobCard),
	}
}

// job调度器,负责bucket分配
func (d *Dispatcher) Run() {
	defer gmq.wg.Done()
	defer func() {
		log.Error("dispatcher退出了")
	}()
	gmq.wg.Add(1)

	if gmq.running == 0 {
		return
	}
	if err := d.initBucket(); err != nil {
		panic(err)
	}

	for {
		select {
		case card := <-d.addToBucket:
			if card.delay > 0 {
				sort.Sort(ByNum(d.bucket))
				d.bucket[0].recvJob <- card
			} else {
				// 延迟时间<=0,直接添加到队列(作为普通队列使用)
				if err := AddToReadyQueue(card.id); err != nil {
					// 添加ready queue失败了,要怎么处理
					log.Error(err)
				} else {
					SetJobStatus(card.id, JOB_STATUS_READY)
				}
			}
		case <-gmq.notify:
			return
		}
	}
}

// 初始化bucket
func (d *Dispatcher) initBucket() error {
	n := 30
	if n <= 0 {
		return ErrBucketNum
	}
	for i := 0; i < n; i++ {
		b := &Bucket{
			Id:              strconv.Itoa(i),
			JobNum:          0,
			recvJob:         make(chan *JobCard),
			addToReadyQueue: make(chan string),
			resetTimerChan:  make(chan struct{}),
		}

		// 初始化job数量,可能上次执行到一半就终止了
		b.JobNum = GetBucketJobNum(b)
		go b.run()
		d.bucket = append(d.bucket, b)
	}
	return nil
}

// 添加任务到对象池
func (d *Dispatcher) AddToJobPool(j *Job) error {
	if err := j.CheckJobData(); err != nil {
		return err
	}
	if err := AddToJobPool(j); err != nil {
		return err
	}

	d.addToBucket <- j.Card()
	return nil
}

func (d *Dispatcher) GetBuckets() []*Bucket {
	return d.bucket
}
