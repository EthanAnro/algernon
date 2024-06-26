package pquery

import (
	"database/sql"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/xyproto/algernon/lua/convert"
	lua "github.com/xyproto/gopher-lua"

	// Using the PostgreSQL database engine
	_ "github.com/lib/pq"
)

const (
	defaultQuery            = "SELECT version()"
	defaultConnectionString = "host=localhost port=5432 user=postgres dbname=test sslmode=disable"
)

var (
	// global map from connection string to database connection, to reuse connections, protected by a mutex
	reuseDB  = make(map[string]*sql.DB)
	reuseMut = &sync.RWMutex{}
)

// Load makes functions related to building a library of Lua code available
func Load(L *lua.LState) {

	// Register the PQ (Postgres Query) function
	L.SetGlobal("PQ", L.NewFunction(func(L *lua.LState) int {
		// Check if the optional argument is given
		query := defaultQuery
		if L.GetTop() >= 1 {
			query = L.ToString(1)
			if query == "" {
				query = defaultQuery
			}
		}
		connectionString := defaultConnectionString
		if L.GetTop() >= 2 {
			connectionString = L.ToString(2)
		}

		// Check if there is a connection that can be reused
		var db *sql.DB
		reuseMut.RLock()
		conn, ok := reuseDB[connectionString]
		reuseMut.RUnlock()

		if ok {
			// It exists, but is it still alive?
			err := conn.Ping()
			if err != nil {
				// no
				reuseMut.Lock()
				delete(reuseDB, connectionString)
				reuseMut.Unlock()
			} else {
				// yes
				db = conn
			}
		}
		// Create a new connection, if needed
		var err error
		if db == nil {
			db, err = sql.Open("postgres", connectionString)
			if err != nil {
				logrus.Error("Could not connect to database using " + connectionString + ": " + err.Error())
				return 0 // No results
			}
			// Save the connection for later
			reuseMut.Lock()
			reuseDB[connectionString] = db
			reuseMut.Unlock()
		}
		// logrus.Info(fmt.Sprintf("PostgreSQL database: %v (%T)\n", db, db))
		reuseMut.Lock()
		rows, err := db.Query(query)
		reuseMut.Unlock()
		if err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, ": connect: connection refused") {
				logrus.Info("PostgreSQL connection string: " + connectionString)
				logrus.Info("PostgreSQL query: " + query)
				logrus.Error("Could not connect to database: " + errMsg)
			} else if strings.Contains(errMsg, "missing") && strings.Contains(errMsg, "in connection info string") {
				logrus.Info("PostgreSQL connection string: " + connectionString)
				logrus.Info("PostgreSQL query: " + query)
				logrus.Error(errMsg)
			} else {
				logrus.Info("PostgreSQL query: " + query)
				logrus.Error("Query failed: " + errMsg)
			}
			return 0 // No results
		}
		if rows == nil {
			// Return an empty table
			L.Push(L.NewTable())
			return 1 // number of results
		}
		// Return the rows as a table
		var (
			values []string
			value  string
		)
		for rows.Next() {
			err = rows.Scan(&value)
			if err != nil {
				break
			}
			values = append(values, value)
		}
		// Convert the strings to a Lua table
		table := convert.Strings2table(L, values)
		// Return the table
		L.Push(table)
		return 1 // number of results
	}))
}
