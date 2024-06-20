package timedTaskMgr

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type JobRunner interface {
	RunJob(context.Context) // called by taskManager. RunJob should handle job exit and error by itself
	Settings() (string, time.Time, time.Duration)
	// returns: 1. job name. 2. specified start time [startAt] 3. repeat [interval]
	// if [startAt] is valid and [interval] is 0 or less, job should be run only once
}

type jobDetail struct {
	job      JobRunner
	jobName  string
	startAt  time.Time     // specified start time
	interval time.Duration // repeat interval
	stopChan chan func()   // chan context.CancelFunc, to be able to stop the running job by user / taskManager
}

type TaskManager struct {
	jobChan     chan *jobDetail
	cancelAll   func() // set in startTaskMgrInBg(), used in StopAllJobs()
	lgr         *log.Logger
	debugMode   bool
}

type TaskConf struct { // used by ComboRecordUpdater, StockListUpdater
	TaskName       string
	StartTimeOfDay int
	Interval       int
	Testing        bool
}

// utility to check input conf values and set correct values based on default conf
func (defaultConf *TaskConf) SetValues(inputConf *TaskConf) (string, time.Time, time.Duration, bool) {
	taskStartAt := calcStartTime(defaultConf.StartTimeOfDay)
	taskInterval := time.Duration(defaultConf.Interval) * time.Second
	if inputConf == nil {
		return defaultConf.TaskName, taskStartAt, taskInterval, defaultConf.Testing
	}
	taskName := defaultConf.TaskName
	if inputConf.TaskName != "" {
		taskName = inputConf.TaskName
	}
	if (inputConf.StartTimeOfDay >= 0) && (inputConf.StartTimeOfDay < 24) {
		taskStartAt = calcStartTime(inputConf.StartTimeOfDay)
	}
	if inputConf.Testing { // allows frequent interval for testing purposes
		taskInterval = time.Duration(inputConf.Interval) * time.Second
	} else {
		if (inputConf.Interval >= 60) && (inputConf.Interval < 86400) {
			taskInterval = time.Duration(inputConf.Interval) * time.Second
		}
	}
	return taskName, taskStartAt, taskInterval, inputConf.Testing
}

func calcStartTime(timeOfDay int) time.Time { // utility to set time at timeOfDay hour of tomorrow
	if timeOfDay < 0 { // use default hour of -1, if we do not need to specify start time
		return time.Time{} // returns empty time.Time value if input hour value < 0
	}
	tomorrowTime := time.Now().AddDate(0, 0, 1)
	year, month, day := tomorrowTime.Date()
	return time.Date(year, month, day, timeOfDay, 0, 0, 0, time.UTC)
}

func SetupTaskManager(gctx context.Context, gwg *sync.WaitGroup, lggr *log.Logger, debugging bool) *TaskManager {
	tm := &TaskManager{
		jobChan:     make(chan *jobDetail, 1),
		lgr:         lggr,
		debugMode:   debugging,
	}
	tm.startTaskMgrInBg(gctx, gwg)
	return tm
}

func (tm *TaskManager) AddJob(jr JobRunner) (func(), error) {
	// returns the cancel func for the current job, facilitating user to stop the job
	jobname, startat, interval_duration := jr.Settings()
	invalid_start_time, interval_too_small := false, false
	if time.Now().After(startat) {
		invalid_start_time = true
	}
	if interval_duration < (100 * time.Millisecond) {
		interval_too_small = true
	}
	if invalid_start_time && interval_too_small {
		return nil, fmt.Errorf("AddJob(%s) params error: invalid startAt time and interval too small", jobname)
	}
	jd := &jobDetail{
		job:      jr,
		jobName:  jobname,
		startAt:  startat,
		interval: interval_duration,
		stopChan: make(chan func(), 1),
	}
	select {
	case tm.jobChan <- jd:
	default: // use default to avoid blocking in above case
		return nil, fmt.Errorf("AddJob(%s) error: sending job into jobChan failed", jobname)
	}
	var stopFunc func()
	waitCtx, cf := context.WithTimeout(context.Background(), time.Duration(100)*time.Millisecond)
	defer cf()
	select {
	case stopFunc = <-jd.stopChan:
	case <-waitCtx.Done():
		return nil, fmt.Errorf("AddJob(%s) error: getting job cancel func failed", jobname)
	}
	return stopFunc, nil
}

