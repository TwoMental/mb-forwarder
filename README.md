# MB-Forwarder


## Features

- 🔄 **Protocol Forwarding**: Support for forwarding between Modbus TCP and RTU protocols
- 🌐 **Multi-Server Support**: Connect to multiple Modbus slave devices simultaneously
- 📡 **Complete Function Code Support**: Support for all standard Modbus function codes
- 🔧 **Flexible Configuration**: Flexible connection parameter configuration via YAML files
- 📊 **Connection Monitoring**: Automatic connection status monitoring with reconnection support
- 🚀 **Cross-Platform**: Support for Linux, Windows, macOS and other platforms

## Supported Modbus Function Codes

| Function Code | Name | Description |
|---------------|------|-------------|
| 01 | Read Coils | Read single or multiple coil states |
| 02 | Read Discrete Inputs | Read single or multiple discrete input states |
| 03 | Read Holding Registers | Read single or multiple holding register values |
| 04 | Read Input Registers | Read single or multiple input register values |
| 05 | Write Single Coil | Write single coil state |
| 06 | Write Single Register | Write single register value |
| 15 | Write Multiple Coils | Write multiple coil states |
| 16 | Write Multiple Registers | Write multiple register values |

## System Requirements

- Go 1.24.0 or higher
- Linux/Windows/macOS operating system

## Installation

### Build from Source

```bash
# Clone repository
git clone https://github.com/TwoMental/mb-forwarder.git
cd mb-forwarder

# Run
make run
```

### Download Pre-built Version

Download pre-built versions suitable for your system from the [Releases](https://github.com/TwoMental/mb-forwarder/releases) page.

## Configuration

### Configuration File Format

Create a `config.yaml` file, following the format in `config.yaml.example`:

```yaml
# Listen port
listen_port: 1602

# Server configuration
servers:
  # Slave device 1 (TCP connection)
  1:
    conn_type: "tcp"        # Connection type: "tcp" or "rtu"
    slave_id: 1             # Slave device ID (1-255)
    addr: "192.168.1.100"   # TCP address or serial device name
    port: 502               # TCP port (required for TCP connections)
    timeout: 5              # Connection timeout in seconds
  
  # Slave device 2 (RTU connection)
  2:
    conn_type: "rtu"        # RTU connection
    slave_id: 1           
    addr: "/dev/ttyUSB0"    # Serial device name
    baud_rate: 9600         # Baud rate
    data_bits: 8            # Data bits
    stop_bits: 1            # Stop bits
    parity: "N"             # Parity: "N"(none), "E"(even), "O"(odd)
    timeout: 3
```

### Configuration Parameters

#### Global Configuration
- `listen_port`: Port number for the forwarder to listen on, default 1602

#### Server Configuration
- `conn_type`: Connection type, supports "tcp" or "rtu"
- `slave_id`: Slave device ID, range 1-255
- `addr`: Connection address
  - TCP: IP address
  - RTU: Serial device name (e.g., `/dev/ttyUSB0`, `COM1`)
- `port`: TCP port number (required only for TCP connections)
- `baud_rate`: Baud rate (required only for RTU connections)
- `data_bits`: Data bits (required only for RTU connections)
- `stop_bits`: Stop bits (required only for RTU connections)
- `parity`: Parity (required only for RTU connections)
- `timeout`: Connection timeout in seconds

## Usage

### Start the Forwarder

```bash
# Start with configuration file
./mb-forwarder -config config.yaml

# Or specify configuration file path
./mb-forwarder -config /path/to/config.yaml
```

## How It Works

1. **Startup Phase**: After startup, the forwarder creates a Modbus server and listens on the specified port
2. **Connection Initialization**: Creates connections to various slave devices according to configuration
3. **Request Processing**: Receives client requests, parses them, and forwards them to corresponding slave devices
4. **Response Return**: Returns slave device responses to clients
5. **Connection Monitoring**: Regularly checks connection status and records connection anomalies

## Log Output

The forwarder outputs detailed runtime logs:

```
2024/01/01 12:00:00 modbus forwarder listening on 0.0.0.0:1602
2024/01/01 12:00:00 initialized slave 1 connection (tcp)
2024/01/01 12:00:00 initialized slave 2 connection (rtu)
2024/01/01 12:00:00 modbus forwarder started with 2 servers
2024/01/01 12:00:00 Modbus forwarder started, press Ctrl+C to stop...
```

## Troubleshooting

### Common Issues

1. **Connection Failures**
   - Check if slave devices are online
   - Verify IP addresses and port numbers
   - Confirm firewall settings

2. **Serial Connection Issues**
   - Check serial device permissions
   - Verify serial parameter settings
   - Confirm device driver installation

3. **Timeout Errors**
   - Increase timeout configuration values
   - Check network latency
   - Verify slave device response times

## Performance Optimization

- Use connection pools to manage connections
- Set appropriate timeout values
- Monitor connection status and handle anomalies promptly
- Adjust buffer sizes based on network environment

## Contributing

Issues and Pull Requests are welcome!

## License

This project is licensed under the [LICENSE](LICENSE) license.

## Contact

- Project URL: [https://github.com/TwoMental/mb-forwarder](https://github.com/TwoMental/mb-forwarder)
- Issue Reports: [Issues](https://github.com/TwoMental/mb-forwarder/issues)
