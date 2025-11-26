package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

type Task struct {
	ID       int      `json:"id"`
	TaskList []string `json:"tasklist"`
	Date     string   `json:"tanggal"`
	Notes    string   `json:"notes"`
}

var db *sql.DB

func init() {
	var err error
	// Ambil DATABASE_URL dari environment variable Koyeb
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Println("❌ ERROR: DATABASE_URL environment variable not set!")
		panic("DATABASE_URL is required")
	}

	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		panic(err)
	}

	// Test connection
	err = db.Ping()
	if err != nil {
		panic(err)
	}

	fmt.Println("✓ Connected to PostgreSQL successfully!")

	// Create table jika belum ada
	createTable()
}

func createTable() {
	query := `
	CREATE TABLE IF NOT EXISTS tasks (
		id SERIAL PRIMARY KEY,
		tasklist TEXT[] NOT NULL,
		tanggal DATE NOT NULL,
		notes TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(query)
	if err != nil {
		panic(err)
	}
}

// GET all tasks
func GetTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	rows, err := db.Query("SELECT id, tasklist, tanggal, notes FROM tasks ORDER BY id DESC")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var task Task
		err := rows.Scan(&task.ID, pq.Array(&task.TaskList), &task.Date, &task.Notes)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		tasks = append(tasks, task)
	}

	if tasks == nil {
		tasks = []Task{}
	}
	json.NewEncoder(w).Encode(tasks)
}

// GET single task by ID
func GetTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid ID"})
		return
	}

	var task Task
	query := "SELECT id, tasklist, tanggal, notes FROM tasks WHERE id = $1"
	err = db.QueryRow(query, id).Scan(&task.ID, pq.Array(&task.TaskList), &task.Date, &task.Notes)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Task not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(task)
}

// CREATE new task
func CreateTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var task Task
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid input"})
		return
	}

	// Validasi input
	if len(task.TaskList) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Tasklist harus diisi"})
		return
	}

	if task.Date == "" {
		task.Date = time.Now().Format("2006-01-02")
	}

	query := "INSERT INTO tasks (tasklist, tanggal, notes) VALUES ($1, $2, $3) RETURNING id"
	err = db.QueryRow(query, pq.Array(task.TaskList), task.Date, task.Notes).Scan(&task.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// UPDATE task
func UpdateTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid ID"})
		return
	}

	var updatedTask Task
	err = json.NewDecoder(r.Body).Decode(&updatedTask)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid input"})
		return
	}

	// Get current task
	var task Task
	query := "SELECT id, tasklist, tanggal, notes FROM tasks WHERE id = $1"
	err = db.QueryRow(query, id).Scan(&task.ID, pq.Array(&task.TaskList), &task.Date, &task.Notes)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Task not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Update fields
	if len(updatedTask.TaskList) > 0 {
		task.TaskList = updatedTask.TaskList
	}
	if updatedTask.Date != "" {
		task.Date = updatedTask.Date
	}
	if updatedTask.Notes != "" {
		task.Notes = updatedTask.Notes
	}

	updateQuery := "UPDATE tasks SET tasklist = $1, tanggal = $2, notes = $3 WHERE id = $4"
	_, err = db.Exec(updateQuery, pq.Array(task.TaskList), task.Date, task.Notes, id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}

// DELETE task
func DeleteTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid ID"})
		return
	}

	result, err := db.Exec("DELETE FROM tasks WHERE id = $1", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if rowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Task not found"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Task deleted"})
}

func main() {
	router := mux.NewRouter()

	// Routes
	router.HandleFunc("/api/tasks", GetTasks).Methods("GET")
	router.HandleFunc("/api/tasks/{id}", GetTask).Methods("GET")
	router.HandleFunc("/api/tasks", CreateTask).Methods("POST")
	router.HandleFunc("/api/tasks/{id}", UpdateTask).Methods("PUT")
	router.HandleFunc("/api/tasks/{id}", DeleteTask).Methods("DELETE")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server running on port %s\n", port)
	http.ListenAndServe(":"+port, router)
}
