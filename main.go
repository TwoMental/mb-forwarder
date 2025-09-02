package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var (
	configFile string = ""
)

func parseArgs() {
	flag.StringVar(&configFile, "config", configFile, "config file")
	flag.Parse()
}

func main() {
	parseArgs()

	// load config
	if err := loadConfig(configFile); err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	// create forwarder
	forwarder := NewForwarder(&C)

	// start forwarder
	if err := forwarder.Start(); err != nil {
		log.Fatalf("start forwarder failed: %v", err)
	}

	// wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Modbus forwarder started, press Ctrl+C to stop...")
	<-sigChan

	// graceful shutdown
	log.Println("stopping forwarder...")
	forwarder.Stop()
	log.Println("forwarder stopped")
}
