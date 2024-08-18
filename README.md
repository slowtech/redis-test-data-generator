# Redis Test Data Generation Tool

This Go-based tool is designed to generate a large volume of keys in Redis, with options for key expiration and TLS support. It can connect to either a standalone Redis instance or a Redis cluster, and supports various data types including strings, lists, hashes, sets, and sorted sets.

## Features

- **Key Generation**: Generate a specified number of keys across multiple Redis data types (strings, lists, hashes, sets, sorted sets).
- **Key Expiration**: Control the ratio of keys that expire, with customizable expiration times.
- **TLS Support**: Optionally enable TLS for secure Redis connections.
- **Cluster Support**: Automatically detects and connects to Redis clusters, falling back to standalone mode if necessary.
- **Batch Processing**: Keys are generated in batches for efficiency.

## Installation

### Method 1: Download Precompiled Binary

1. **Download the precompiled binary**:

   ```bash
   wget https://github.com/slowtech/redis-test-data-generator/releases/download/v1.0.0/redis-test-data-generator-linux-amd64.tar.gz
   ```

2. **Extract the binary**:

   ```bash
   tar xvf redis-test-data-generator-linux-amd64.tar.gz
   ```

### Method 2: Build from Source

1. **Clone the repository**:

   ```bash
   git clone https://github.com/slowtech/redis-test-data-generator.git
   ```

2. **Navigate to the project directory**:

   ```bash
   cd redis-test-data-generator
   ```

3. **Build the project**:

   ```bash
   go build -o redis-test-data-generator
   ```

## Usage

Run the tool with the following command-line options:

```bash
# ./redis-test-data-generator --help
Usage of ./redis-test-data-generator:
  -P int
        Redis port (default 6379)
  -a string
        Redis password
  -e int
        End of expiration time in seconds (default 3600)
  -h string
        Redis host (required)
  -n int
        Total number of keys to generate (default 10000)
  -r float
        Ratio of keys to expire (0.0 to 1.0) (default 1)
  -s int
        Start of expiration time in seconds (default 60)
  -tls
        Enable TLS for Redis connection
```

### Example

To generate 20,000 keys on a Redis instance running at `192.168.1.10:6380`, with 50% of the keys set to expire within 120 to 600 seconds, run:

```
./redis-test-data-generator -h 192.168.1.10 -P 6380 -n 20000 -r 0.5 -s 120 -e 600
```
