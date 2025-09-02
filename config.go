package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

var C Config

type Config struct {
	ListenPort int             `yaml:"listen_port"`
	Servers    map[byte]Server `yaml:"servers"` // SlaveID -> Server
	// LogLevel   string         `yaml:"log_level"`
}

type Server struct {
	ConnType string `yaml:"conn_type"` // "tcp" or "rtu"
	SlaveID  int    `yaml:"slave_id"`
	Addr     string `yaml:"addr"`      // TCP IP or RTU COMADDR
	Port     int    `yaml:"port"`      // TCP Port
	BaudRate int    `yaml:"baud_rate"` // RTU Baud Rate
	DataBits int    `yaml:"data_bits"` // RTU Data Bits
	StopBits int    `yaml:"stop_bits"` // RTU Stop Bits
	Parity   string `yaml:"parity"`    // RTU Parity
	Timeout  int    `yaml:"timeout"`   // Timeout(seconds)
}

func loadConfig(path string) error {
	if path == "" {
		return fmt.Errorf("config file path is required")
	}

	// read file
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// unmarshal yaml
	if err := yaml.Unmarshal(content, &C); err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}

	// validate config
	if err := validateConfig(); err != nil {
		return fmt.Errorf("config validation failed: %v", err)
	}

	return nil
}

func validateConfig() error {
	if C.ListenPort <= 0 {
		C.ListenPort = 1602 // Default port
	}

	if len(C.Servers) == 0 {
		return fmt.Errorf("no servers configured")
	}

	for slaveID, server := range C.Servers {
		if err := validateServer(slaveID, server); err != nil {
			return err
		}
	}

	return nil
}

func validateServer(slaveID byte, server Server) error {
	if slaveID < 1 || slaveID > 255 {
		return fmt.Errorf("invalid slave_id %d: must be between 1-255", slaveID)
	}

	if server.ConnType == "" {
		return fmt.Errorf("server %d: conn_type is required", slaveID)
	}

	if server.ConnType != "tcp" && server.ConnType != "rtu" {
		return fmt.Errorf("server %d: invalid conn_type %s, must be 'tcp' or 'rtu'", slaveID, server.ConnType)
	}

	if server.ConnType == "tcp" {
		if server.Addr == "" {
			return fmt.Errorf("server %d: addr is required for TCP connection", slaveID)
		}
		if server.Port <= 0 {
			server.Port = 502 // Default modbus port
		}
	} else if server.ConnType == "rtu" {
		if server.Addr == "" {
			return fmt.Errorf("server %d: addr is required for RTU connection", slaveID)
		}
		if server.BaudRate <= 0 {
			server.BaudRate = 9600 // Default baud rate
		}
		if server.DataBits <= 0 {
			server.DataBits = 8 // Default data bits
		}
		if server.StopBits <= 0 {
			server.StopBits = 1 // Default stop bits
		}
		if server.Parity == "" {
			server.Parity = "N" // Default parity
		}
	}

	if server.Timeout <= 0 {
		server.Timeout = 2 // Default timeout(seconds)
	}

	return nil
}
