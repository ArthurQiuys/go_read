package main

// 导入所需包
import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"

	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

type baseBucket struct {
	success   int64
	fail      int64
	timeout   int64
	rejection int64
}

type bucket struct {
	baseBucket  baseBucket
	windowStart int32
}

type SlidingWindow struct {
	buckets   []*bucket
	width     int32
	buckWidth int32
	tail      int32
	mux       sync.RWMutex
}

func NewSlidingWindow(width, buckWidth int32) *SlidingWindow {
	if width < 1 {
		width = 1
	}
	if buckWidth < 1 {
		buckWidth = 1
	}
	return &SlidingWindow{
		width:     width,
		buckWidth: buckWidth,
		buckets:   make([]*bucket, width),
		tail:      0,
	}
}

func (sldwindow *SlidingWindow) getCurrentBucket() *bucket {
	sldwindow.mux.Lock()
	defer sldwindow.mux.Unlock()
	currentSecondTime := time.Now().Unix()
	if sldwindow.tail == 0 && sldwindow.buckets[sldwindow.tail] == nil {
		sldwindow.tail = 0
		sldwindow.buckets[sldwindow.tail] = &bucket{
			baseBucket:  baseBucket{},
			windowStart: int32(currentSecondTime),
		}
		return sldwindow.buckets[sldwindow.tail]
	}
	tail := sldwindow.buckets[sldwindow.tail]
	if int64(tail.windowStart+sldwindow.buckWidth) > currentSecondTime {
		return tail
	}

	for i := int32(0); i < sldwindow.width; i++ {
		tail := sldwindow.buckets[sldwindow.tail]
		if int64(tail.windowStart+sldwindow.buckWidth) > currentSecondTime {
			return tail
		} else if (currentSecondTime - int64((tail.windowStart + sldwindow.buckWidth))) > int64(sldwindow.width*sldwindow.buckWidth) {
			sldwindow.tail = 0
			sldwindow.buckets = make([]*bucket, sldwindow.width)
			return &bucket{
				baseBucket:  baseBucket{},
				windowStart: int32(currentSecondTime),
			}
		} else {
			sldwindow.tail++
			bucket := &bucket{
				baseBucket:  baseBucket{},
				windowStart: tail.windowStart + sldwindow.buckWidth,
			}
			if sldwindow.tail >= sldwindow.width {
				copy(sldwindow.buckets[:], sldwindow.buckets[1:])
				sldwindow.tail--
			}
			sldwindow.buckets[sldwindow.tail] = bucket
		}
	}
	return sldwindow.buckets[sldwindow.tail]
}

func (sldwindow *SlidingWindow) incrSuccess() {
	bucket := sldwindow.getCurrentBucket()
	atomic.AddInt64(&bucket.baseBucket.success, 1)
}

func (sldwindow *SlidingWindow) incFail() {
	bucket := sldwindow.getCurrentBucket()
	atomic.AddInt64(&bucket.baseBucket.fail, 1)
}

func (sldwindow *SlidingWindow) incrTimeOut() {
	bucket := sldwindow.getCurrentBucket()
	atomic.AddInt64(&bucket.baseBucket.timeout, 1)
}

func (sldwindow *SlidingWindow) incrReject() {
	bucket := sldwindow.getCurrentBucket()
	atomic.AddInt64(&bucket.baseBucket.rejection, 1)
}

func main() {
	group, _ := errgroup.WithContext(context.Background())

	rw := NewSlidingWindow(20, 4)
	fmt.Println(time.Now().Unix())
	group.Go(
		func() error {
			rand.Seed(time.Now().UnixNano())
			num := rand.Intn(1000)
			println("number is: %s", num)
			for i := 0; i < num; i++ {
				rw.incrSuccess()
				time.Sleep(time.Millisecond * 3)
			}
			return nil
		},
	)
	group.Go(
		func() error {
			rand.Seed(time.Now().UnixNano())
			num := rand.Intn(333)
			println("number is: %s", num)
			for i := 0; i < num; i++ {
				rw.incFail()
				time.Sleep(time.Millisecond * 3)
			}

			return nil
		},
	)
	group.Go(
		func() error {
			rand.Seed(time.Now().UnixNano())
			num := rand.Intn(222)
			println("number is: %s", num)
			for i := 0; i < num; i++ {
				rw.incrTimeOut()
				time.Sleep(time.Millisecond * 3)
			}

			return nil
		},
	)
	group.Go(
		func() error {
			rand.Seed(time.Now().UnixNano())
			num := rand.Intn(111)
			println("number is: %s", num)
			for i := 0; i < num; i++ {
				rw.incrReject()
				time.Sleep(time.Millisecond * 3)
			}

			return nil
		},
	)

	if err := group.Wait(); err != nil {
		fmt.Println("something has error", err)
	} else {
		stat := baseBucket{}
		for _, bucket := range rw.buckets {
			if bucket != nil {
				stat.success += bucket.baseBucket.success
				stat.fail += bucket.baseBucket.fail
				stat.timeout += bucket.baseBucket.timeout
				stat.rejection += bucket.baseBucket.rejection
			}
		}
		fmt.Printf("打印统计结果 %v \n", stat)
	}
}
