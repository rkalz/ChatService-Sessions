package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/go-redis/redis"
	"github.com/gocql/gocql"
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

type Session struct {
	UUID      string `json:"uuid"`
	Origin    string `json:"origin,omitempty"`
	SessionID string `json:"session,omitempty"`
}

// RandomString Generates a random string of [A-Za-z0-9] of length n
func RandomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)

	rand.Seed(int64(time.Now().Nanosecond()))
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

func SetHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
}

func CassConnect(keyspace string) *gocql.Session {
	acctCluster := gocql.NewCluster("host.docker.internal")
	acctCluster.Keyspace = keyspace
	acctCluster.Consistency = gocql.Three
	acctSess, _ := acctCluster.CreateSession()
	return acctSess
}

func RedisConnect(db int) (*redis.Client, error) {
	cache := redis.NewClient(&redis.Options{
		Addr:     "host.docker.internal:6379",
		Password: "",
		DB:       db,
	})
	_, err := cache.Ping().Result()
	return cache, err
}

func ResponseNoData(w http.ResponseWriter, code int) {
	resp := Response{}
	resp.Code = code
	response, _ := json.Marshal(resp)
	fmt.Fprint(w, string(response))
}
