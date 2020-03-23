package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime/pprof"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	logger "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type IdentifyCfg struct {
	AuthSecret string `yaml:"AuthSecret"`
}

type LogCfg struct {
	LogPath   string `yaml:"LogPath"`
	LogLevel  string `yaml:"LogLevel"`
	IsStdOut  bool   `yaml:"IsStdOut"`
	IsPProf   bool   `yaml:"IsPProf"`
	PathPProf string `yaml:"PathPProf"`
}

type MySQLCfg struct {
	DSN         string        `yaml:"DSN"`
	Active      int           `yaml:"Active"`
	Idle        int           `yaml:"Idle"`
	IdleTimeout time.Duration `yaml:"IdleTimeout"`
}

type GrpcSrvCfg struct {
	Address string `yaml:"Address"`
}

type Config struct {
	ProjectName string      `yaml:"ProjectName"`
	Identify    IdentifyCfg `yaml:"Identify"`
	Log         LogCfg      `yaml:"Log"`
	MySQL       MySQLCfg    `yaml:"MySQL"`
	GrpcSrv     GrpcSrvCfg  `yaml:"GrpcSrv"`
}

type Env struct {
	Cfg      *Config
	MysqlCli *gorm.DB
}

var (
	cpuProfilingFile,
	memProfilingFile,
	blockProfilingFile,
	goroutineProfilingFile,
	threadCreateProfilingFile *os.File
)

func InitLog(setting *Config) error {
	logger.SetFormatter(&logger.JSONFormatter{})
	if setting.Log.LogLevel == "" {
		return nil
	}

	lvl, err := logger.ParseLevel(setting.Log.LogLevel)
	if err != nil {
		return err
	}
	logger.SetLevel(lvl)

	if setting.Log.IsStdOut {
		logger.SetOutput(os.Stdout)
		logger.SetFormatter(&logger.TextFormatter{})
	}

	logger.SetReportCaller(true)

	// todo AddHook

	return nil
}

func InitMySQLClient(setting *Config) (*gorm.DB, error) {
	db, err := gorm.Open("mysql", setting.MySQL.DSN)
	if err != nil {
		return nil, err
	}

	db.DB().SetMaxIdleConns(setting.MySQL.Idle)
	db.DB().SetMaxOpenConns(setting.MySQL.Active)
	db.DB().SetConnMaxLifetime(time.Duration(setting.MySQL.IdleTimeout) / time.Second)

	if setting.Log.LogLevel == "debug" {
		db.LogMode(true)
	}

	return db, nil
}

func InitDebugPProf(setting *Config) error {
	if setting.Log.IsPProf {
		pathPrefix := path.Join(setting.Log.PathPProf, fmt.Sprintf("%d", os.Getpid()))
		logger.Infof("start pprof, and will save to %s", pathPrefix)
		cpuProfilingFile, _ = os.Create(pathPrefix + "-cpu.prof")
		memProfilingFile, _ = os.Create(pathPrefix + "-mem.prof")
		blockProfilingFile, _ = os.Create(pathPrefix + "-block.prof")
		goroutineProfilingFile, _ = os.Create(pathPrefix + "-goroutine.prof")
		threadCreateProfilingFile, _ = os.Create(pathPrefix + "-threadcreat.prof")
		_ = pprof.StartCPUProfile(cpuProfilingFile)
	}
	return nil
}

// SaveProfile try to save pprof into local file
func (e *Env) SaveProfile() {
	if e.Cfg.Log.IsPProf {
		goroutine := pprof.Lookup("goroutine")
		goroutine.WriteTo(goroutineProfilingFile, 1)
		heap := pprof.Lookup("heap")
		heap.WriteTo(memProfilingFile, 1)
		block := pprof.Lookup("block")
		block.WriteTo(blockProfilingFile, 1)
		threadcreate := pprof.Lookup("threadcreate")
		threadcreate.WriteTo(threadCreateProfilingFile, 1)
		pprof.StopCPUProfile()
	}
}

func Init(confPath string) (*Env, error) {
	content, err := ioutil.ReadFile(confPath)
	if err != nil {
		panic(err)
	}

	var setting Config
	err = yaml.Unmarshal(content, &setting)
	if err != nil {
		panic(err)
	}

	err = InitDebugPProf(&setting)
	if err != nil {
		return nil, err
	}

	err = InitLog(&setting)
	if err != nil {
		return nil, err
	}

	mysqlCli, err := InitMySQLClient(&setting)
	if err != nil {
		return nil, err
	}

	return &Env{&setting, mysqlCli}, nil
}
