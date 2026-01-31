package handlers

import (
	"sync"
	"time"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/services"
)

// JobStatus represents the current state of a job
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
)

// Job represents an async analysis job
type Job struct {
	ID        string
	Status    JobStatus
	Result    *AnalyzeResult
	Error     string
	CreatedAt time.Time
}

// AnalyzeResult is the response structure for completed jobs
type AnalyzeResult struct {
	EnhancedImageURL string                  `json:"enhancedImageUrl"`
	Analysis         services.AnalysisResult `json:"analysis"`
	InitialAdvice    string                  `json:"initialAdvice"`
}

// JobStore manages async jobs in memory
type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewJobStore creates a new job store
func NewJobStore() *JobStore {
	return &JobStore{
		jobs: make(map[string]*Job),
	}
}

// Create creates a new pending job and returns its ID
func (s *JobStore) Create(id string) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()

	job := &Job{
		ID:        id,
		Status:    JobStatusPending,
		CreatedAt: time.Now(),
	}
	s.jobs[id] = job
	return job
}

// Get retrieves a job by ID
func (s *JobStore) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[id]
	return job, ok
}

// SetProcessing marks a job as processing
func (s *JobStore) SetProcessing(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job, ok := s.jobs[id]; ok {
		job.Status = JobStatusProcessing
	}
}

// SetCompleted marks a job as completed with a result
func (s *JobStore) SetCompleted(id string, result *AnalyzeResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job, ok := s.jobs[id]; ok {
		job.Status = JobStatusCompleted
		job.Result = result
	}
}

// SetFailed marks a job as failed with an error message
func (s *JobStore) SetFailed(id string, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if job, ok := s.jobs[id]; ok {
		job.Status = JobStatusFailed
		job.Error = errMsg
	}
}

// Cleanup removes jobs older than the given duration
func (s *JobStore) Cleanup(maxAge time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, job := range s.jobs {
		if job.CreatedAt.Before(cutoff) {
			delete(s.jobs, id)
		}
	}
}

// Global job store instance
var globalJobStore = NewJobStore()

// GetJobStore returns the global job store
func GetJobStore() *JobStore {
	return globalJobStore
}
