package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"git.dev.box.net/skynet/grumpy/pkg/webhook"
)

const (
	port = "8080"
)

var (
	tlscert, tlskey string
)

func main() {

	flag.StringVar(&tlscert, "tlsCertFile", "/etc/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&tlskey, "tlsKeyFile", "/etc/certs/key.pem", "File containing the x509 private key to --tlsCertFile.")

	flag.Parse()

	certs, err := tls.LoadX509KeyPair(tlscert, tlskey)
	if err != nil {
		fmt.Printf("Filed to load key pair: %v", err)
	}

	server := &http.Server{
		Addr:      fmt.Sprintf(":%v", port),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{certs}},
	}

	// define http server and server handler
	gs := webhook.GrumpyServerHandler{}
	mux := http.NewServeMux()
	mux.HandleFunc("/validate", gs.Serve)
	server.Handler = mux

	// start webhook server in new rountine
	go func() {
		if err := server.ListenAndServeTLS("", ""); err != nil {
			fmt.Printf("Failed to listen and serve webhook server: %v", err)
		}
	}()

	fmt.Printf("Server running listening in port: %s", port)

	// listening shutdown singal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	fmt.Printf("Got shutdown signal, shutting down webhook server gracefully...")
	server.Shutdown(context.Background())
}
