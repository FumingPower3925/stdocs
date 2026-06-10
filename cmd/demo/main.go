// Package main is a demo of stdocs using a fictional Task Tracker API.
//
// It demonstrates both tiers:
//
//   - Tier 1: a route registered with no documentation opts gets a
//     summary inferred from the function name and a tag from the first
//     path segment.
//   - Tier 2: routes that pass stdocs.WithResponse / stdocs.WithBody
//     produce schemas from Go types in the OpenAPI spec.
//
// Run with: go run ./cmd/demo
// Then visit http://localhost:8080/docs/ for the docs UI.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/ui/scalar"
)

// Task is a unit of work in the tracker. It exercises time.Time and a
// recursive *Task pointer (parent-child relationships).
type Task struct {
	ID        string    `json:"id" doc:"Unique task identifier"`
	Title     string    `json:"title" doc:"Human-readable title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
	Tags      []string  `json:"tags,omitempty"`
	Parent    *Task     `json:"parent,omitempty"`
}

// CreateTaskRequest is the body for POST /tasks.
type CreateTaskRequest struct {
	Title string   `json:"title" doc:"Title of the new task"`
	Tags  []string `json:"tags,omitempty"`
}

// UpdateTaskRequest is the body for PATCH /tasks/{id}. All fields are
// optional pointers; nil means "don't change".
type UpdateTaskRequest struct {
	Title *string `json:"title,omitempty"`
	Done  *bool   `json:"done,omitempty"`
}

// NotFoundError is returned for 404 responses.
type NotFoundError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// store is an in-memory task store. It's just for the demo; real code
// would use a database.
type store struct {
	mu    sync.Mutex
	tasks map[string]*Task
}

func newStore() *store {
	return &store{tasks: make(map[string]*Task)}
}

func (s *store) list() []*Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, t)
	}
	return out
}

func (s *store) get(id string) (*Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	return t, ok
}

func (s *store) put(t *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[t.ID] = t
}

func (s *store) delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.tasks[id]
	if ok {
		delete(s.tasks, id)
	}
	return ok
}

var s = newStore()
var nextID = 0

func nextTaskID() string {
	nextID++
	return fmt.Sprintf("t-%d", nextID)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, NotFoundError{Code: status, Message: msg})
}

func listTasks(w http.ResponseWriter, r *http.Request) {
	tasks := s.list()
	writeJSON(w, 200, tasks)
}

func getTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, ok := s.get(id)
	if !ok {
		writeErr(w, 404, "task not found")
		return
	}
	writeJSON(w, 200, t)
}

func createTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid body: "+err.Error())
		return
	}
	if req.Title == "" {
		writeErr(w, 400, "title is required")
		return
	}
	t := &Task{
		ID:        nextTaskID(),
		Title:     req.Title,
		Done:      false,
		CreatedAt: time.Now(),
		Tags:      req.Tags,
	}
	s.put(t)
	writeJSON(w, 201, t)
}

func updateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, ok := s.get(id)
	if !ok {
		writeErr(w, 404, "task not found")
		return
	}
	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid body: "+err.Error())
		return
	}
	if req.Title != nil {
		t.Title = *req.Title
	}
	if req.Done != nil {
		t.Done = *req.Done
	}
	s.put(t)
	writeJSON(w, 200, t)
}

func deleteTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.delete(id) {
		writeErr(w, 404, "task not found")
		return
	}
	w.WriteHeader(204)
}

func health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func main() {
	mux := stdocs.New(
		stdocs.WithTitle("Task Tracker API"),
		stdocs.WithAPIVersion("1.0.0"),
		stdocs.WithDescription("A tiny task tracker used to demonstrate stdocs."),
		stdocs.WithServer("http://localhost:8080", "local"),
		stdocs.WithContact("FumingPower3925", "fuming@example.com", ""),
		stdocs.WithTag("tasks", "Operations on tasks"),
		stdocs.WithTag("health", "Service health"),
		// Use the Scalar UI (loaded from CDN).
		scalar.WithUI(),
	)

	// Tier 2 examples: rich metadata. Each call uses per-route opts
	// to document the response body and request body types.
	mux.HandleFunc("GET /tasks", listTasks,
		stdocs.Summary("List all tasks"),
		stdocs.Tags("tasks"),
		stdocs.WithResponse(200, []*Task{}),
	)
	mux.HandleFunc("GET /tasks/{id}", getTask,
		stdocs.Summary("Get a single task by ID"),
		stdocs.Tags("tasks"),
		stdocs.WithResponse(200, Task{}),
		stdocs.WithResponse(404, NotFoundError{}),
	)
	mux.HandleFunc("POST /tasks", createTask,
		stdocs.Summary("Create a new task"),
		stdocs.Tags("tasks"),
		stdocs.WithBody(CreateTaskRequest{}),
		stdocs.WithResponse(201, Task{}),
		stdocs.WithResponse(400, NotFoundError{}),
	)
	mux.HandleFunc("PATCH /tasks/{id}", updateTask,
		stdocs.Summary("Update a task"),
		stdocs.Description("All fields are optional. Only non-nil fields are applied."),
		stdocs.Tags("tasks"),
		stdocs.WithBody(UpdateTaskRequest{}),
		stdocs.WithResponse(200, Task{}),
		stdocs.WithResponse(404, NotFoundError{}),
	)
	mux.HandleFunc("DELETE /tasks/{id}", deleteTask,
		stdocs.Summary("Delete a task"),
		stdocs.Tags("tasks"),
		stdocs.WithResponse(204, nil),
		stdocs.WithResponse(404, NotFoundError{}),
	)

	// Tier 1 example: no opts at all. The function name and first
	// path segment are used as defaults.
	mux.HandleFunc("GET /health", health)

	// Mount the docs handler on the mux itself.
	mux.Mount()

	// Seed some demo data so the responses aren't empty.
	seedDemoData()

	addr := ":8080"
	fmt.Printf("Task Tracker API listening on %s\nDocs at http://localhost%s/docs/\n", addr, addr)
	if err := http.ListenAndServe(addr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func seedDemoData() {
	t1 := &Task{ID: nextTaskID(), Title: "Write the README", Done: false, CreatedAt: time.Now(), Tags: []string{"docs"}}
	t2 := &Task{ID: nextTaskID(), Title: "Ship v0", Done: true, CreatedAt: time.Now().Add(-24 * time.Hour), Tags: []string{"release"}}
	t3 := &Task{ID: nextTaskID(), Title: "Fix bug #42", Done: false, CreatedAt: time.Now().Add(-2 * time.Hour), Parent: t1}
	s.put(t1)
	s.put(t2)
	s.put(t3)
}
