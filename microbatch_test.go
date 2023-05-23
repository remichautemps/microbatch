package microbatch

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// DummyBatchProcessor is a dummy implementation of the BatchProcessor interface
type DummyBatchProcessor struct{}

// ProcessBatch processes a batch of jobs (dummy implementation)
func (bp *DummyBatchProcessor) ProcessBatch(batch []Job) []JobResult {
	results := make([]JobResult, len(batch))

	for i, job := range batch {
		results[i] = JobResult{JobID: job.ID, Result: fmt.Sprintf("Result of job %d", job.ID)}
	}
	return results
}

// DummyErroredBatchProcessor is a dummy implementation of the BatchProcessor interface with Errors
type DummyErroredBatchProcessor struct{}

// ProcessBatch processes a batch of jobs (dummy implementation)
func (bp *DummyErroredBatchProcessor) ProcessBatch(batch []Job) []JobResult {
	results := make([]JobResult, len(batch))

	for i, job := range batch {
		results[i] = JobResult{JobID: job.ID, Result: fmt.Sprintf("Failed Job %d", job.ID), Err: errors.New(fmt.Sprintf("BatchProcessor was not able to process job #%d", job.ID))}
	}
	return results
}

type TestCases struct {
	caseName            string
	nbJobs              int
	batchSize           int
	batchCycle          time.Duration
	delayBeforeShutdown time.Duration
	delayBeforeSubmit   time.Duration
	withError           bool
	expectedResults     []JobResult
	expectedError       error
}

func Test_Cases(t *testing.T) {
	cases := []TestCases{
		{
			// This test process 1 job1 by batch of 1, then shutdown.
			caseName:            "One Job",
			nbJobs:              1,
			batchSize:           1,
			batchCycle:          time.Millisecond,
			delayBeforeShutdown: time.Duration(100) * time.Millisecond,
			withError:           false,
			expectedResults: []JobResult{
				{JobID: 1, Result: "Result of job 1"},
			},
		},
		{
			// This test processes 7 jobs by batch of 2. It processes 2 jobs in the first batch cycle,
			// then shutdown after 100 milliseconds.
			// We expect all remaining 5 jobs to be processed.
			caseName:            "Shutdown and process remaining jobs",
			nbJobs:              7,
			batchSize:           2,
			batchCycle:          time.Millisecond * 100,
			delayBeforeShutdown: time.Duration(100) * time.Millisecond,
			withError:           false,
			expectedResults: []JobResult{
				{JobID: 1, Result: "Result of job 1"},
				{JobID: 2, Result: "Result of job 2"},
				{JobID: 3, Result: "Result of job 3"},
				{JobID: 4, Result: "Result of job 4"},
				{JobID: 5, Result: "Result of job 5"},
				{JobID: 6, Result: "Result of job 6"},
				{JobID: 7, Result: "Result of job 7"},
			},
		},
		{
			caseName:            "Errors in ResultJob",
			nbJobs:              1,
			batchSize:           2,
			batchCycle:          time.Millisecond * 100,
			delayBeforeShutdown: time.Duration(100) * time.Millisecond,
			withError:           true,
			expectedResults: []JobResult{
				{JobID: 1, Result: "Failed Job 1", Err: errors.New(fmt.Sprintf("BatchProcessor was not able to process job #%d", 1))},
			},
		},
	}

	// Run all the test cases
	for _, c := range cases {
		fmt.Println("\n>>> TEST ", c.caseName)

		results, err := setup(c.batchSize, c.batchCycle, c.nbJobs, c.delayBeforeShutdown, c.delayBeforeSubmit, c.withError)
		if err != nil {
			t.Fatalf("unexpected error when instantiating microbatch: %v", err)
		}

		compare(t, results, c.expectedResults)
	}
}

func Test_Errors(t *testing.T) {
	cases := []TestCases{
		{
			caseName:            "Send Jobs after shutdown",
			nbJobs:              2,
			batchSize:           1,
			batchCycle:          time.Millisecond * 100,
			delayBeforeShutdown: time.Duration(1) * time.Millisecond,
			delayBeforeSubmit:   time.Duration(1) * time.Second,
			withError:           false,
			expectedError:       errors.New("microBatch is not open for new jobs"),
		},
		{
			// This test process 1 job1 by batch of 1, then shutdown.
			caseName:            "Batch size of 0",
			nbJobs:              1,
			batchSize:           0,
			batchCycle:          time.Millisecond,
			delayBeforeShutdown: time.Duration(100) * time.Millisecond,
			withError:           false,
			expectedResults:     []JobResult{},
			expectedError:       errors.New("batchSize cannot be zero"),
		},
	}
	// Run all the test cases
	for _, c := range cases {
		fmt.Println("\n>>> TEST ", c.caseName)

		_, err := setup(c.batchSize, c.batchCycle, c.nbJobs, c.delayBeforeShutdown, c.delayBeforeSubmit, c.withError)

		if err.Error() != c.expectedError.Error() {
			t.Fatalf("error expected: %v", err)
		}
	}
}

// Compare check the size of the results and expected results and do a one by one comparison.
func compare(t *testing.T, results []JobResult, expectedResults []JobResult) {
	if len(results) != len(expectedResults) {
		t.Errorf("Expected %d results, but got %d", len(expectedResults), len(results))
	}

	for i, result := range results {
		expected := expectedResults[i]
		if result.JobID != expected.JobID || result.Result != expected.Result {
			t.Errorf("Mismatched result for JobID %d: expected (%d, %s), got (%d, %s)",
				expected.JobID, expected.JobID, expected.Result, result.JobID, result.Result)
		}
	}
}

func setup(batchSize int, batchCycle time.Duration, nbJobs int, delayBeforeShutdown, delayBeforeSubmit time.Duration, withErrors bool) ([]JobResult, error) {
	batchProcessor := &DummyBatchProcessor{}
	mb, err := New(batchSize, batchCycle, batchProcessor)
	if err != nil {
		return nil, err
	}
	if withErrors {
		mb, _ = New(batchSize, batchCycle, &DummyErroredBatchProcessor{})
	}

	// Start the MicroBatch
	mb.Start()
	go func() {
		time.Sleep(delayBeforeShutdown)
		mb.Shutdown()
	}()

	// Create mock jobs
	var jobs []Job
	for i := 1; i <= nbJobs; i++ {
		jobs = append(jobs, Job{ID: i, Payload: fmt.Sprintf("Job %v", i)})
	}

	// Submit jobs
	var errs error
	time.Sleep(delayBeforeSubmit)
	errs = mb.SubmitJob(jobs...)

	// Wait for the results
	var results []JobResult
	for result := range mb.GetResults() {
		results = append(results, result)
	}
	return results, errs
}
