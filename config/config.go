package config

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

var config *Config

func Current() *Config {
	return config
}

type Program struct {
	OrganizationName    string   `json:"organization_name"`
	OrganizationID      int      `json:"organization_id"`
	UserTypeID          int      `json:"user_type_id"`
	CaregiverUserTypeID int      `json:"caregiver_user_type_id"`
	ProIDs              []string `json:"pro_ids"`
}

type LandingConfig struct {
	ClientID    string             `mapstructure:"client_id" json:"client_id"`
	Username    string             `mapstructure:"username" json:"username"`
	Password    string             `mapstructure:"password" json:"password"`
	ProgramsRaw string             `mapstructure:"programs" json:"-"`
	ProgramMap  map[string]Program `json:"programs"`
}

type CommonConfig struct {
	PublicBaseURI string            `mapstructure:"public_base_uri" json:"public_base_uri"`
	Redirects     map[string]string `mapstructure:"redirects"`
}

type Config struct {
	Common  CommonConfig              `mapstructure:"common" json:"common"`
	Landing map[string]*LandingConfig `mapstructure:"landing" json:"landing"`
}

func LoadConfigFromParamStore(region, path string, logger *zap.Logger) {
	session, _ := awssession.NewSession(&aws.Config{Region: aws.String(region)})
	svc := ssm.New(session)

	in := &ssm.GetParametersByPathInput{}
	in.SetPath(path)
	in.SetWithDecryption(true)
	in.SetRecursive(true)

	config = &Config{}

	pm := make(map[string]string)
	err := svc.GetParametersByPathPages(in, func(params *ssm.GetParametersByPathOutput, lastPage bool) bool {
		for _, p := range params.Parameters {
			pm[strings.TrimPrefix(*p.Name, path)] = *p.Value
		}
		return !lastPage
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			logger.Fatal(
				"AWS error",
				zap.String("code", awsErr.Code()),
				zap.String("message", awsErr.Message()),
			)
		} else {
			logger.Fatal(
				"System error",
				zap.Error(err),
			)
		}
		return
	} else {
		cm := map[string]map[string]interface{}{}
		for k, v := range pm {
			ks := strings.Split(k, "/")
			if _, ok := cm[ks[0]]; !ok {
				cm[ks[0]] = map[string]interface{}{}
			}
			m := cm[ks[0]]

			var i int
			for i = 1; i < len(ks)-1; i++ {
				if _, ok := m[ks[i]]; !ok {
					m[ks[i]] = map[string]interface{}{}
				}
				m = m[ks[i]].(map[string]interface{})
			}
			m[ks[i]] = v
		}
		mapstructure.Decode(cm, config)
		for _, l := range config.Landing {

			if l.ProgramsRaw != "" {
				l.ProgramMap = map[string]Program{}
				programs := []Program{}
				err := json.Unmarshal([]byte(l.ProgramsRaw), &programs)
				if err != nil {
					logger.Fatal(
						"System error, bad programs json",
						zap.Error(err),
					)
				}
				for _, p := range programs {
					l.ProgramMap[p.OrganizationName] = p
				}
			}
		}
	}
}

func LoadConfigFromJSON(path string, logger *zap.Logger) {
	config = &Config{}
	d, err := ioutil.ReadFile(path)
	if err != nil {
		logger.Fatal(
			"Config read error",
			zap.Error(err),
		)
	}
	err = json.Unmarshal(d, config)
	if err != nil {
		logger.Fatal(
			"Config parse error",
			zap.Error(err),
		)
	}
}
