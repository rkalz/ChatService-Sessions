package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gocql/gocql"
	"github.com/gorilla/mux"
)

// DefaultEndpoint ...
func DefaultEndpoint(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Default")
}

func GetSessionEndpoint(w http.ResponseWriter, r *http.Request) {
	uuid := mux.Vars(r)["uuid"]
	SetHeaders(w)
	resp := Response{}

	// Connect to Cassandra cluster and get session
	sess := CassConnect("sessions")
	defer sess.Close()

	// Connect to Redis server
	cache, err := RedisConnect(1)
	if err != nil {
		ResponseNoData(w, GetSessionError)
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
		ResponseNoData(w, GetSessionError)
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
	SetHeaders(w)

	// Connect to Cassandra cluster and get session
	sess := CassConnect("sessions")
	defer sess.Close()

	// Connect to Redis server
	cache, err := RedisConnect(1)
	if err != nil {
		ResponseNoData(w, GetSessionError)
		return
	}

	// Generate ID and add to query
	sessionid := RandomString(16)
	if err := sess.Query(`INSERT INTO sessions (sessionid, active, ts, userid) VALUES (?, true, ?, ?)`,
		sessionid, time.Now(), uuid).Exec(); err != nil {
		ResponseNoData(w, PostSessionError)
		log.Print("Query insert failed: ")
		log.Print(err)
		return
	}

	// Add pair to Redis
	err = cache.Set(sessionid, uuid, 0).Err()

	resp := Response{}
	resp.Code = PostSessionSuccess
	resp.SessionID = sessionid
	response, _ := json.Marshal(resp)
	fmt.Fprint(w, string(response))
}

func DeleteSessionEndpoint(w http.ResponseWriter, r *http.Request) {
	sess := mux.Vars(r)["sess"]
	SetHeaders(w)
	resp := Response{}

	// Connect to Cassandra cluster and get session
	db := CassConnect("sessions")
	defer db.Close()

	// Connect to Redis server
	cache, err := RedisConnect(1)
	if err != nil {
		ResponseNoData(w, GetSessionError)
		return
	}

	// Remove session from redis
	err = cache.Del(sess).Err()

	// Remove session from Cassandra
	if err := db.Query(`UPDATE sessions SET active=false WHERE sessionid = ?`,
		sess).Exec(); err != nil {
		ResponseNoData(w, DelSessionError)
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
