package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ResponseNoData(w, GetSessionError)
		log.Print("Body read failed")
		log.Print(err)
		return
	}

	request := Session{}
	err = json.Unmarshal(body, &request)
	if err != nil {
		ResponseNoData(w, GetSessionError)
		log.Print("Body Unmarshal failed")
		log.Print(err)
		return
	}

	SetHeaders(w)
	resp := Response{}

	// Connect to Cassandra cluster
	sess, err := ConnectToCassandra("sessions")
	if err != nil {
		ResponseNoData(w, GetSessionError)
		log.Print("Cassandra connection failed")
		log.Print(err)
		return
	}
	defer sess.Close()

	// Connect to Redis server
	cache, err := RedisConnect(1)
	if err != nil {
		ResponseNoData(w, GetSessionError)
		log.Print("Redis connection failed")
		log.Print(err)
		return
	}

	// Check Redis server
	val, err := cache.Get(request.UUID).Result()
	if val != "" {
		session := Session{}
		_ = json.Unmarshal([]byte(val), &session)
		resp.SessionID = session.SessionID
		resp.Code = GetSessionSuccess
		response, _ := json.Marshal(resp)
		fmt.Fprint(w, string(response))
		return
	}

	if err := sess.Query(`SELECT sessionid FROM sessions WHERE userid = ? AND origin = ? AND active=True ALLOW FILTERING`,
		request.UUID, request.Origin).Consistency(gocql.One).Scan(&resp.SessionID); err != nil || resp.SessionID == "" {
		ResponseNoData(w, GetSessionError)
		log.Print("Cassandra query failed")
		log.Print(err)
		return
	}

	// Add to Redis server
	storedData := Session{
		UUID:      request.UUID,
		Origin:    request.Origin,
		SessionID: resp.SessionID,
	}
	storedDataBytes, err := json.Marshal(storedData)
	err = cache.Set(resp.SessionID, storedDataBytes, 0).Err()
	if err != nil {
		log.Print("Redis caching failed")
		log.Print(err)
	}

	resp.Code = GetSessionSuccess
	response, _ := json.Marshal(resp)
	fmt.Fprint(w, string(response))
}

func NewSessionEndpoint(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	request := Session{}
	err = json.Unmarshal(body, &request)
	SetHeaders(w)

	// Connect to Cassandra cluster
	sess, err := ConnectToCassandra("sessions")
	if err != nil {
		ResponseNoData(w, GetSessionError)
		log.Print("Cassandra connection failed")
		log.Print(err)
		return
	}
	defer sess.Close()

	// Connect to Redis server
	cache, err := RedisConnect(1)
	if err != nil {
		ResponseNoData(w, GetSessionError)
		log.Print("Redis connection failed")
		log.Print(err)
		return
	}

	// Generate ID and add to query
	sessionid := RandomString(16)
	if err := sess.Query(`INSERT INTO sessions (sessionid, active, ts, userid, origin) VALUES (?, true, ?, ?, ?)`,
		sessionid, time.Now(), request.UUID, request.Origin).Exec(); err != nil {
		ResponseNoData(w, PostSessionError)
		log.Print("Query insert failed: ")
		log.Print(err)
		return
	}

	// Add data to Redis
	storedData := Session{
		UUID:      request.UUID,
		Origin:    request.Origin,
		SessionID: sessionid,
	}
	storedDataBytes, err := json.Marshal(storedData)
	err = cache.Set(sessionid, storedDataBytes, 0).Err()
	if err != nil {
		log.Print("Redis caching failed")
		log.Print(err)
	}

	resp := Response{}
	resp.Code = PostSessionSuccess
	resp.SessionID = sessionid
	response, _ := json.Marshal(resp)
	fmt.Fprint(w, string(response))
}

func DeleteSessionEndpoint(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ResponseNoData(w, GetSessionError)
		log.Print("Body read failed")
		log.Print(err)
		return
	}

	request := Session{}
	err = json.Unmarshal(body, &request)
	if err != nil {
		ResponseNoData(w, GetSessionError)
		log.Print("Body Unmarshal failed")
		log.Print(err)
		return
	}

	SetHeaders(w)
	resp := Response{}

	// Connect to Cassandra cluster
	db, err := ConnectToCassandra("sessions")
	defer db.Close()

	// Connect to Redis server
	cache, err := RedisConnect(1)
	if err != nil {
		ResponseNoData(w, GetSessionError)
		log.Print(err)
		return
	}

	// Remove session from redis
	err = cache.Del(request.SessionID).Err()

	// Remove session from Cassandra
	if err := db.Query(`UPDATE sessions SET active=false WHERE userid = ? AND origin = ?`,
		request.UUID, request.Origin).Exec(); err != nil {
		ResponseNoData(w, DelSessionError)
		log.Print("Query update failed: ")
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
	r.HandleFunc("/api/v1/private/sessions/check/", GetSessionEndpoint).Methods("POST")
	r.HandleFunc("/api/v1/private/sessions/add/", NewSessionEndpoint).Methods("POST")
	r.HandleFunc("/api/v1/private/sessions/del/", DeleteSessionEndpoint).Methods("POST")

	if os.Getenv("PORT") == "" {
		os.Setenv("PORT", "80")
	}

	if err := http.ListenAndServe(":"+os.Getenv("PORT"), r); err != nil {
		log.Print("Server start failed")
		log.Fatal(err)
	}
}
