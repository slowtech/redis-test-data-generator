package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/redis/go-redis/v9"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

// Global variable to keep track of the number of expired keys to be set
var expiredKeyCount int
var expireEnabled bool
var memoryLimitEnabled bool

type Config struct {
	Host          string
	Port          int
	Password      string
	TotalKeys     int
	ExpiryRatio   float64
	ExpiryStart   int
	ExpiryEnd     int
	TLSEnabled    bool
	ExpireEnabled bool
	MaxMemoryMB   string
}

// Define an interface for Redis client
type RedisClient interface {
	Ping(ctx context.Context) *redis.StatusCmd
	Pipeline() redis.Pipeliner
	Close() error
}

func getConnection(host string, port int, password string, tlsEnabled bool) RedisClient {
	address := fmt.Sprintf("%s:%d", host, port)
	options := &redis.Options{
		Addr:        address,
		Password:    password,
		DialTimeout: 2 * time.Second,
	}

	if tlsEnabled {
		options.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	// Try connecting as a cluster client first
	clusterOptions := &redis.ClusterOptions{
		Addrs:       []string{address},
		Password:    password,
		DialTimeout: 2 * time.Second,
		TLSConfig:   options.TLSConfig,
	}

	clusterClient := redis.NewClusterClient(clusterOptions)
	if err := testClusterConnection(clusterClient); err != nil {
		// If cluster connection fails, fallback to standalone
		standaloneClient := redis.NewClient(options)
		return standaloneClient
	}
	return clusterClient
}

func testClusterConnection(client *redis.ClusterClient) error {
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		// Check if the error is related to cluster support being disabled
		if err.Error() == "ERR This instance has cluster support disabled" {
			return err
		}
	}
	return nil
}

func setKeyExpiry(ctx context.Context, pipe redis.Pipeliner, key string, i, threshold, expiryStart, expiryEnd int) {
	if (memoryLimitEnabled || expiredKeyCount > 0) && expireEnabled {

		if i%10 < threshold {
			expireTime := time.Duration(rand.Intn(expiryEnd-expiryStart+1)+expiryStart) * time.Second
			pipe.Expire(ctx, key, expireTime)
			if !memoryLimitEnabled {
				expiredKeyCount--
			}
		}
	}
}

func getUsedMemory(ctx context.Context, redisClient RedisClient) (int, error) {
	var usedMemoryStr string
	var memoryStr string
	if client, ok := redisClient.(*redis.Client); ok {
		memoryInfo := client.Info(ctx, "memory")
		memoryStr = memoryInfo.Val()
	} else if client, ok := redisClient.(*redis.ClusterClient); ok {
		memoryInfo := client.Info(ctx, "memory")
		memoryStr = memoryInfo.Val()
	}
	for _, line := range strings.Split(memoryStr, "\n") {
		if strings.HasPrefix(line, "used_memory:") {
			fields := strings.Split(line, ":")
			if len(fields) > 1 {
				usedMemoryStr = strings.TrimSpace(fields[1])
				break
			}
		}
	}
	usedMemory, err := strconv.Atoi(usedMemoryStr)
	if err != nil {
		return 0, fmt.Errorf("failed to convert used_memory to int: %w", err)
	}
	return usedMemory, nil
}

func generateDataWithPipeline(ctx context.Context, redisClient RedisClient, startIndex, batchSize int, timestamp string, expiryRatio float64, expiryStart, expiryEnd int) int {
	pipe := redisClient.Pipeline()
	dataTypes := 5
	keysPerType := batchSize / dataTypes
	remainingKeys := batchSize - (keysPerType * dataTypes)
	endIndex := startIndex + keysPerType
	threshold := int(expiryRatio * 10)

	for i := startIndex; i < endIndex; i++ {
		key := fmt.Sprintf("mykey_%s:%d", timestamp, i)
		pipe.Set(ctx, key, fmt.Sprintf("value_%d", i), 0)
		setKeyExpiry(ctx, pipe, key, i, threshold, expiryStart, expiryEnd)

		key = fmt.Sprintf("mylist_%s:%d", timestamp, i)
		values := []interface{}{fmt.Sprintf("list_value_%d_1", i), fmt.Sprintf("list_value_%d_2", i), fmt.Sprintf("list_value_%d_3", i)}
		pipe.RPush(ctx, key, values...)
		setKeyExpiry(ctx, pipe, key, i, threshold, expiryStart, expiryEnd)

		key = fmt.Sprintf("myhash_%s:%d", timestamp, i)
		pipe.HSet(ctx, key, map[string]interface{}{
			"field1": fmt.Sprintf("value1_%d", i),
			"field2": fmt.Sprintf("value2_%d", i),
			"field3": fmt.Sprintf("value3_%d", i),
		})
		setKeyExpiry(ctx, pipe, key, i, threshold, expiryStart, expiryEnd)

		key = fmt.Sprintf("myset_%s:%d", timestamp, i)
		pipe.SAdd(ctx, key, fmt.Sprintf("set_value_%d_1", i), fmt.Sprintf("set_value_%d_2", i), fmt.Sprintf("set_value_%d_3", i))
		setKeyExpiry(ctx, pipe, key, i, threshold, expiryStart, expiryEnd)

		key = fmt.Sprintf("mysortedset_%s:%d", timestamp, i)
		pipe.ZAdd(ctx, key, redis.Z{Score: float64(i * 1), Member: fmt.Sprintf("sorted_value_%d_1", i)},
			redis.Z{Score: float64(i * 2), Member: fmt.Sprintf("sorted_value_%d_2", i)},
			redis.Z{Score: float64(i * 3), Member: fmt.Sprintf("sorted_value_%d_3", i)})
		setKeyExpiry(ctx, pipe, key, i, threshold, expiryStart, expiryEnd)
	}

	for i := endIndex; i < endIndex+remainingKeys; i++ {
		key := fmt.Sprintf("mykey_%s:%d", timestamp, i)
		pipe.Set(ctx, key, fmt.Sprintf("value_%d", i), 0)
		setKeyExpiry(ctx, pipe, key, i, threshold, expiryStart, expiryEnd)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		fmt.Printf("Error generating data: %v\n", err)
		os.Exit(1)
	}
	return endIndex
}

