// internal/config/config.go
// Loader konfigurasi dari environment variables
// internal/config/config.go
package config

import (
	"fmt"
	"log"
	"os"
)


type Config struct {
	AppName   string
	AppEnv    string
	AppPort   string
	MCPPort   string
	LogLevel  string
	LogFormat string

	MySQL struct {
		Host     string
		Port     string
		DB       string
		User     string
		Password string
		MaxOpen  int
		MaxIdle  int
	}

	LLM struct {
		Provider       string // default: openai
		APIKey         string
		APIBase        string
		Model          string
		EmbeddingModel string
	}
}

func Load() *Config {
	c := &Config{}
	c.AppName = getEnv("APP_NAME", "mcp-oilgas")
	c.AppEnv = getEnv("APP_ENV", "development")
	c.AppPort = getEnv("APP_PORT", "8080")
	c.MCPPort = getEnv("MCP_PORT", "8090")
	c.LogLevel = getEnv("LOG_LEVEL", "debug")
	c.LogFormat = getEnv("LOG_FORMAT", "json")

	c.MySQL.Host = getEnv("MYSQL_HOST", "localhost")
	c.MySQL.Port = getEnv("MYSQL_PORT", "3306")
	c.MySQL.DB = getEnv("MYSQL_DB", "mcp")
	c.MySQL.User = getEnv("MYSQL_USER", "root")
	c.MySQL.Password = getEnv("MYSQL_PASSWORD", "")
	c.MySQL.MaxOpen = getEnvInt("MYSQL_MAX_OPEN_CONNS", 10)
	c.MySQL.MaxIdle = getEnvInt("MYSQL_MAX_IDLE_CONNS", 5)

	// LLM / OpenAI
	c.LLM.Provider = getEnv("LLM_PROVIDER", "openai")
	c.LLM.APIKey = getEnv("OPENAI_API_KEY", "")
	c.LLM.APIBase = getEnv("OPENAI_API_BASE", "https://api.openai.com/v1")
	c.LLM.Model = getEnv("OPENAI_MODEL", "gpt-4o-mini")
	c.LLM.EmbeddingModel = getEnv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")

	if c.LLM.APIKey == "" {
		log.Println("[WARN] OPENAI_API_KEY is not set, LLM features may not work")
	}

	return c
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		_, err := fmt.Sscanf(v, "%d", &i)
		if err == nil {
			return i
		}
	}
	return def
}
