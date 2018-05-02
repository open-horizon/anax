package worker

import (
	"fmt"
	"sync"
	"time"
)

const (
	STATUS_NONE        = "none"
	STATUS_ADDED       = "added"
	STATUS_STARTED     = "started"
	STATUS_INITIALIZED = "initialized"
	STATUS_INIT_FAILED = "initialization failed"
	STATUS_TERMINATING = "terminating"
	STATUS_TERMINATED  = "terminated"
)

var workerStatusManager = NewWorkerStatusManager()

func GetWorkerStatusManager() *WorkerStatusManager {
	return workerStatusManager
}

// reset the worker status manager for test purpose only
func resetWorkerStatusManager() {
	workerStatusManager = NewWorkerStatusManager()
}

// status for a worker
type WorkerStatus struct {
	Name            string            `json:"name"`
	Status          string            `json:"status"`
	SubworkerStatus map[string]string `json:"subworker_status"`
	StatusLock      sync.Mutex        `json:"-"` // The lock that protects modification from different threads at the same time
}

func (w *WorkerStatus) SetWorkerStatus(status string) {
	w.StatusLock.Lock()
	defer w.StatusLock.Unlock()

	w.Status = status
}

func (w *WorkerStatus) SetSubworkerStatus(name string, status string) {
	w.StatusLock.Lock()
	defer w.StatusLock.Unlock()

	w.SubworkerStatus[name] = status
}

type WorkerStatusManager struct {
	Workers     map[string]*WorkerStatus `json:"workers"`
	StatusLog   []string                 `json:"worker_status_log"`
	ManagerLock sync.Mutex               `json:"-"` // The lock that protects modification from different threads at the same time
}

func NewWorkerStatusManager() *WorkerStatusManager {
	return &WorkerStatusManager{
		Workers:   make(map[string]*WorkerStatus),
		StatusLog: make([]string, 0),
	}
}

func (w *WorkerStatusManager) SetWorkerStatus(name string, status string) {
	w.ManagerLock.Lock()
	defer w.ManagerLock.Unlock()

	if _, ok := w.Workers[name]; !ok {
		w.Workers[name] = &WorkerStatus{
			Name:            name,
			Status:          status,
			SubworkerStatus: make(map[string]string),
		}
	} else {
		w.Workers[name].SetWorkerStatus(status)
	}

	time_s := fmt.Sprintf(time.Now().Format("2006-01-02 15:04:05"))
	w.StatusLog = append(w.StatusLog, fmt.Sprintf("%v Worker %v: %v.", time_s, name, status))
}

func (w *WorkerStatusManager) SetSubworkerStatus(name string, subname string, status string) {
	w.ManagerLock.Lock()
	defer w.ManagerLock.Unlock()

	if _, ok := w.Workers[name]; !ok {
		w.Workers[name] = &WorkerStatus{
			Name:            name,
			Status:          STATUS_NONE,
			SubworkerStatus: make(map[string]string),
		}
	}
	w.Workers[name].SetSubworkerStatus(subname, status)

	time_s := fmt.Sprintf(time.Now().Format("2006-01-02 15:04:05"))
	w.StatusLog = append(w.StatusLog, fmt.Sprintf("%v Worker %v: subworker %v %v.", time_s, name, subname, status))
}

// Get the status string for the given worker. It returns an empty string if the worker does not exist.
func (w *WorkerStatusManager) GetWorkerStatus(name string) string {
	if ws, ok := w.Workers[name]; ok {
		return ws.Status
	} else {
		// returns an empty string if the worker does not exist.
		return ""
	}
}

// Get the status string for the given subworker. It returns an empty string if the subworker does not exist.
func (w *WorkerStatusManager) GetSubworkerStatus(name string, subname string) string {
	if ws, ok := w.Workers[name]; ok {
		if status, ok2 := ws.SubworkerStatus[subname]; ok2 {
			return status
		}
	}

	return ""
}

// Get all the subworer status for the given worker. It returns nil if the worker does not exist.
func (w *WorkerStatusManager) GetAllSubworkerStatus(name string) map[string]string {
	if ws, ok := w.Workers[name]; ok {
		return ws.SubworkerStatus
	}

	return nil
}