func main() {
	var c Config
	f := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	f.StringVar(&c.Host, "h", "", "Redis host (required)")
	f.IntVar(&c.Port, "P", 6379, "Redis port")
	f.StringVar(&c.Password, "a", "", "Redis password")
	f.IntVar(&c.TotalKeys, "n", 10000, "Total number of keys to generate")
	f.Float64Var(&c.ExpiryRatio, "r", 1.0, "Ratio of keys to expire (0.0 to 1.0)")
	f.IntVar(&c.ExpiryStart, "s", 60, "Start of expiration time in seconds")
	f.IntVar(&c.ExpiryEnd, "e", 3600, "End of expiration time in seconds")
	f.BoolVar(&c.TLSEnabled, "tls", false, "Enable TLS for Redis connection")
	f.BoolVar(&c.ExpireEnabled, "expire", true, "Enable expiration time for Redis keys")
	f.StringVar(&c.MaxMemoryMB, "max_memory", "", "Specify the size of test data to generate (in MB)")

	f.Parse(os.Args[1:])

	// Check if host was provided
	if c.Host == "" {
		fmt.Println("Error: -h (Redis host) is required.")
		f.PrintDefaults()
		os.Exit(1)
	}
	var maxMemoryBytes int
	if c.MaxMemoryMB != "" {
		maxMemoryMB, err := strconv.Atoi(c.MaxMemoryMB)
		if err != nil {
			fmt.Printf("Invalid value for max_memory: %v\n", err)
			return
		}
		maxMemoryBytes = maxMemoryMB * 1024 * 1024 // Convert MB to bytes
		memoryLimitEnabled = true
	}
	redisClient := getConnection(c.Host, c.Port, c.Password, c.TLSEnabled)
	defer redisClient.Close()

	ctx := context.Background()
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		fmt.Printf("Failed to ping Redis: %v\n", err)
		return
	}
	timestamp := time.Now().Format("20060102150405")
	expiredKeyCount = int(float64(c.TotalKeys) * c.ExpiryRatio)
	expireEnabled = c.ExpireEnabled
	batchSize := 10000
	startIndex := 1
	generatedKeys := 0

        fmt.Println(c.MaxMemoryMB)
	if c.MaxMemoryMB != "" {
		for {
			usedMemory, _ := getUsedMemory(ctx, redisClient)

			if usedMemory >= maxMemoryBytes {
				fmt.Printf("\nUsed memory (%d bytes) exceeds max memory limit (%d bytes). Exiting...\n", usedMemory, maxMemoryBytes)
				return
			}
			startIndex = generateDataWithPipeline(ctx, redisClient, startIndex, batchSize, timestamp, c.ExpiryRatio, c.ExpiryStart, c.ExpiryEnd)
                        generatedKeys += batchSize
                            remainingMemoryBytes := maxMemoryBytes - usedMemory
remainingMemoryMB := float64(remainingMemoryBytes) / 1048576 

			fmt.Printf("\rGenerate: %d keys, Memory needed to fill: %.2f MB", generatedKeys, remainingMemoryMB)
		}
	}

	for generatedKeys < c.TotalKeys {
		if c.TotalKeys-generatedKeys < batchSize {
			batchSize = c.TotalKeys - generatedKeys
		}
		startIndex = generateDataWithPipeline(ctx, redisClient, startIndex, batchSize, timestamp, c.ExpiryRatio, c.ExpiryStart, c.ExpiryEnd)
		generatedKeys += batchSize
		progress := float64(generatedKeys) / float64(c.TotalKeys) * 100
		fmt.Printf("\rProgress: %.2f%% (%d/%d keys)", progress, generatedKeys, c.TotalKeys)
	}
	fmt.Println()
}
