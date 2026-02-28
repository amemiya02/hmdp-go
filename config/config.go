package config

import (
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/viper"
)

// Config 根配置结构体
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	MySQL    MySQLConfig    `mapstructure:"mysql"`
	Redis    RedisConfig    `mapstructure:"redis"`
	RocketMQ RocketMQConfig `mapstructure:"rocketmq"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// MySQLConfig MySQL配置
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

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string        `mapstructure:"host"`
	Port     string        `mapstructure:"port"`
	Password string        `mapstructure:"password"`
	Db       int           `mapstructure:"db"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

// RocketMQConfig RocketMQ配置
type RocketMQConfig struct {
	NameServers []string `mapstructure:"name_servers"`
	GroupName   string   `mapstructure:"group_name"`
	Topic       string   `mapstructure:"topic"`
	Retries     int      `mapstructure:"retries"`
}

var GlobalConfig *Config

func init() {
	// 设置Viper基础配置
	v := viper.New()
	// 配置文件路径（相对于项目根目录）
	v.SetConfigType("yaml")
	// 这里用绝对路径是为了让test也能加载config
	// --- 获取当前 config.go 文件所在的绝对路径 ---
	_, filename, _, _ := runtime.Caller(0)
	// filename 是绝对路径: /Users/.../hmdp-go/config/config.go
	// filepath.Dir(filename) 就是 config 目录的绝对路径
	configDir := filepath.Dir(filename)

	// 拼接真正的配置文件绝对路径
	configPath := filepath.Join(configDir, "config.yaml")
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		panic(err)
	}

	GlobalConfig = &Config{}

	if err := v.Unmarshal(GlobalConfig); err != nil {
		panic(err)
	}
}
