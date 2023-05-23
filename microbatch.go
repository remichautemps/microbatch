package microbatch

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Job represents a single task to be processed
type Job struct {
	ID      int
	Payload string
}

// JobResult represents the result of a processed job
type JobResult struct {
	JobID  int
	Result interface{}
	Err    error
}

// BatchProcessor represents the external component responsible for processing batches of jobs
type BatchProcessor interface {
	ProcessBatch(batch []Job) []JobResult
}

// MicroBatch represents the micro-batching library
type MicroBatch struct {
	batchSize    int
	batchCycle   time.Duration
	bp           BatchProcessor
	jobStream    chan Job
	resultStream chan JobResult
	stop         chan struct{}
	wg           sync.WaitGroup
	open         bool
	mu           sync.Mutex
}

// New creates a new MicroBatch with the given batch size, batchCycle, and BatchProcessor
func New(batchSize int, batchCycle time.Duration, bp BatchProcessor) (*MicroBatch, error) {
	if batchSize <= 0 {
		return nil, errors.New("batchSize cannot be zero")
	}
	return &MicroBatch{
		batchSize:    batchSize,
		bp:           bp,
		batchCycle:   batchCycle,
		jobStream:    make(chan Job),
		resultStream: make(chan JobResult),
		stop:         make(chan struct{}),
		open:         true,
	}, nil
}

// SubmitJob submits a job or a bulk of job to be batched
func (mb *MicroBatch) SubmitJob(jobs ...Job) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	if !mb.isOpen() {
		return errors.New("microBatch is not open for new jobs")
	}
	for _, b := range jobs {
		mb.jobStream <- b
	}
	return nil
}

// isOpen checks if the job channel is opened
func (mb *MicroBatch) isOpen() bool {
	return mb.open
}

// GetResults returns the channel to receive job results
func (mb *MicroBatch) GetResults() <-chan JobResult {
	return mb.resultStream
}

// Start starts the MicroBatch
func (mb *MicroBatch) Start() {
	mb.wg.Add(1)
	go mb.processJobs()
}

// Shutdown stops the MicroBatch and waits for all jobs to be processed
func (mb *MicroBatch) Shutdown() {
	fmt.Println("Start of the shutdown process")
	mb.mu.Lock()
	mb.open = false
	mb.mu.Unlock()
	close(mb.stop)
}

// processJobs continuously processes incoming jobs
func (mb *MicroBatch) processJobs() {
	defer mb.wg.Done()

	var batch []Job
	ticker := time.NewTicker(mb.batchCycle)
	size := mb.batchSize

	for {
		select {
		case job, open := <-mb.jobStream:
			if !open {
				// Job stream closed, process any remaining jobs
				fmt.Println("Job stream closed")
				mb.flush(batch)
				return
			}

			batch = append(batch, job)

		case <-ticker.C:
			if len(batch) > 0 {
				if len(batch) < mb.batchSize {
					size = len(batch)
				}
				mb.handleBatch(batch[:size])
				batch = batch[size:]
			}

		case <-mb.stop:
			fmt.Println("Shutting down... Remaining jobs: ", len(batch))
			mb.mu.Lock()
			mb.open = false
			mb.mu.Unlock()
			close(mb.jobStream)
			mb.flush(batch)
			mb.wg.Wait()
			return
		}
	}
}

// flush handles the remaining jobs gracefully when a shutdown command is sent
func (mb *MicroBatch) flush(batch []Job) {
	size := mb.batchSize
	for {
		select {
		case <-time.Tick(mb.batchCycle):
			if len(batch) > 0 {
				// Now we add the batch slice logic here
				if len(batch) < size {
					size = len(batch)
				}
				mb.handleBatch(batch[:size])
				batch = batch[size:]
			} else {
				fmt.Println("Closing Result stream")
				close(mb.resultStream)
				return
			}
		}
	}
}

// handleBatch processes a batch of jobs using the external BatchProcessor and sends the results to the result stream
func (mb *MicroBatch) handleBatch(batch []Job) {
	fmt.Printf("Processing batch of size %d\n", len(batch))

	results := mb.bp.ProcessBatch(batch)

	for i, result := range results {
		mb.resultStream <- JobResult{JobID: batch[i].ID, Result: result.Result}
	}
}
