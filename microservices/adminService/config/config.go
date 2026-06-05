package config

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var envPtr = pflag.String("env", "dev", "Environment: dev or prod")

func InitLoadConfig() *AllConfig {
	// 使用pflag库来读取命令行参数，用于指定环境，默认为"dev"
	pflag.Parse()

	config := viper.New()
	// 兼容不同启动目录，优先在 adminService 内查找配置文件。
	for _, p := range candidateConfigPaths() {
		config.AddConfigPath(p)
	}
	// 设置读取文件名字
	config.SetConfigName(fmt.Sprintf("application-%s", *envPtr))
	// 设置读取文件类型
	config.SetConfigType("yaml")
	// 读取文件载体
	var configData *AllConfig
	// 读取配置文件
	err := config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Use Viper ReadInConfig Fatal error config err:%s \n", err))
	}
	// 查找对应配置文件
	err = config.Unmarshal(&configData)
	if err != nil {
		panic(fmt.Errorf("read config file to struct err: %s\n", err))
	}
	// 打印配置文件信息
	fmt.Printf("配置文件信息：%+v", configData)
	return configData
}

func candidateConfigPaths() []string {
	paths := []string{
		"./config",
		"../config",
		"../../config",
		"../../../config",
		"../../../../config",
	}

	if _, file, _, ok := runtime.Caller(0); ok {
		paths = append(paths, filepath.Dir(file))
	}

	seen := make(map[string]struct{}, len(paths))
	unique := make([]string, 0, len(paths))
	for _, p := range paths {
		cleaned := filepath.Clean(strings.TrimSpace(p))
		if cleaned == "" {
			continue
		}
		if _, exists := seen[cleaned]; exists {
			continue
		}
		seen[cleaned] = struct{}{}
		unique = append(unique, cleaned)
	}

	return unique
}

// AllConfig 整合Config
type AllConfig struct {
	Server     Server
	DataSource DataSource
	Redis      Redis
	Log        Log
	Jwt        Jwt
	AliOss     AliOss
	Wechat     Wechat
}

type Server struct {
	Port  string
	Level string
}

type DataSource struct {
	Host     string
	Port     string
	UserName string
	Password string
	DBName   string `mapstructure:"db_name"`
	Config   string
}

func (d *DataSource) Dsn() string {
	return d.UserName + ":" + d.Password + "@tcp(" + d.Host + ":" + d.Port + ")/" + d.DBName + "?" + d.Config
}

type Redis struct {
	Host     string
	Port     string
	Password string
	DataBase int `mapstructure:"data_base"`
}

type Log struct {
	Level    string
	FilePath string
}

type Jwt struct {
	Admin JwtOption
	User  JwtOption
}

type JwtOption struct {
	Secret string
	TTL    string
	Name   string
}

type AliOss struct {
	EndPoint        string
	AccessKeyId     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	BucketName      string `mapstructure:"bucket_name"`
}

type Wechat struct {
	AppId  string
	Secret string
}
