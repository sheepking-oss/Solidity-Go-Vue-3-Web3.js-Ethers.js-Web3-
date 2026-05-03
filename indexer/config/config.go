package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	EthRPCURL         string
	ContractAddress   string
	DBHost            string
	DBPort            int
	DBUser            string
	DBPassword        string
	DBName            string
	DBSSLMode         string
	ServerPort        int
	StartBlock        uint64
	BlockConfirmations uint64
	ReorgCheckInterval int
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using default values")
	}

	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	serverPort, _ := strconv.Atoi(getEnv("SERVER_PORT", "8080"))
	startBlock, _ := strconv.ParseUint(getEnv("START_BLOCK", "0"), 10, 64)
	blockConfirmations, _ := strconv.ParseUint(getEnv("BLOCK_CONFIRMATIONS", "6"), 10, 64)
	reorgCheckInterval, _ := strconv.Atoi(getEnv("REORG_CHECK_INTERVAL", "15"))

	return &Config{
		EthRPCURL:          getEnv("ETH_RPC_URL", "http://localhost:8545"),
		ContractAddress:    getEnv("CONTRACT_ADDRESS", ""),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             dbPort,
		DBUser:             getEnv("DB_USER", "postgres"),
		DBPassword:         getEnv("DB_PASSWORD", ""),
		DBName:             getEnv("DB_NAME", "supply_chain"),
		DBSSLMode:          getEnv("DB_SSL_MODE", "disable"),
		ServerPort:         serverPort,
		StartBlock:         startBlock,
		BlockConfirmations: blockConfirmations,
		ReorgCheckInterval: reorgCheckInterval,
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