func (tm *TaskManager) startTaskMgrInBg(gctx context.Context, gwg *sync.WaitGroup) {
	allJobsCtx, cancelFn := context.WithCancel(gctx)
	tm.cancelAll = cancelFn // used in StopAllJobs()
	gwg.Add(1)              // must add(1) before go func()
	go func() {
		defer func() {
			cancelFn() // cancel all jobs
			gwg.Done()
			close(tm.jobChan) // close and drain the channel to prevent memory leak
			for range tm.jobChan {
			}
		}()
		tm.lgr.Print("timed Task Manager started\n")
		for {
			select {
			case jobEntry, jobOk := <-tm.jobChan:
				if jobOk {
					tm.startJob(allJobsCtx, jobEntry)
				}
			case <-allJobsCtx.Done(): // stopped by user
				tm.lgr.Print("timed Task Manager stopped\n")
				return
			}
			case <-gctx.Done(): // proc stopped by system
				tm.lgr.Print("timed Task Manager stopped\n")
				return
			}
		}
	}()
}

func (tm *TaskManager) startJob(allJobsCtx context.Context, jd *jobDetail) {
	currentJobCtx, currentCancelFunc := context.WithCancel(allJobsCtx)
	timerValid, tickerValid := false, false
	jobTimer, timerValid := createTimer(jd.startAt)
	jobTicker, tickerValid := createTicker(jd.interval)
	if timerValid { // ensures timer job runs before any ticker jobs
		tickerValid = false
	}
	select {
	case jd.stopChan <- currentCancelFunc: // return the cancel func to user for user controlled stopTask()
	default: // should not happeen. the channel has a buffer size of 1
		tm.lgr.Print(fmt.Sprintf("TaskManager startJob(%s) sending cancelFunc to job stop chan error\n", jd.jobName))
	}
	tm.lgr.Print("Info", fmt.Sprintf("TaskManager startJob(%s) started\n", jd.jobName))
	go func() {
		defer func() {
			currentCancelFunc()
			if tickerValid {
				jobTicker.Stop()
			}
		}()
		counter := 1
		for {
			select {
			case <-jobTimer.C:
				if timerValid { // nil jobTimer will cause panic, so must check if timer is valid (instead of dummy timer)
					jd.job.RunJob(currentJobCtx)
					stopTimer(jobTimer)
					timerValid = false
					jobTicker, tickerValid = createTicker(jd.interval) // re-create valid ticker after timer fired
					if tm.debugMode {
						tm.lgr.Print(fmt.Sprintf("TaskManager startJob(%s) timer_run, count:%d", jd.jobName, counter))
					}
					if !tickerValid {
						tm.lgr.Print(fmt.Sprintf("TaskManager startJob(%s) stopped. no active timer or ticker running\n", jd.jobName))
						return
					}
					counter++
				}
			case <-jobTicker.C:
				if tickerValid { // nil jobTicker will cause panic, so must check if ticker is valid (instead of dummy ticker)
					jd.job.RunJob(currentJobCtx)
					/* if tm.debugMode {
						tm.lgr.Print(fmt.Sprintf("TaskManager startJob(%s) ticker_run, count:%d", jd.jobName, counter))
					} */
					counter++
				} else {
					if !timerValid {
						tm.lgr.Print(fmt.Sprintf("TaskManager startJob(%s) stopped. no active timer or ticker running\n", jd.jobName))
						return
					}
				}
			case <-currentJobCtx.Done():
				tm.lgr.Print(fmt.Sprintf("TaskManager startJob(%s) stopped by context\n", jd.jobName))
				return
			}
		}
	}()
}

func createTimer(startAt time.Time) (*time.Timer, bool) {
	/* second return value indicating if timer is valid,
	nil timer will cause jobTicker.C panic, so return dummy timer and inform caller
	*/
	if startAt.After(time.Now()) {
		return time.NewTimer(time.Until(startAt)), true
	}
	dummyTimer := time.NewTimer(time.Until(time.Now().Add(3 * time.Minute)))
	stopTimer(dummyTimer)
	return dummyTimer, false // return a dummy and stopped timer
}

func stopTimer(jobTimer *time.Timer) {
	if jobTimer == nil {
		return
	}
	if !jobTimer.Stop() { // http://russellluo.com/2018/09/the-correct-way-to-use-timer-in-golang.html
		select {
		case <-jobTimer.C: // try to drain the channel
		default: // use default to avoid blocking in above case
		}
	}
}

func createTicker(td time.Duration) (*time.Ticker, bool) {
	/* second return value indicating if ticker is valid,
	nil ticker will cause jobTicker.C panic, so return dummy ticker and inform caller
	*/
	if td >= (100 * time.Millisecond) {
		return time.NewTicker(td), true // false means ticker is not dummy (actually useful)
	}
	dummyTicker := time.NewTicker(1 * time.Hour)
	stopTicker(dummyTicker)
	return dummyTicker, false // return a dummy and stopped ticker
}

func stopTicker(jobTicker *time.Ticker) {
	if jobTicker == nil {
		return
	}
	jobTicker.Stop()
	select {
	case <-jobTicker.C: // try to drain the channel
	default: // use default to avoid blocking in above case
	}
}

func (tm *TaskManager) StopManager() {
// stops the manager and all running jobs
	tm.cancelAll()
}
