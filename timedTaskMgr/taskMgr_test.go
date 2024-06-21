package timedTaskMgr

import (
	"context"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"
)

var (
	lg               *log.Logger
	gtx              context.Context
	globalCancelFunc context.CancelFunc
	gwg              *sync.WaitGroup
	tm               *TaskManager
	cancellers       []func()
)

type simpleJob struct {
	jobName  string
	startAt  time.Time     // specified start time
	interval time.Duration // repeat interval
	counter  int
}

func TestAll(t *testing.T) { // tested ok 2024-0508
	starttest()
	tm = SetupTaskManager(gtx, gwg, lg, true)
	t.Run("test_AddJobs", test_AddJobs)
	t.Run("test_StopSingleJob", test_StopSingleJob)
	t.Run("test_StopAllJobs", test_StopAllJobs)
	stoptest()
}

func test_AddJobs(t *testing.T) { // tested ok 2024-0508
	fmt.Println("")
	fmt.Println(" ----------- test_AddJobs AddJobs  ----------- ")
	for i := 1; i <= 9; i++ {
		startTime := time.Now().Add(time.Duration(i) * time.Second)
		job := simpleJob{ // must use a new job instead of reusing existing one, because AddJob() will use its address
			jobName:  fmt.Sprintf("job_%02d", i),
			startAt:  startTime,
			interval: time.Duration(i*200) * time.Millisecond,
			counter:  1,
		}
		cancelFunc, err := tm.AddJob(&job)
		if err != nil {
			t.Errorf("AddJob() error: %v\n", err)
			return
		}
		cancellers = append(cancellers, cancelFunc)
	}
}

func test_StopSingleJob(t *testing.T) { // tested ok 2024-0508
	time.Sleep(3 * time.Second)
	for i := 0; i < 3; i++ {
		fmt.Printf(" ----------- stopping / canceling job [%02d] ----------- \n", i+1)
		cancellers[i]()
	}
}

func test_StopAllJobs(t *testing.T) { // tested ok 2024-0508
	time.Sleep(3 * time.Second)
	fmt.Println(" ----------- stopping all jobs  ----------- ")
	tm.StopManager()
}

func (job *simpleJob) RunJob(ctx context.Context) {
	fmt.Println(" ----------- RunJob ----------- ")
	fmt.Printf("job: [%s], counter: [%02d], current time: %s\n\n", job.jobName, job.counter, time.Now().Format(time.DateTime))
	job.counter++
}

func (job *simpleJob) Settings() (string, time.Time, time.Duration) {
	return job.jobName, job.startAt, job.interval
}

func starttest() {
  lg = log.New(os.Stdout, "DebugLog: ", log.Ldate|log.Ltime)
	gtx, globalCancelFunc = context.WithCancel(context.Background())
	gwg = &sync.WaitGroup{}
}

func stoptest() {
	time.Sleep(1 * time.Second)
	fmt.Println("cancel global context")
	globalCancelFunc() // MUST cancel global context to stop all already running tasks
	gwg.Wait()
	fmt.Println("program stopped")
}
