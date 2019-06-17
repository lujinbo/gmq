package mq

import (
	"fmt"
	"go-mq/logs"
	"go-mq/utils"
	"sync"
	"time"
)

var (
	timerDefaultDuration = 1 * time.Second
	timerResetDuration   = 5 * time.Second
	timerSleepDuration   = 24 * time.Hour
)

type bucket struct {
	sync.Mutex
	id              string
	jobNum          int
	nextTime        time.Time
	recvJob         chan *JobCard
	addToReadyQueue chan string
	resetTimerChan  chan struct{}
}

type ByNum []*bucket

func (a ByNum) Len() int           { return len(a) }
func (a ByNum) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByNum) Less(i, j int) bool { return a[i].jobNum < a[j].jobNum }

func (b *bucket) Key() string {
	return GetBucketKeyById(b.id)
}

func (b *bucket) run() {
	go b.retrievalTimeoutJobs()

	for {
		select {
		case card := <-b.recvJob:
			b.Lock()
			if err := db.AddToBucket(b, card); err != nil {
				// job添加到bucket失败了,要怎么处理???
				fmt.Println("添加bucket失败", err)
				log.Error(fmt.Sprintf("Add to bucket failed, the error is %v", err))
				b.Unlock()
				continue
			}

			// 如果bucket下次扫描检索时间比新job的delay相差5秒以上,则重置定时器,
			// 确保新的job能够即时添加到readyQueue,设置5秒间隔可以防止频繁重置定时器
			subTime := b.nextTime.Sub(time.Now())
			if subTime > 0 && subTime-time.Duration(card.delay) > timerResetDuration {
				log.Debug(logs.LogCategory("resetTimer"), fmt.Sprintf("bid:%v,resettime", b.id))
				b.resetTimerChan <- struct{}{}
			}

			db.SetJobStatus(card.id, JOB_STATUS_DELAY)
			b.jobNum++
			b.Unlock()
		case jobId := <-b.addToReadyQueue:
			if err := db.AddToReadyQueue(jobId); err != nil {
				// 添加ready queue失败了,要怎么处理
				log.Error(err)
				continue
			}

			db.SetJobStatus(jobId, JOB_STATUS_READY)
			b.jobNum--
		}
	}
}

// 检索到时任务
func (b *bucket) retrievalTimeoutJobs() {
	var (
		duration = timerDefaultDuration
		timer    = time.NewTimer(duration)
	)

	for {
		select {
		case <-timer.C:
			jobIds, nextTime, err := db.RetrivalTimeoutJobs(b)
			if err != nil {
				log.Error(fmt.Sprintf("bucketId: %v retrival failed, error:%v", b.Key(), err))
				timer.Reset(duration)
				break
			}

			// 若addToReadyQueue处理太慢,注意这里会阻塞
			// 不要单独的goroutine去处理,会导致addToReadyQueue堆积太多job
			for _, jobId := range jobIds {
				b.addToReadyQueue <- jobId
			}

			if nextTime == -1 {
				duration = timerSleepDuration
			} else {
				duration = time.Duration(nextTime) * time.Second
			}

			b.nextTime = time.Now().Add(duration)
			logInfo := fmt.Sprintf("%v,nexttime:%v", b.Key(), utils.FormatTime(b.nextTime))
			log.Info(logs.LogCategory("retrivaltime"), logInfo)

			timer.Reset(duration)
		case <-b.resetTimerChan:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}

			b.nextTime = time.Now().Add(timerDefaultDuration)
			timer.Reset(timerDefaultDuration)
		}
	}
}
