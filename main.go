// THIS PACE OF CODE IS AN EXAMPLE ONLY
// THAT IS WHY IT IS KEPT ALL IN ONE FILE AND ONE PACKAGE

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"time"

	_ "net/http/pprof"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

// HookRequest contains subscriber address
type HookRequest struct {
	Address string `json:"address"`
}

// Payload contains raw payload send to subscriber
type Payload struct {
	Payload []byte `json:"payload"`
}

type dispatcher func(address string, payload []byte)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var isDebug bool

type debugServer struct {
	*http.Server
}

func newDebugServer(address string) *debugServer {
	return &debugServer{
		&http.Server{
			Addr:    address,
			Handler: http.DefaultServeMux,
		},
	}
}

func runDebugger() {
	debugServer := newDebugServer(fmt.Sprintf("%s:%d", "localhost", 6060))
	log.Printf("debugger started on %s", "localhost:6060")
	go func() {
		log.Fatal(debugServer.ListenAndServe())
	}()
}

func randStringBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return b
}

func runQueue(cq <-chan HookRequest, f dispatcher) {
	go func() {
		for v := range cq {
			// heavy computations
			time.Sleep(time.Second * 5)
			s := randStringBytes(100)
			p := Payload{s}
			jsonPayload, err := json.Marshal(&p)
			if err != nil {
				log.Printf("error while encoding request: %s", err)
			}
			f(v.Address, jsonPayload)
		}
	}()
}

func runResponder(address string, payload []byte) {
	go func() {
		client := &http.Client{}
		req, err := http.NewRequest("POST", address, bytes.NewBuffer(payload))
		if err != nil {
			log.Printf("error while encoding request: %s", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("error while sending request: %s", err)
		}
		defer resp.Body.Close()
	}()
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	info := struct {
		Status string `json:"status"`
	}{Status: "ok"}
	jsonInfo, err := json.Marshal(info)
	if err != nil {
		log.Printf("error parsing json %S\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonInfo)
	return
}

func loggerMiddlewere(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("request from IP: %s", r.RemoteAddr)
		h.ServeHTTP(w, r)
	})
}

func webhookHandler(w http.ResponseWriter, r *http.Request, cq chan<- HookRequest) {
	var hookData HookRequest
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(&hookData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	cq <- hookData
	w.WriteHeader(http.StatusAccepted)
	return
}

func subscriberHandler(w http.ResponseWriter, r *http.Request) {
	var hookData Payload
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(&hookData)
	if err != nil {
		log.Printf("error subscribe handler decoding message %S\n", err)
	}
	log.Printf("<<< SUBSCRIBER >>> message decoded to : %v\n", hookData)
	w.WriteHeader(http.StatusOK)
	return
}

// ServerRun executes server
func serverRun(addr string, cs <-chan os.Signal, cq chan<- HookRequest) {

	r := mux.NewRouter()
	r.Use(loggerMiddlewere)
	r.HandleFunc("/", rootHandler).Methods("GET")
	r.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) { webhookHandler(w, r, cq) }).Methods("POST")
	r.HandleFunc("/subscriber", subscriberHandler).Methods("POST")

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
	})

	srv := &http.Server{
		Addr:         addr,
		WriteTimeout: time.Second * 30,
		ReadTimeout:  time.Second * 30,
		IdleTimeout:  time.Second * 60,
		Handler:      c.Handler(r),
	}

	// Run server in a go routine so that it doesn't block
	go func() {
		log.Printf("starting server on %s\n", addr)
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	<-cs

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("server shut down")

	time.Sleep(time.Millisecond * 2)

	close(cq)

	os.Exit(0)
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("cannot load .env file %s\n", err)
	}
	isDebug = os.Getenv("DEBUG") == "true"
}

func main() {
	if isDebug {
		runtime.SetBlockProfileRate(1)
		runDebugger()
	}

	cs := make(chan os.Signal, 1)
	signal.Notify(cs, os.Interrupt)

	cq := make(chan HookRequest, 100) //TODO: here we can make map of hook chan to dispatcher function

	runQueue(cq, runResponder)
	serverRun("127.0.0.1:8080", cs, cq)
}
