package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis"
	"github.com/gocql/gocql"
	"github.com/gorilla/mux"
)

// Result Code of operation
const (
	GetSessionSuccess   = 100
	GetSessionMultiple  = 101
	GetSessionNoneFound = 102
	GetSessionTooOld    = 103
	GetSessionError     = 104
	PostSessionSuccess  = 200
	PostSessionError    = 201
	DelSessionSuccess   = 300
	DelSessionError     = 301
)

type Response struct {
	Code      int    `json:"code"`
	SessionID string `json:"session,omitempty"`
}

// RandomString Generates a random string of [A-Za-z0-9] of length n
func RandomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

// DefaultEndpoint ...
func DefaultEndpoint(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Default")
}

func GetSessionEndpoint(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	resp := Response{}

	// Connect to Cassandra cluster and get session
	cluster := gocql.NewCluster("127.0.0.1", "127.0.0.2", "127.0.0.3")
	cluster.Keyspace = "sessions"
	cluster.Consistency = gocql.Three
	sess, _ := cluster.CreateSession()
	defer sess.Close()

	// Connect to Redis server
	cache := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       1,
	})
	_, err := cache.Ping().Result()
	if err != nil {
		resp.Code = GetSessionError
		response, _ := json.Marshal(resp)
		fmt.Fprint(w, string(response))
		return
	}

	// Check Redis server
	val, err := cache.Get(uuid).Result()
	if val != "" {
		resp.SessionID = val
		resp.Code = GetSessionSuccess
		response, _ := json.Marshal(resp)
		fmt.Fprint(w, string(response))
		return
	}

	if err := sess.Query(`SELECT sessionid FROM sessions WHERE userid = ? AND active=True ALLOW FILTERING`,
		uuid).Consistency(gocql.One).Scan(&resp.SessionID); err != nil || resp.SessionID == "" {
		resp.Code = GetSessionError
		response, _ := json.Marshal(resp)
		fmt.Fprint(w, string(response))
		return
	}

	// Add to Redis server
	err = cache.Set(resp.SessionID, uuid, 0).Err()

	resp.Code = GetSessionSuccess
	response, _ := json.Marshal(resp)
	fmt.Fprint(w, string(response))
}

func NewSessionEndpoint(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	resp := Response{}

	// Connect to Cassandra cluster and get session
	cluster := gocql.NewCluster("127.0.0.1", "127.0.0.2", "127.0.0.3")
	cluster.Keyspace = "sessions"
	cluster.Consistency = gocql.Three
	sess, _ := cluster.CreateSession()
	defer sess.Close()

	// Connect to Redis server
	cache := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       1,
	})
	_, err := cache.Ping().Result()
	if err == nil {
		fmt.Println("connected to Redis")
	}

	// Generate ID and add to query
	sessionid := RandomString(16)
	if err := sess.Query(`INSERT INTO sessions (sessionid, active, ts, userid) VALUES (?, true, ?, ?)`,
		sessionid, time.Now(), uuid).Exec(); err != nil {
		resp.Code = PostSessionError
		response, _ := json.Marshal(resp)
		fmt.Fprint(w, string(response))
		log.Print("Query insert failed: ")
		log.Print(err)
		return
	}

	// Add pair to Redis
	err = cache.Set(sessionid, uuid, 0).Err()

	resp.Code = PostSessionSuccess
	resp.SessionID = sessionid
	response, _ := json.Marshal(resp)
	fmt.Fprint(w, string(response))
}

func DeleteSessionEndpoint(w http.ResponseWriter, r *http.Request) {
	sess := mux.Vars(r)["sess"]
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	resp := Response{}

	// Connect to Cassandra cluster and get session
	cluster := gocql.NewCluster("127.0.0.1")
	cluster.Keyspace = "sessions"
	cluster.Consistency = gocql.Three
	db, _ := cluster.CreateSession()
	defer db.Close()

	// Connect to Redis server
	cache := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       1,
	})
	_, err := cache.Ping().Result()
	if err == nil {
		fmt.Println("connected to Redis")
	}

	// Remove session from redis
	err = cache.Del(sess).Err()

	// Remove session from Cassandra
	if err := db.Query(`UPDATE sessions SET active=false WHERE sessionid = ?`,
		sess).Exec(); err != nil {
		resp.Code = DelSessionError
		response, _ := json.Marshal(resp)
		fmt.Fprint(w, string(response))
		log.Print("Query insert failed: ")
		log.Print(err)
		return
	}

	resp.Code = DelSessionSuccess
	response, _ := json.Marshal(resp)
	fmt.Fprint(w, string(response))
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", DefaultEndpoint)
	r.HandleFunc("/api/v1/private/sessions/get/{uuid}", GetSessionEndpoint)
	r.HandleFunc("/api/v1/private/sessions/add/{uuid}", NewSessionEndpoint).Methods("POST")
	r.HandleFunc("/api/v1/private/sessions/del/{sess}", DeleteSessionEndpoint).Methods("POST")

	if os.Getenv("PORT") == "" {
		os.Setenv("PORT", "8081")
	}

	if err := http.ListenAndServe(":"+os.Getenv("PORT"), r); err != nil {
		log.Fatal(err)
	}
}
