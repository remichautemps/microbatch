#MicroBatch 

MicroBatch is a micro-batching library that allows you to process individual tasks in small batches, 
improving throughput by reducing the number of requests made to a downstream system. 
It provides a simple and efficient way to submit jobs, process them in batches, 
and retrieve the results.

Requirements:

    ✓ It allows the caller to submit a single Job, and it should return a JobResult
    ✓ It processes accepted Jobs in batches using a BatchProcessor
       ✓ It does not implement BatchProcessor. This is a dependency of the library.
    ✓ It provides a way to configure the batching behaviour i.e. size and frequency
    ✓ It exposes a shutdown method that returns after all previously accepted Jobs are processed.

## Design

- The library exposes a New() method to create a new instance of a microBatch. 
The batchCycle (Frequency) and batchSize (Size of each batch) can be configured by 
passing them at instantiation. 
- The Start() method starts the processJobs routine responsible for ingesting jobs 
and grouping them in batches.
- It exposes a Shutdown method to “Flush” already accepted jobs and stop accepting new jobs.
- The library is using 2 channels.
- JobStream to receive incoming jobs from a caller
- ResultStream to publish results coming back from the batchProcessor (external dependency).
- A public method getResults give the caller access to the result channel.

## Installation

`git clone https://github.com/remichautemps/microbatch`

## How to use

```go
// Create a new microbatch instance
batchSize := 1
batchCycle := time.Second
bp := &BatchProcessor{}
mb, err := microbatch.New(batchSize, batchCycle, bp)
if err != nil {
    // Handle errors
}
mb.Start()

// Submit Jobs
job1 := microbatch.Job{ID: 1, Payload: "Job 1"}
job2 := microbatch.Job{ID: 2, Payload: "Job 2"}
mb.SubmitJob(job1, job2)

// Get Results
results := mb.GetResults()
for result := range results {
    fmt.Println(result)
}
```
## Tests

`go test`

## Dependency

The library relies on an external batchProcessor.

```go
    // Interface defining a batchProcessor.
    // BatchProcessor represents the external component responsible for processing batches of jobs
        type BatchProcessor interface {
        ProcessBatch(batch []microbatch.Job) []microbatch.JobResult
    }
```

```go
// Example of a mock batchProcessor.
type BatchProcessor struct{}

// ProcessBatch processes a batch of jobs (dummy implementation)
func (bp *BatchProcessor) ProcessBatch(batch []microbatch.Job) []microbatch.JobResult {
    results := make([]microbatch.JobResult, len(batch))

    for i, job := range batch {
        results[i] = microbatch.JobResult{JobID: job.ID, Result: fmt.Sprintf("Result of job %d", job.ID)}
    }
return results
}
```

## Improvements
- We could imagine having a configurable pool of workers to handle the ingestion of jobs.
- In an ideal setup, errors would be wrapped 
- JobResult could be abstracted more.