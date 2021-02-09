package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var testDataDir string

func TestMain(m *testing.M) {
	_, filePath, _, _ := runtime.Caller(0)
	testDataDir = strings.Replace(filepath.Dir(filePath), "config", "testdata", 1)

	exitVal := m.Run()

	//do any additional teardown here
	os.Exit(exitVal)
}

func TestConfig(t *testing.T) {
	path := fmt.Sprintf("%s/config/test.json", testDataDir)
	LoadConfigFromJSON(path, configTestLogger())

	c := Current()

	require.NotNil(t, c)
	assert.Equal(t, "https://app.dev.alwaysreach.net/public", c.Common.PublicBaseURI)
	require.NotNil(t, c.Landing["test-sample"])
	assert.Equal(t, "oauth.client.id", c.Landing["test-sample"].ClientID)
	assert.Equal(t, "apidude", c.Landing["test-sample"].Username)
	assert.Equal(t, "therug", c.Landing["test-sample"].Password)
	require.NotNil(t, c.Landing["test-sample"].ProgramMap["test-program"])
	p := c.Landing["test-sample"].ProgramMap["test-program"]
	assert.Equal(t, "test-org", p.OrganizationName)
	assert.Equal(t, 987, p.OrganizationID)
	assert.Equal(t, 654, p.UserTypeID)
	assert.Equal(t, []string{"pro1", "pro2"}, p.ProIDs)

}

func configTestLogger() *zap.Logger {

	var logger *zap.Logger
	l := zap.NewAtomicLevel()
	l.UnmarshalText([]byte("debug"))
	conf := zap.Config{
		Level:             l,
		Development:       false,
		DisableStacktrace: true,
		Encoding:          "console",
		EncoderConfig:     zap.NewProductionEncoderConfig(),
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stdout"},
	}
	conf.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	newLogger, _ := conf.Build()
	logger = newLogger.Named("cs-common")
	return logger
}
