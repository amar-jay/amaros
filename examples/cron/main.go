package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/node"
	"github.com/amar-jay/amaros/pkg/topic"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/robfig/cron/v3"
)

// This node demonstrates how to use the cron package to schedule tasks at specific times or intervals.
// It creates cron jobs that publish messages to a topic according to a given schedule, and continues
// publishing until a valid response is received on a corresponding response topic.

// Multiple cron jobs are managed using a map of cron entries, allowing dynamic creation and cancellation.

// The node subscribes to a request topic where it receives job configurations, including the cron schedule,
// target publish topic, expected response condition, and optional timeout or retry behavior.

// Jobs may be one-time or recurring. If a valid response is not received within the expected conditions, a
// warning is logged. Depending on configuration, the job may continue retrying until a valid response is
// received or the node shuts down.

// CronJobRequest configures a new scheduled job
type CronJobRequest struct {
	msgs.AMAROS_MSG
	JobID         string `json:"job_id" msgpack:"job_id"`
	Schedule      string `json:"schedule" msgpack:"schedule"`
	TargetTopic   string `json:"target_topic" msgpack:"target_topic"`
	Payload       string `json:"payload" msgpack:"payload"`
	ResponseTopic string `json:"response_topic,omitempty" msgpack:"response_topic,omitempty"`
	MaxRetries    int    `json:"max_retries,omitempty" msgpack:"max_retries,omitempty"`
}

// CronJobResponse defines the response that clears a job
type CronJobResponse struct {
	msgs.AMAROS_MSG
	JobID  string `json:"job_id" msgpack:"job_id"`
	Status string `json:"status" msgpack:"status"` // "success" halts further attempts
}

type JobState struct {
	EntryID    cron.EntryID
	Retries    int
	MaxRetries int
	Completed  bool
}

func main() {
	n := node.Init("cron_node")
	n.OnShutdown(func() {
		fmt.Println("Shutting down cron node")
	})
	n.DescribeTopics([]msgs.TopicMetadata{
		{
			Topic:         "/cron.request",
			Type:          msgs.GetType(CronJobRequest{}),
			Purpose:       "Schedule a new cron job with the given configuration",
			ResponseTopic: "/cron.response",
			ResponseType:  msgs.GetType(CronJobResponse{}),
		},
		{
			Topic:   "/cron.response",
			Type:    msgs.GetType(CronJobResponse{}),
			Purpose: "Receive responses that indicate cron job completion",
		},
	})

	c := cron.New(
		cron.WithParser(
			cron.NewParser(
				cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
			),
		),
	)
	c.Start()
	defer c.Stop()

	var mu sync.Mutex
	activeJobs := make(map[string]*JobState)

	requestMsg := &CronJobRequest{}

	// Listen for incoming job requests
	n.SubscribeWithCallback("/cron.request", requestMsg, func(ctx topic.CallbackContext) {
		jobID := requestMsg.JobID
		schedule := requestMsg.Schedule
		targetTopic := requestMsg.TargetTopic
		payloadStr := requestMsg.Payload
		responseTarget := requestMsg.ResponseTopic
		maxRetries := requestMsg.MaxRetries

		if jobID == "" {
			jobID, _ = gonanoid.New()
		}

		if jobID == "" || schedule == "" || targetTopic == "" {
			fmt.Println("[WARN] Received invalid /cron.request, missing minimum fields.")
			return
		}

		mu.Lock()
		// Cancel existing if replacing
		if state, exists := activeJobs[jobID]; exists {
			fmt.Printf("[INFO] Updating existing job: %s\n", jobID)
			c.Remove(state.EntryID)
		}

		state := &JobState{
			MaxRetries: maxRetries,
		}
		activeJobs[jobID] = state
		mu.Unlock()

		fmt.Printf("[INFO] Scheduling job '%s' (%s) -> %s\n", jobID, schedule, targetTopic)

		entryID, err := c.AddFunc(schedule, func() {
			mu.Lock()
			currentJob, exists := activeJobs[jobID]
			if !exists || currentJob.Completed {
				mu.Unlock()
				return // Job was already processed or removed
			}

			// Check retry limits
			if currentJob.MaxRetries > 0 && currentJob.Retries >= currentJob.MaxRetries {
				fmt.Printf("[WARN] Job %s reached max retries (%d). Canceling.\n", jobID, currentJob.MaxRetries)
				c.Remove(currentJob.EntryID)
				delete(activeJobs, jobID)
				mu.Unlock()
				return
			}

			currentJob.Retries++
			attempt := currentJob.Retries
			mu.Unlock()

			fmt.Printf("[%s] Trigger job ID: %s (attempt %d). Publishing to %s\n", time.Now().Format(time.Kitchen), jobID, attempt, targetTopic)

			// Dispatch
			pubMsg := &msgs.Message{Data: payloadStr}
			n.Publish(targetTopic, pubMsg)
		})

		if err != nil {
			fmt.Printf("[ERROR] Failed to schedule job %s: %v\n", jobID, err)
			mu.Lock()
			delete(activeJobs, jobID)
			mu.Unlock()
			return
		}

		mu.Lock()
		state.EntryID = entryID
		mu.Unlock()

		// If a response topic is defined, listen on it to clear the job
		if responseTarget != "" {
			fmt.Printf("[INFO] Job %s expects response on %s\n", jobID, responseTarget)
			respMsg := &CronJobResponse{}

			n.SubscribeWithCallback(responseTarget, respMsg, func(ctx topic.CallbackContext) {
				if respMsg.JobID == jobID && respMsg.Status == "success" {
					mu.Lock()
					if st, ok := activeJobs[jobID]; ok && !st.Completed {
						fmt.Printf("[INFO] Job %s received valid response. Marking complete.\n", jobID)
						st.Completed = true
						c.Remove(st.EntryID)
						delete(activeJobs, jobID)
					}
					mu.Unlock()
				} else if respMsg.JobID == jobID {
					fmt.Printf("[WARN] Job %s received non-success response: %s\n", jobID, respMsg.Status)
				}
			})
		}
	})

	fmt.Println("Cron node ready. Listening on /cron.request...")

	// Keep the process alive
	select {}
}
