package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/goburrow/modbus"
	"github.com/tbrandon/mbserver"
)

// Forwarder modbus forwarder
type Forwarder struct {
	config     *Config
	server     *mbserver.Server
	clients    map[byte]*modbusClient // slaveID -> client
	clientsMux sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// modbusClient modbus client connection
type modbusClient struct {
	client    modbus.Client
	handler   modbus.ClientHandler
	connType  string
	addr      string
	port      int
	baudRate  int
	dataBits  int
	stopBits  int
	parity    string
	timeout   time.Duration
	lastError error
	lastConn  time.Time
}

// NewForwarder create new forwarder
func NewForwarder(config *Config) *Forwarder {
	ctx, cancel := context.WithCancel(context.Background())
	return &Forwarder{
		config:  config,
		clients: make(map[byte]*modbusClient),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start start forwarder
func (s *Forwarder) Start() error {
	// create modbus server
	s.server = mbserver.NewServer()

	// register function code handlers
	s.registerHandlers()

	// initialize client connections
	if err := s.initClients(); err != nil {
		return fmt.Errorf("failed to init clients: %v", err)
	}

	// start listening
	listenAddr := fmt.Sprintf("0.0.0.0:%d", s.config.ListenPort)
	log.Printf("modbus forwarder listening on %s", listenAddr)

	if err := s.server.ListenTCP(listenAddr); err != nil {
		return fmt.Errorf("failed to listen on %s: %v", listenAddr, err)
	}

	// start connection monitoring
	go s.monitorConnections()

	log.Printf("modbus forwarder started with %d servers", len(s.config.Servers))
	return nil
}

// Stop stop forwarder
func (s *Forwarder) Stop() {
	s.cancel()
	if s.server != nil {
		s.server.Close()
	}

	s.clientsMux.Lock()
	defer s.clientsMux.Unlock()

	for _, client := range s.clients {
		if client.handler != nil {
			// for TCP and RTU connections, close underlying connection
			if tcpHandler, ok := client.handler.(*modbus.TCPClientHandler); ok {
				tcpHandler.Close()
			} else if rtuHandler, ok := client.handler.(*modbus.RTUClientHandler); ok {
				rtuHandler.Close()
			}
		}
	}

	log.Println("modbus forwarder stopped")
}

// registerHandlers register function code handlers
func (s *Forwarder) registerHandlers() {
	// read coils (function code 1)
	s.server.RegisterFunctionHandler(1, s.readCoils)
	// read discrete inputs (function code 2)
	s.server.RegisterFunctionHandler(2, s.readDiscreteInputs)
	// read holding registers (function code 3)
	s.server.RegisterFunctionHandler(3, s.readHoldingRegisters)
	// read input registers (function code 4)
	s.server.RegisterFunctionHandler(4, s.readInputRegisters)
	// write single coil (function code 5)
	s.server.RegisterFunctionHandler(5, s.writeSingleCoil)
	// write single register (function code 6)
	s.server.RegisterFunctionHandler(6, s.writeSingleRegister)
	// write multiple coils (function code 15)
	s.server.RegisterFunctionHandler(15, s.writeMultipleCoils)
	// write multiple registers (function code 16)
	s.server.RegisterFunctionHandler(16, s.writeMultipleRegisters)
}

// initClients initialize client connections
func (s *Forwarder) initClients() error {
	for slaveID, serverConfig := range s.config.Servers {
		client, err := s.createClient(slaveID, serverConfig)
		if err != nil {
			return fmt.Errorf("failed to create client for slave %d: %v", slaveID, err)
		}

		s.clientsMux.Lock()
		s.clients[slaveID] = client
		s.clientsMux.Unlock()

		log.Printf("initialized slave %d connection (%s)", slaveID, serverConfig.ConnType)
	}
	return nil
}

// createClient create modbus client
func (s *Forwarder) createClient(slaveID byte, config Server) (*modbusClient, error) {
	var handler modbus.ClientHandler

	timeout := time.Duration(config.Timeout) * time.Second

	switch config.ConnType {
	case "tcp", "TCP":
		addr := fmt.Sprintf("%s:%d", config.Addr, config.Port)
		handler = modbus.NewTCPClientHandler(addr)
		if tcpHandler, ok := handler.(*modbus.TCPClientHandler); ok {
			tcpHandler.Timeout = timeout
			tcpHandler.SlaveId = byte(slaveID)
		}
	case "rtu", "RTU":
		handler = modbus.NewRTUClientHandler(config.Addr)
		if rtuHandler, ok := handler.(*modbus.RTUClientHandler); ok {
			rtuHandler.BaudRate = config.BaudRate
			rtuHandler.DataBits = config.DataBits
			rtuHandler.StopBits = config.StopBits
			rtuHandler.Parity = config.Parity
			rtuHandler.Timeout = timeout
			rtuHandler.SlaveId = byte(slaveID)
		}
	}

	if handler == nil {
		return nil, fmt.Errorf("failed to create handler for %s connection", config.ConnType)
	}

	client := modbus.NewClient(handler)

	return &modbusClient{
		client:   client,
		handler:  handler,
		connType: config.ConnType,
		addr:     config.Addr,
		port:     config.Port,
		baudRate: config.BaudRate,
		dataBits: config.DataBits,
		stopBits: config.StopBits,
		parity:   config.Parity,
		timeout:  timeout,
	}, nil
}

// getClient get client for specified slaveID
func (s *Forwarder) getClient(slaveID byte) (*modbusClient, error) {
	s.clientsMux.RLock()
	client, exists := s.clients[slaveID]
	s.clientsMux.RUnlock()

	if !exists {
		return nil, fmt.Errorf("slave %d not configured", slaveID)
	}

	return client, nil
}

// monitorConnections monitor connection status
func (s *Forwarder) monitorConnections() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkConnections()
		}
	}
}

