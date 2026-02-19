package config

import (
	"path/filepath"
	"time"

	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/spf13/viper"
)

// 根配置结构体
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	MySQL  MySQLConfig  `mapstructure:"mysql"`
	Redis  RedisConfig  `mapstructure:"redis"`
}

// 服务器配置
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// MySQL配置
type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         string `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	DbName       string `mapstructure:"dbname"`
	Charset      string `mapstructure:"charset"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

// Redis配置
type RedisConfig struct {
	Host     string        `mapstructure:"host"`
	Port     string        `mapstructure:"port"`
	Password string        `mapstructure:"password"`
	Db       int           `mapstructure:"db"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

var GlobalConfig *Config

func InitGlobalConfig() {
	// 设置Viper基础配置
	v := viper.New()
	// 配置文件路径（相对于项目根目录）
	v.SetConfigType("yaml")
	configPath := filepath.Join("config", "config.yaml")
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		global.Logger.Fatalf("Error reading config file: %s", err.Error())
	}

	GlobalConfig = &Config{}

	if err := v.Unmarshal(GlobalConfig); err != nil {
		global.Logger.Fatalf("Error unmarshalling config: %s", err.Error())
	}
}

func init() {
	InitGlobalConfig()
	InitDb()
	InitRedis()
}
