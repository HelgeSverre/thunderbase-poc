package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
)

const dbPath = "./thunderbase.db"
const changesTable = "_changes"

type ChangeEvent struct {
	Collection string      `json:"collection"`
	Event      string      `json:"event"`
	Data       interface{} `json:"data"`
}

type ThunderBase struct {
	db           *sql.DB
	clients      map[*websocket.Conn]bool
	broadcast    chan ChangeEvent
	upgrader     websocket.Upgrader
	clientsMutex sync.Mutex
}

func NewThunderBase(dbPath string) (*ThunderBase, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	tb := &ThunderBase{
		db:        db,
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan ChangeEvent),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	if err := tb.initDatabase(); err != nil {
		return nil, err
	}

	return tb, nil
}

func (tb *ThunderBase) initDatabase() error {

	log.Println("Initializing database...")

	// Create the changes table
	_, err := tb.db.Exec(fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            collection TEXT NOT NULL,
            event TEXT NOT NULL,
            data JSON NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    `, changesTable))
	if err != nil {
		return fmt.Errorf("failed to create %s table: %v", changesTable, err)
	}

	log.Println("Created changes table: ", changesTable)

	// Create a trigger function for each table
	tables, err := tb.getTables()
	if err != nil {
		return fmt.Errorf("failed to get tables: %v", err)
	}

	log.Printf("Creating triggers for tables: %v", tables)

	for _, table := range tables {
		// Get column names for the table
		columns, err := tb.getTableColumns(table)
		if err != nil {
			return fmt.Errorf("failed to get columns for table %s: %v", table, err)
		}

		for _, event := range []string{"INSERT", "UPDATE", "DELETE"} {
			triggerSQL := fmt.Sprintf(`
                CREATE TRIGGER IF NOT EXISTS %s_%s_trigger
                AFTER %s ON %s
                BEGIN
                    INSERT INTO %s (collection, event, data)
                    VALUES ('%s', '%s', json_object(%s));
                END;
            `, table, event, event, table, changesTable, table, event, tb.buildJsonObject(columns, event))

			_, err := tb.db.Exec(triggerSQL)
			if err != nil {
				return fmt.Errorf("failed to create trigger for %s %s: %v", table, event, err)
			}
		}
	}

	return nil
}

func (tb *ThunderBase) getTableColumns(table string) ([]string, error) {
	rows, err := tb.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		//goland:noinspection ALL
		var (
			cid, notnull, pk int
			name, type_      string
			dflt_value       sql.NullString
		)

		if err := rows.Scan(&cid, &name, &type_, &notnull, &dflt_value, &pk); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, nil
}

func (tb *ThunderBase) buildJsonObject(columns []string, event string) string {
	var pairs []string
	for _, col := range columns {
		var value string
		if event == "DELETE" {
			value = fmt.Sprintf("OLD.%s", col)
		} else {
			value = fmt.Sprintf("NEW.%s", col)
		}
		pairs = append(pairs, fmt.Sprintf("'%s', %s", col, value))
	}
	return strings.Join(pairs, ", ")
}

func (tb *ThunderBase) getTables() ([]string, error) {
	rows, err := tb.db.Query(`
        SELECT name FROM sqlite_master 
        WHERE type='table' 
        AND name NOT LIKE 'sqlite_%' 
        AND name != ?`, changesTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}

	return tables, nil
}

func (tb *ThunderBase) pollChanges() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		rows, err := tb.db.Query(fmt.Sprintf("SELECT id, collection, event, data FROM %s ORDER BY id ASC LIMIT 100", changesTable))
		if err != nil {
			log.Printf("Error querying changes: %v", err)
			continue
		}

		var processedIDs []int64
		for rows.Next() {
			var id int64
			var change ChangeEvent
			if err := rows.Scan(&id, &change.Collection, &change.Event, &change.Data); err != nil {
				log.Printf("Error scanning change: %v", err)
				continue
			}

			tb.broadcast <- change
			processedIDs = append(processedIDs, id)
		}
		rows.Close()

		if len(processedIDs) > 0 {
			_, err := tb.db.Exec(
				fmt.Sprintf("DELETE FROM %s WHERE id IN ("+placeholders(len(processedIDs))+")", changesTable),
				intsToInterface(processedIDs)...,
			)
			if err != nil {
				log.Printf("Error deleting processed changes: %v", err)
			}
		}
	}
}

func (tb *ThunderBase) handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := tb.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("New connection from %s", r.RemoteAddr)

	err = conn.WriteJSON(map[string]string{"message": "Welcome to ThunderBase!"})
	if err != nil {
		log.Printf("Error sending welcome message: %v", err)
		return
	}

	tb.clientsMutex.Lock()
	tb.clients[conn] = true
	tb.clientsMutex.Unlock()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			tb.clientsMutex.Lock()
			delete(tb.clients, conn)
			tb.clientsMutex.Unlock()
			break
		}

		log.Printf("Received message from %s: %s", r.RemoteAddr, string(msg))
	}
}

func (tb *ThunderBase) broadcastChanges() {
	for event := range tb.broadcast {

		log.Printf("Broadcasting change: %v", event)

		tb.clientsMutex.Lock()
		for client := range tb.clients {
			err := client.WriteJSON(event)
			if err != nil {
				log.Printf("Error broadcasting to client: %v", err)
				client.Close()
				delete(tb.clients, client)
			}
		}
		tb.clientsMutex.Unlock()
	}
}

func (tb *ThunderBase) createTable(name string) error {
	_, err := tb.db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY, name TEXT)", name))
	if err != nil {
		return err
	}
	log.Printf("> Created table: %s", name)

	return tb.initDatabase()
}

func (tb *ThunderBase) insertDummyData(tableName string) error {
	_, err := tb.db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('John Doe')", tableName))
	return err
}

func main() {
	tb, err := NewThunderBase(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize ThunderBase: %v", err)
	}
	defer tb.db.Close()

	// Create a test table
	err = tb.createTable("users")
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	go tb.pollChanges()
	go tb.broadcastChanges()

	http.HandleFunc("/ws", tb.handleConnections)

	http.HandleFunc("/insert", func(w http.ResponseWriter, r *http.Request) {
		err := tb.insertDummyData("users")
		if err != nil {
			log.Printf("Error inserting dummy data: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte("Inserted dummy data"))
	})

	// Serve a simple HTML page for testing
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "gui.html")
	})

	log.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return "?" + strings.Repeat(",?", n-1)
}

func intsToInterface(ints []int64) []interface{} {
	result := make([]interface{}, len(ints))
	for i, v := range ints {
		result[i] = v
	}
	return result
}