// checkConnections check connection status
func (s *Forwarder) checkConnections() {
	s.clientsMux.RLock()
	defer s.clientsMux.RUnlock()

	for slaveID, client := range s.clients {
		// try to read a register to test connection
		_, err := client.client.ReadHoldingRegisters(1, 1)
		if err != nil {
			if client.lastError == nil || client.lastError.Error() != err.Error() {
				log.Printf("slave %d connection exception: %v", slaveID, err)
				client.lastError = err
			}
		} else {
			if client.lastError != nil {
				log.Printf("slave %d connection restored", slaveID)
				client.lastError = nil
			}
			client.lastConn = time.Now()
		}
	}
}

// ===================== below are the implementations of the function code handlers =====================

// readCoils read coils, function code 1
func (s *Forwarder) readCoils(server *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	slaveID, address, quantity, err := s.parseRequest(frame)
	if err != nil {
		log.Printf("failed to parse read coils request: %v", err)
		return nil, &mbserver.IllegalDataAddress
	}

	client, err := s.getClient(slaveID)
	if err != nil {
		log.Printf("failed to get client: %v", err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	results, err := client.client.ReadCoils(uint16(address), uint16(quantity))
	if err != nil {
		log.Printf("failed to read coils (slave %d, addr %d, count %d): %v", slaveID, address, quantity, err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	// construct response
	response := make([]byte, 1+len(results))
	response[0] = byte(len(results))
	copy(response[1:], results)

	// log.Printf("read coils success (slave %d, addr %d, count %d)", slaveID, address, quantity)
	return response, &mbserver.Success
}

// readDiscreteInputs read discrete inputs, function code 2
func (s *Forwarder) readDiscreteInputs(server *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	slaveID, address, quantity, err := s.parseRequest(frame)
	if err != nil {
		log.Printf("failed to parse read discrete inputs request: %v", err)
		return nil, &mbserver.IllegalDataAddress
	}

	client, err := s.getClient(slaveID)
	if err != nil {
		log.Printf("failed to get client: %v", err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	results, err := client.client.ReadDiscreteInputs(uint16(address), uint16(quantity))
	if err != nil {
		log.Printf("failed to read discrete inputs (slave %d, addr %d, count %d): %v", slaveID, address, quantity, err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	response := make([]byte, 1+len(results))
	response[0] = byte(len(results))
	copy(response[1:], results)

	// log.Printf("read discrete inputs success (slave %d, addr %d, count %d)", slaveID, address, quantity)
	return response, &mbserver.Success
}

// readHoldingRegisters read holding registers, function code 3
func (s *Forwarder) readHoldingRegisters(server *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	slaveID, address, quantity, err := s.parseRequest(frame)
	if err != nil {
		log.Printf("failed to parse read holding registers request: %v", err)
		return nil, &mbserver.IllegalDataAddress
	}

	client, err := s.getClient(slaveID)
	if err != nil {
		log.Printf("failed to get client: %v", err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	results, err := client.client.ReadHoldingRegisters(uint16(address), uint16(quantity))
	if err != nil {
		log.Printf("failed to read holding registers (slave %d, addr %d, count %d): %v", slaveID, address, quantity, err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	response := make([]byte, 1+len(results))
	response[0] = byte(len(results) * 2)
	for i, value := range results {
		response[1+i] = value
	}

	// log.Printf("read holding registers success (slave %d, addr %d, count %d)", slaveID, address, quantity)
	return response, &mbserver.Success
}

// readInputRegisters read input registers, function code 4
func (s *Forwarder) readInputRegisters(server *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	slaveID, address, quantity, err := s.parseRequest(frame)
	if err != nil {
		log.Printf("failed to parse read input registers request: %v", err)
		return nil, &mbserver.IllegalDataAddress
	}

	client, err := s.getClient(slaveID)
	if err != nil {
		log.Printf("failed to get client: %v", err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	results, err := client.client.ReadInputRegisters(uint16(address), uint16(quantity))
	if err != nil {
		log.Printf("failed to read input registers (slave %d, addr %d, count %d): %v", slaveID, address, quantity, err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	response := make([]byte, 1+len(results))
	response[0] = byte(len(results) * 2)
	for i, value := range results {
		response[1+i] = value
	}

	// log.Printf("read input registers success (slave %d, addr %d, count %d)", slaveID, address, quantity)
	return response, &mbserver.Success
}

// writeSingleCoil write single coil, function code 5
func (s *Forwarder) writeSingleCoil(server *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	slaveID, address, value, err := s.parseWriteSingleRequest(frame)
	if err != nil {
		log.Printf("failed to parse write single coil request: %v", err)
		return nil, &mbserver.IllegalDataAddress
	}

	client, err := s.getClient(slaveID)
	if err != nil {
		log.Printf("failed to get client: %v", err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	coilValue := value == 0xFF00
	_, err = client.client.WriteSingleCoil(uint16(address), uint16(value))
	if err != nil {
		log.Printf("failed to write single coil (slave %d, addr %d, value %v): %v", slaveID, address, coilValue, err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	log.Printf("write single coil success (slave %d, addr %d, value %v)", slaveID, address, coilValue)
	return frame.GetData()[0:4], &mbserver.Success
}

// writeSingleRegister write single register, function code 6
func (s *Forwarder) writeSingleRegister(server *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	slaveID, address, value, err := s.parseWriteSingleRequest(frame)
	if err != nil {
		log.Printf("failed to parse write single register request: %v", err)
		return nil, &mbserver.IllegalDataAddress
	}

	client, err := s.getClient(slaveID)
	if err != nil {
		log.Printf("failed to get client: %v", err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	_, err = client.client.WriteSingleRegister(uint16(address), uint16(value))
	if err != nil {
		log.Printf("failed to write single register (slave %d, addr %d, value %d): %v", slaveID, address, value, err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	log.Printf("write single register success (slave %d, addr %d, value %d)", slaveID, address, value)
	return frame.GetData()[0:4], &mbserver.Success
}

// writeMultipleCoils write multiple coils, function code 15
func (s *Forwarder) writeMultipleCoils(server *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	slaveID, address, quantity, data, err := s.parseWriteMultipleRequest(frame)
	if err != nil {
		log.Printf("failed to parse write multiple coils request: %v", err)
		return nil, &mbserver.IllegalDataAddress
	}

	client, err := s.getClient(slaveID)
	if err != nil {
		log.Printf("failed to get client: %v", err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	// convert data format
	coils := make([]bool, quantity)
	for i := 0; i < quantity; i++ {
		byteIndex := i / 8
		bitIndex := i % 8
		if byteIndex < len(data) {
			coils[i] = (data[byteIndex] & (1 << bitIndex)) != 0
		}
	}

	// convert bool slice to byte slice
	coilBytes := make([]byte, (quantity+7)/8)
	for i := 0; i < quantity; i++ {
		if coils[i] {
			byteIndex := i / 8
			bitIndex := i % 8
			coilBytes[byteIndex] |= 1 << bitIndex
		}
	}

	_, err = client.client.WriteMultipleCoils(uint16(address), uint16(quantity), coilBytes)
	if err != nil {
		log.Printf("failed to write multiple coils (slave %d, addr %d, count %d): %v", slaveID, address, quantity, err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	log.Printf("write multiple coils success (slave %d, addr %d, count %d)", slaveID, address, quantity)
	// safe return data, avoid array out of bounds
	frameData := frame.GetData()
	maxLen := len(frameData)
	if quantity*4 > maxLen {
		return frameData[0:maxLen], &mbserver.Success
	}
	return frameData[0 : quantity*4], &mbserver.Success
}

// writeMultipleRegisters write multiple registers, function code 16
func (s *Forwarder) writeMultipleRegisters(server *mbserver.Server, frame mbserver.Framer) ([]byte, *mbserver.Exception) {
	slaveID, address, quantity, data, err := s.parseWriteMultipleRequest(frame)
	if err != nil {
		log.Printf("failed to parse write multiple registers request: %v", err)
		return nil, &mbserver.IllegalDataAddress
	}

	client, err := s.getClient(slaveID)
	if err != nil {
		log.Printf("failed to get client: %v", err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	// convert data format
	registers := make([]uint16, quantity)
	for i := 0; i < quantity && i*2+1 < len(data); i++ {
		registers[i] = uint16(data[i*2])<<8 | uint16(data[i*2+1])
	}

	// convert uint16 slice to byte slice
	registerBytes := make([]byte, quantity*2)
	for i, value := range registers {
		registerBytes[i*2] = byte(value >> 8)
		registerBytes[i*2+1] = byte(value)
	}

	_, err = client.client.WriteMultipleRegisters(uint16(address), uint16(quantity), registerBytes)
	if err != nil {
		log.Printf("failed to write multiple registers (slave %d, addr %d, count %d): %v", slaveID, address, quantity, err)
		return nil, &mbserver.SlaveDeviceFailure
	}

	log.Printf("write multiple registers success (slave %d, addr %d, count %d)", slaveID, address, quantity)
	// safe return data, avoid array out of bounds
	frameData := frame.GetData()
	maxLen := len(frameData)
	if quantity*4 > maxLen {
		return frameData[0:maxLen], &mbserver.Success
	}
	return frameData[0 : quantity*4], &mbserver.Success
}

// parseRequest parse read request
func (s *Forwarder) parseRequest(frame mbserver.Framer) (slaveID byte, address, quantity int, err error) {
	data := frame.GetData()
	if len(data) < 4 {
		return 0, 0, 0, fmt.Errorf("insufficient data")
	}

	// extract slaveID from frame
	frameSlaveID := getSlaveID(frame)
	if frameSlaveID == 0 {
		return 0, 0, 0, fmt.Errorf("failed to get slaveID from frame")
	}

	// validate slaveID is in config
	if _, exists := s.config.Servers[frameSlaveID]; !exists {
		return 0, 0, 0, fmt.Errorf("slave %d not configured", frameSlaveID)
	}

	address = int(data[0])<<8 | int(data[1])
	quantity = int(data[2])<<8 | int(data[3])

	return frameSlaveID, address, quantity, nil
}

// parseWriteSingleRequest parse write single request
func (s *Forwarder) parseWriteSingleRequest(frame mbserver.Framer) (slaveID byte, address, value int, err error) {
	data := frame.GetData()
	if len(data) < 4 {
		return 0, 0, 0, fmt.Errorf("insufficient data")
	}

	// extract slaveID from frame
	frameSlaveID := getSlaveID(frame)
	if frameSlaveID == 0 {
		return 0, 0, 0, fmt.Errorf("failed to get slaveID from frame")
	}

	// validate slaveID is in config
	if _, exists := s.config.Servers[frameSlaveID]; !exists {
		return 0, 0, 0, fmt.Errorf("slave %d not configured", frameSlaveID)
	}

	address = int(data[0])<<8 | int(data[1])
	value = int(data[2])<<8 | int(data[3])

	return frameSlaveID, address, value, nil
}

// parseWriteMultipleRequest parse write multiple request
func (s *Forwarder) parseWriteMultipleRequest(frame mbserver.Framer) (slaveID byte, address, quantity int, data []byte, err error) {
	frameData := frame.GetData()
	if len(frameData) < 6 {
		return 0, 0, 0, nil, fmt.Errorf("insufficient data")
	}

	// extract slaveID from frame
	frameSlaveID := getSlaveID(frame)
	if frameSlaveID == 0 {
		return 0, 0, 0, nil, fmt.Errorf("failed to get slaveID from frame")
	}

	// validate slaveID is in config
	if _, exists := s.config.Servers[frameSlaveID]; !exists {
		return 0, 0, 0, nil, fmt.Errorf("slave %d not configured", frameSlaveID)
	}

	address = int(frameData[0])<<8 | int(frameData[1])
	quantity = int(frameData[2])<<8 | int(frameData[3])
	byteCount := int(frameData[4])

	if len(frameData) < 5+byteCount {
		return 0, 0, 0, nil, fmt.Errorf("insufficient data for byte count")
	}

	data = frameData[5 : 5+byteCount]

	return frameSlaveID, address, quantity, data, nil
}

func getSlaveID(frame mbserver.Framer) byte {
	if len(frame.Bytes()) < 7 {
		return 0
	}
	return frame.Bytes()[6]
}
