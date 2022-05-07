//go:build unit
// +build unit

package worker

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_WorkerStatus(t *testing.T) {

	// reset the workerStatusManager for testing
	workerStatusManager = NewWorkerStatusManager()

	workerStatusManager.SetWorkerStatus("worker1", STATUS_STARTED)
	workerStatusManager.SetWorkerStatus("worker2", STATUS_STARTED)
	workerStatusManager.SetWorkerStatus("worker1", STATUS_INITIALIZED)
	workerStatusManager.SetWorkerStatus("worker2", STATUS_INIT_FAILED)
	assert.Equal(t, 4, len(workerStatusManager.StatusLog), "There should be 4 log entries.")
	assert.Equal(t, 2, len(workerStatusManager.Workers), "There should be 2 workers.")
	assert.Equal(t, STATUS_INITIALIZED, workerStatusManager.GetWorkerStatus("worker1"), "The status for worker1 should be "+STATUS_INITIALIZED)
	assert.Equal(t, STATUS_INIT_FAILED, workerStatusManager.GetWorkerStatus("worker2"), "The status for worker2 should be "+STATUS_INIT_FAILED)
	assert.Equal(t, "", workerStatusManager.GetWorkerStatus("worker3"), "The status for worker3 should be an empty string")
}

func Test_SubworkerStatus(t *testing.T) {

	// reset the workerStatusManager for testing
	workerStatusManager = NewWorkerStatusManager()

	workerStatusManager.SetWorkerStatus("worker1", STATUS_STARTED)
	workerStatusManager.SetWorkerStatus("worker2", STATUS_STARTED)
	workerStatusManager.SetWorkerStatus("worker1", STATUS_INITIALIZED)
	workerStatusManager.SetWorkerStatus("worker2", STATUS_INIT_FAILED)
	workerStatusManager.SetSubworkerStatus("worker1", "sub1", STATUS_ADDED)
	workerStatusManager.SetSubworkerStatus("worker1", "sub2", STATUS_ADDED)
	workerStatusManager.SetSubworkerStatus("worker1", "sub2", STATUS_TERMINATED)
	workerStatusManager.SetSubworkerStatus("worker2", "sub1", STATUS_ADDED)
	workerStatusManager.SetSubworkerStatus("worker2", "sub2", STATUS_ADDED)
	workerStatusManager.SetSubworkerStatus("worker2", "sub1", STATUS_TERMINATING)
	workerStatusManager.SetSubworkerStatus("worker3", "sub1", STATUS_ADDED)

	assert.Equal(t, 11, len(workerStatusManager.StatusLog), "There should be 11 log entries.")
	assert.Equal(t, 3, len(workerStatusManager.Workers), "There should be 3 workers.")

	assert.Equal(t, STATUS_INITIALIZED, workerStatusManager.GetWorkerStatus("worker1"), "The status for worker1 should be "+STATUS_INITIALIZED)
	assert.Equal(t, STATUS_INIT_FAILED, workerStatusManager.GetWorkerStatus("worker2"), "The status for worker2 should be "+STATUS_INIT_FAILED)
	assert.Equal(t, STATUS_NONE, workerStatusManager.GetWorkerStatus("worker3"), "The status for worker3 should be "+STATUS_NONE)

	assert.Equal(t, STATUS_ADDED, workerStatusManager.GetSubworkerStatus("worker1", "sub1"), "The status for worker1 subworker sub1 one should be "+STATUS_ADDED)
	assert.Equal(t, STATUS_TERMINATED, workerStatusManager.GetSubworkerStatus("worker1", "sub2"), "The status for worker1 subworker sub2 should be "+STATUS_TERMINATED)
	assert.Equal(t, STATUS_TERMINATING, workerStatusManager.GetSubworkerStatus("worker2", "sub1"), "The status for worker2 subworker sub1 one should be "+STATUS_TERMINATING)
	assert.Equal(t, STATUS_ADDED, workerStatusManager.GetSubworkerStatus("worker2", "sub2"), "The status for worker2 subworker sub2 should be "+STATUS_ADDED)
	assert.Equal(t, STATUS_ADDED, workerStatusManager.GetSubworkerStatus("worker3", "sub1"), "The status for worker3 subworker sub2 should be "+STATUS_ADDED)
}
